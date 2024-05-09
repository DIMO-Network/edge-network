package oauth

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
)

type ChallengeResponse struct {
	State     string `json:"state"`
	Challenge string `json:"challenge"`
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
}

const generateChallenge = "/auth/web3/generate_challenge"
const submitChallenge = "/auth/web3/submit_challenge"
const domain = "http://127.0.0.1:10000"

// GetOauthToken  retrieves an oauth token from the auth server by generating a challenge, signing it and submitting it
func GetOauthToken(logger zerolog.Logger, oauthUrl, oauthClientId, oauthClientSecret, ethAddress string, unitID uuid.UUID) (string, error) {
	// Init/generate challenge
	initParams := url.Values{}
	initParams.Set("domain", domain)
	initParams.Set("client_id", oauthClientId)
	initParams.Set("response_type", "code")
	initParams.Set("scope", "openid email")
	initParams.Set("address", ethAddress)

	resp, err := http.PostForm(oauthUrl+generateChallenge, initParams)
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
	logger.Debug().Msgf("Challenge generated: %s", nonce)

	// Hash and sign challenge
	challenge := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(nonce), nonce)
	signedChallenge, err := signChallenge(challenge, unitID)
	if err != nil {
		return "", err
	}
	logger.Debug().Msgf("challenge signed: %s ", signedChallenge)

	// Submit challenge
	state := challengeResponse.State
	submitParams := url.Values{}
	submitParams.Set("client_id", oauthClientId)
	submitParams.Set("domain", domain)
	submitParams.Set("grant_type", "authorization_code")
	submitParams.Set("state", state)
	submitParams.Set("signature", signedChallenge)
	submitParams.Set("client_secret", oauthClientSecret)

	resp, err = http.Post(oauthUrl+submitChallenge, "application/x-www-form-urlencoded", strings.NewReader(submitParams.Encode()))
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

// SignWeb3Certificate exchanges an JWT  for a signed certificate
func SignWeb3Certificate(ethAddress, oauthUrl, oauthClientId, oauthClientSecret, caUrl, caFingerprint, certificatePath string, confirm bool, logger zerolog.Logger, unitID uuid.UUID) (string, error) {
	// duplicated from python code, not sure if we need it
	if !confirm {
		return "", errors.New("This command will create and sign a new client certificate - add parameter 'confirm=true' to continue anyway")
	}

	token, err := GetOauthToken(logger, oauthUrl, oauthClientId, oauthClientSecret, ethAddress, unitID)
	if err != nil {
		return "", err
	}

	logger.Debug().Msgf("Token response: %s", token)

	// Create a new client for the CA
	stepCa, err := ca.NewClient(caUrl, ca.WithRootSHA256(caFingerprint))
	if err != nil {
		return "", err
	}
	// Generate a new sign request with a randomly generated key.
	req, _, err := CreateSignRequest(token, ethAddress)
	if err != nil {
		panic(err)
	}

	certificate, err := stepCa.Sign(req)
	if err != nil {
		return "", err
	}

	certificatePem := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.CaPEM.Raw})

	// Save certificate
	err = os.WriteFile(certificatePath, certificatePem, 0644)
	if err != nil {
		return "", err
	}

	return string(certificatePem), nil
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

// CreateSignRequest is a helper function that given an x509 OTT returns a
// simple but secure sign request as well as the private key used. It is almost copy of ca.CreateSignRequest, the only difference
// that we set the CommonName to the etherAdd instead of the claims.Subject
func CreateSignRequest(ott string, etherAdd string) (*api.SignRequest, cr.PrivateKey, error) {
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
