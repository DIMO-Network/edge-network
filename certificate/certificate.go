package certificate

import (
	cr "crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"github.com/DIMO-Network/edge-network/commands"
	"github.com/DIMO-Network/edge-network/internal/gateways"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/smallstep/certificates/api"
	"github.com/smallstep/certificates/authority"
	"github.com/smallstep/certificates/ca"
	"go.step.sm/crypto/jose"
	"go.step.sm/crypto/x509util"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const generateChallenge = "/auth/web3/generate_challenge"
const submitChallenge = "/auth/web3/submit_challenge"
const domain = "http://127.0.0.1:10000"
const certificatePath = "/opt/autopi/client.pem"

//go:generate mockgen -source certificate.go -destination mocks/certificate_mock.go
type Signer interface {
	Sign(req *api.SignRequest) (*api.SignResponse, error)
}

type CertificateService struct {
	logger            zerolog.Logger
	oauthURL          string
	oauthClientID     string
	oauthClientSecret string
	caURL             string
	caFingerprint     string
	certificatePath   string
	stepCa            Signer
}

func NewCertificateService(logger zerolog.Logger, env gateways.Environment, client Signer) *CertificateService {
	// set the auth and ca urls based on the environment
	var authURL string
	var caURL string
	var oauthClientID string
	var oauthClientSecret string
	var caFingerprint string
	if env == gateways.Development {
		authURL = "https://auth.dev.dimo.zone"
		caURL = "https://ca.dev.dimo.zone"
		oauthClientID = "step-ca"
		oauthClientSecret = "KsQ7pruHob6D3NLFQEg9"
		caFingerprint = "a563363f0bc9cc76031695743c059cf1e694f294e4d1548e981d18cb96348f5f"
	} else {
		authURL = "https://auth.dimo.zone"
		caURL = "https://ca.dimo.zone"
		oauthClientID = "step-ca"
		oauthClientSecret = "mkoLsNAfiG2DM2DfqYsX"
		caFingerprint = "9992e3ce6a87c5d8dc6a09daddd4365c9e0f50593f3e897dedc1b89c037270ed"
	}

	return &CertificateService{
		logger:            logger,
		oauthURL:          authURL,
		oauthClientID:     oauthClientID,
		oauthClientSecret: oauthClientSecret,
		caURL:             caURL,
		caFingerprint:     caFingerprint,
		certificatePath:   certificatePath,
		stepCa:            client,
	}
}

type ChallengeResponse struct {
	State     string `json:"state"`
	Challenge string `json:"challenge"`
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
}

// CheckCertAndRenewIfExpiresSoon checks if the certificate exists and renews it if it expires in 1 day
func (cs *CertificateService) CheckCertAndRenewIfExpiresSoon(ethAddr common.Address, unitID uuid.UUID) error {
	// check if the certificate file exists
	_, err := os.Stat(cs.certificatePath)
	if os.IsNotExist(err) {
		cert, err := cs.SignWeb3Certificate(ethAddr.String(), true, unitID)

		if err != nil {
			return fmt.Errorf("failed to request certificate: %w", err)
		}

		cs.logger.Info().Msgf("Certificate response: %s", cert)
		// Save certificate
		err = os.WriteFile(cs.certificatePath, []byte(cert), 0644)
		if err != nil {
			return fmt.Errorf("failed to save certificate: %w", err)
		}
		return nil
	}

	// Read the cert file
	certPEM, err := os.ReadFile(cs.certificatePath)
	if err != nil {
		cs.logger.Warn().Msgf("Failed to read the cert file: %s", err)
	}

	// Parse the PEM-encoded certificate
	block, _ := pem.Decode(certPEM)
	if block == nil || block.Type != "CERTIFICATE" {
		return fmt.Errorf("failed to decode PEM block containing the certificate")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	cs.logger.Info().Msgf("Certificate expires on: %s", cert.NotAfter)

	// Check if the certificate will expire in 1 day
	if time.Until(cert.NotAfter) <= 24*time.Hour {
		cs.logger.Warn().Msgf("Certificate will expire on: %s. Renewing now...", cert.NotAfter)
		cert, err := cs.SignWeb3Certificate(ethAddr.String(), true, unitID)

		cs.logger.Info().Msgf("Certificate response: %s", cert)
		if err != nil {
			return fmt.Errorf("failed to renew certificate: %w", err)
		}
		cs.logger.Info().Msg("Certificate renewed successfully")
		// Save certificate
		err = os.WriteFile(cs.certificatePath, []byte(cert), 0644)
		if err != nil {
			return fmt.Errorf("failed to save certificate: %w", err)
		}
	}

	return nil
}

// SignWeb3Certificate exchanges an JWT  for a signed certificate
func (cs *CertificateService) SignWeb3Certificate(ethAddress string, confirm bool, unitID uuid.UUID) (string, error) {
	// duplicated from python code, not sure if we need it
	if !confirm {
		return "", errors.New("This command will create and sign a new client certificate - add parameter 'confirm=true' to continue anyway")
	}

	token, err := cs.GetOauthToken(ethAddress, unitID)
	if err != nil {
		return "", err
	}

	cs.logger.Debug().Msgf("Token response: %s", token)

	// Create a new client for the CA
	//  This gives us ability to pass mock stepCa client in tests
	if cs.stepCa == nil {
		cs.stepCa, err = ca.NewClient(cs.caURL, ca.WithRootSHA256(cs.caFingerprint))
		if err != nil {
			return "", err
		}
	}
	// Generate a new sign request with a randomly generated key.
	req, _, err := createSignRequest(token, ethAddress)
	if err != nil {
		panic(err)
	}

	certificate, err := cs.stepCa.Sign(req)
	if err != nil {
		return "", err
	}

	certificatePem := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.CaPEM.Raw})

	return string(certificatePem), nil
}

// GetOauthToken  retrieves an oauth token from the auth server by generating a challenge, signing it and submitting it
func (cs *CertificateService) GetOauthToken(ethAddress string, unitID uuid.UUID) (string, error) {
	// Init/generate challenge
	initParams := url.Values{}
	initParams.Set("domain", domain)
	initParams.Set("client_id", cs.oauthClientID)
	initParams.Set("response_type", "code")
	initParams.Set("scope", "openid email")
	initParams.Set("address", ethAddress)

	resp, err := http.PostForm(cs.oauthURL+generateChallenge, initParams)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var challengeResponse ChallengeResponse
	if err := json.Unmarshal(body, &challengeResponse); err != nil {
		return "", err
	}
	nonce := challengeResponse.Challenge
	cs.logger.Debug().Msgf("Challenge generated: %s", nonce)

	// Hash and sign challenge
	challenge := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(nonce), nonce)
	signedChallenge, err := signChallenge(challenge, unitID)
	if err != nil {
		return "", err
	}
	cs.logger.Debug().Msgf("challenge signed: %s ", signedChallenge)

	// Submit challenge
	state := challengeResponse.State
	submitParams := url.Values{}
	submitParams.Set("client_id", cs.oauthClientID)
	submitParams.Set("domain", domain)
	submitParams.Set("grant_type", "authorization_code")
	submitParams.Set("state", state)
	submitParams.Set("signature", signedChallenge)
	submitParams.Set("client_secret", cs.oauthClientSecret)

	resp, err = http.Post(cs.oauthURL+submitChallenge, "application/x-www-form-urlencoded", strings.NewReader(submitParams.Encode()))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Extract 'access_token' from the response body
	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", err
	}

	return tokenResp.AccessToken, nil
}

// CreateSignRequest is a helper function that given an x509 OTT returns a
// simple but secure sign request as well as the private key used. It is almost copy of ca.CreateSignRequest, the only difference
// that we set the CommonName to the etherAdd instead of the claims.Subject
func createSignRequest(ott string, etherAdd string) (*api.SignRequest, cr.PrivateKey, error) {
	token, err := jose.ParseSigned(ott)
	if err != nil {
		return nil, nil, errors.Wrap(err, "error parsing ott")
	}
	var claims authority.Claims
	if err := token.UnsafeClaimsWithoutVerification(&claims); err != nil {
		return nil, nil, errors.Wrap(err, "error parsing ott")
	}

	pk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, errors.Wrap(err, "error generating key")
	}

	dnsNames, ips, emails, uris := x509util.SplitSANs(claims.SANs)
	if claims.Email != "" {
		emails = append(emails, claims.Email)
	}

	template := &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName: etherAdd,
		},
		SignatureAlgorithm: x509.ECDSAWithSHA256,
		DNSNames:           dnsNames,
		IPAddresses:        ips,
		EmailAddresses:     emails,
		URIs:               uris,
	}

	csr, err := x509.CreateCertificateRequest(rand.Reader, template, pk)
	if err != nil {
		return nil, nil, errors.Wrap(err, "error creating certificate request")
	}
	cr, err := x509.ParseCertificateRequest(csr)
	if err != nil {
		return nil, nil, errors.Wrap(err, "error parsing certificate request")
	}
	if err := cr.CheckSignature(); err != nil {
		return nil, nil, errors.Wrap(err, "error signing certificate request")
	}
	return &api.SignRequest{
		CsrPEM: api.CertificateRequest{CertificateRequest: cr},
		OTT:    ott,
	}, pk, nil
}

// signChallenge signs the challenge message with the unitID
func signChallenge(message string, unitID uuid.UUID) (string, error) {
	// Hash the message
	keccak256Hash := crypto.Keccak256Hash([]byte(message))

	// Sign the hash
	sig, err := commands.SignHash(unitID, keccak256Hash.Bytes())
	if err != nil {
		return "", errors.Wrap(err, fmt.Sprintf("failed to sign the challenge for oauth with serial number: %s", unitID))
	}

	return "0x" + hex.EncodeToString(sig), nil
}
