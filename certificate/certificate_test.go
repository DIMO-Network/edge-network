package certificate

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"reflect"
	"testing"
	"time"

	dimoConfig "github.com/DIMO-Network/edge-network/config"
	"github.com/google/uuid"
	"github.com/jarcoal/httpmock"
	"github.com/rs/zerolog"
	"github.com/smallstep/certificates/api"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestCertificateService_GetOauthToken(t *testing.T) {
	// given
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	const autoPiBaseURL = "http://192.168.4.1:9000"
	const etherAddr = "b794f5"
	var serial = uuid.New()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	logger := zerolog.New(os.Stdout).With().
		Timestamp().
		Str("app", "edge-network").
		Logger()

	// read config file
	config, confErr := dimoConfig.ReadConfigFromPath("../config-dev.yaml")
	if confErr != nil {
		logger.Fatal().Err(confErr).Msg("unable to read config file")
	}
	cs := NewCertificateService(logger, *config, nil, mockFileSystem())

	// when
	psPath := fmt.Sprintf("/dongle/%s/execute_raw", serial.String())
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+psPath,
		httpmock.NewStringResponder(200, `{"value": "0x064493aF03c949d58EE03Df0e771B6Eb19A1018A"}`))

	// Set up the expectation for the PostForm call
	httpmock.RegisterResponder(http.MethodPost, config.Services.Auth.Host+"/auth/web3/generate_challenge",
		httpmock.NewStringResponder(200, `{"state": "oae7fkpeyxdatezkac5lzmo2p","challenge": "auth.dimo.zone wants you to sign in with your Ethereum account:\n0x064493aF03c949d58EE03Df0e771B6Eb19A1018A\n\n127.0.0.1 is asking you sign in.\n\nURI: https://auth.dimo.zone\nVersion: 1\nChain ID: 1\nNonce: zrIC3hmEvCsv8exZxjsMBYhEciu7oB\nIssued At: 2024-05-09T16:11:21Z"}`))
	// set up the expectation for the Post call
	httpmock.RegisterResponder(http.MethodPost, config.Services.Auth.Host+"/auth/web3/submit_challenge",
		httpmock.NewStringResponder(200, `{"access_token": "eyJhbGciOiJSUzI1NiIsImtpZCI6ImMzZWVhNzJjNDFjMzJlMjg2YThhZTc3ZmE5OTA1NmQ2YjA3ZjAxMjUifQ.eyJpc3MiOiJodHRwczovL2F1dGguZGV2LmRpbW8uem9uZSIsInByb3ZpZGVyX2lkIjoid2ViMyIsInN1YiI6IkNpb3dlRGs0UkRjNFpEY3hNVU13WldNMU5EUkdObVppTldRMU5HWmpaalkxTlRsRFJqUXhOVFEyWVRrU0JIZGxZak0iLCJhdWQiOiJzdGVwLWNhIiwiZXhwIjoxNzE2NDg5MzgyLCJpYXQiOjE3MTUyNzk3ODIsImF0X2hhc2giOiJNeDNJc3F4T2xYN0w0WVlyMVFsWFN3IiwiZW1haWxfdmVyaWZpZWQiOmZhbHNlLCJldGhlcmV1bV9hZGRyZXNzIjoiMHg5OEQ3OGQ3MTFDMGVjNTQ0RjZmYjVkNTRmY2Y2NTU5Q0Y0MTU0NmE5In0.nOgnxTtAHTX-HKaRet1yAKvIC91XehgS33MrdGUAWrdgmDWhfJykevMlnQxolDrykE8-foTDaB-ePpbr1vtcMfQ2cPhGZTJyI0nWEGNUK0qEYO4tMzgBwUGtTL6-CR3q_qTLu7DJ71_znbYxKgzVJHvsOJEju_vDKo9g2gtoaAUqUC_xN12jyhOsjn1ZVBEaXfkduoLtJgB5RdmoD8P-PGArkccBGwSKc6iCO8M2UH901WfdL8Zoh8D1-jqwaq-KdNAvyumj4viWPHys0mAXCnEqgmlfXcBaFSuNhLUck1G7Tjgs6KfYY6QkSGJapCo-RsuI5DD3jWTh396bR6o0iw"}`))

	// then
	token, err := cs.GetOauthToken(etherAddr, serial)

	// verify
	assert.NoError(t, err)
	assert.True(t, token != "")
}

func TestCertificateService_SignWeb3Certificate(t *testing.T) {
	// given
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	const autoPiBaseURL = "http://192.168.4.1:9000"
	const etherAddr = "b794f5"
	var serial = uuid.New()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockSigner := NewMockSigner(mockCtrl)

	logger := zerolog.New(os.Stdout).With().
		Timestamp().
		Str("app", "edge-network").
		Logger()

	// read config file
	config, confErr := dimoConfig.ReadConfigFromPath("../config-dev.yaml")
	if confErr != nil {
		logger.Fatal().Err(confErr).Msg("unable to read config file")
	}
	cs := NewCertificateService(logger, *config, mockSigner, mockFileSystem())

	// when
	psPath := fmt.Sprintf("/dongle/%s/execute_raw", serial.String())
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+psPath,
		httpmock.NewStringResponder(200, `{"value": "0x064493aF03c949d58EE03Df0e771B6Eb19A1018A"}`))

	// set up the expectation for the Post call to "https://ca.dimo.zone"
	cert := generateCert()
	mockSigner.EXPECT().Sign(gomock.Any()).Return(&api.SignResponse{ServerPEM: api.Certificate{Certificate: cert}}, nil)
	// Set up the expectation for the PostForm call
	httpmock.RegisterResponder(http.MethodPost, config.Services.Auth.Host+"/auth/web3/generate_challenge",
		httpmock.NewStringResponder(200, `{"state": "oae7fkpeyxdatezkac5lzmo2p","challenge": "auth.dimo.zone wants you to sign in with your Ethereum account:\n0x064493aF03c949d58EE03Df0e771B6Eb19A1018A\n\n127.0.0.1 is asking you sign in.\n\nURI: https://auth.dimo.zone\nVersion: 1\nChain ID: 1\nNonce: zrIC3hmEvCsv8exZxjsMBYhEciu7oB\nIssued At: 2024-05-09T16:11:21Z"}`))
	// set up the expectation for the Post call
	httpmock.RegisterResponder(http.MethodPost, config.Services.Auth.Host+"/auth/web3/submit_challenge",
		httpmock.NewStringResponder(200, `{"access_token": "eyJhbGciOiJSUzI1NiIsImtpZCI6ImMzZWVhNzJjNDFjMzJlMjg2YThhZTc3ZmE5OTA1NmQ2YjA3ZjAxMjUifQ.eyJpc3MiOiJodHRwczovL2F1dGguZGV2LmRpbW8uem9uZSIsInByb3ZpZGVyX2lkIjoid2ViMyIsInN1YiI6IkNpb3dlRGs0UkRjNFpEY3hNVU13WldNMU5EUkdObVppTldRMU5HWmpaalkxTlRsRFJqUXhOVFEyWVRrU0JIZGxZak0iLCJhdWQiOiJzdGVwLWNhIiwiZXhwIjoxNzE2NDg5MzgyLCJpYXQiOjE3MTUyNzk3ODIsImF0X2hhc2giOiJNeDNJc3F4T2xYN0w0WVlyMVFsWFN3IiwiZW1haWxfdmVyaWZpZWQiOmZhbHNlLCJldGhlcmV1bV9hZGRyZXNzIjoiMHg5OEQ3OGQ3MTFDMGVjNTQ0RjZmYjVkNTRmY2Y2NTU5Q0Y0MTU0NmE5In0.nOgnxTtAHTX-HKaRet1yAKvIC91XehgS33MrdGUAWrdgmDWhfJykevMlnQxolDrykE8-foTDaB-ePpbr1vtcMfQ2cPhGZTJyI0nWEGNUK0qEYO4tMzgBwUGtTL6-CR3q_qTLu7DJ71_znbYxKgzVJHvsOJEju_vDKo9g2gtoaAUqUC_xN12jyhOsjn1ZVBEaXfkduoLtJgB5RdmoD8P-PGArkccBGwSKc6iCO8M2UH901WfdL8Zoh8D1-jqwaq-KdNAvyumj4viWPHys0mAXCnEqgmlfXcBaFSuNhLUck1G7Tjgs6KfYY6QkSGJapCo-RsuI5DD3jWTh396bR6o0iw"}`))

	// then
	certFromServer, err := cs.SignWeb3Certificate(etherAddr, true, serial)

	// verify
	assert.NoError(t, err)
	assert.True(t, certFromServer != "")
}

// helper function to mock Filesystem
func mockFileSystem() *MockFileSystem {
	system := MockFileSystem{
		WriteFileFunc: func(_ string, _ []byte, _ os.FileMode) error {
			return nil
		},
		ReadFileFunc: func(_ string) ([]byte, error) {
			return nil, nil
		},
		StatFileFunc: func(_ string) (os.FileInfo, error) {
			return nil, nil
		},
		IsNotExistFunc: func(_ error) bool {
			return false
		},
	}
	return &system
}

// helper function to generate a certificate
func generateCert() *x509.Certificate {
	// Generate a new private key
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}

	// Create a template for the certificate
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "test",
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(time.Hour * 24 * 365),
		KeyUsage:  x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
		BasicConstraintsValid: true,
	}

	// Create the certificate
	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		panic(err)
	}

	// Parse the DER format byte slice to an x509.Certificate
	cert, err := x509.ParseCertificate(derBytes)
	if err != nil {
		panic(err)
	}

	return cert
}

// MockSigner is a mock of Signer interface.
type MockSigner struct {
	ctrl     *gomock.Controller
	recorder *MockSignerMockRecorder
}

// MockSignerMockRecorder is the mock recorder for MockSigner.
type MockSignerMockRecorder struct {
	mock *MockSigner
}

// NewMockSigner creates a new mock instance.
func NewMockSigner(ctrl *gomock.Controller) *MockSigner {
	mock := &MockSigner{ctrl: ctrl}
	mock.recorder = &MockSignerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSigner) EXPECT() *MockSignerMockRecorder {
	return m.recorder
}

// Sign mocks base method.
func (m *MockSigner) Sign(req *api.SignRequest) (*api.SignResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Sign", req)
	ret0, _ := ret[0].(*api.SignResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Sign indicates an expected call of Sign.
func (mr *MockSignerMockRecorder) Sign(req interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Sign", reflect.TypeOf((*MockSigner)(nil).Sign), req)
}

// MockFileSystem is a mock of FileSystem interface.
type MockFileSystem struct {
	WriteFileFunc  func(filename string, data []byte, perm os.FileMode) error
	ReadFileFunc   func(filename string) ([]byte, error)
	StatFileFunc   func(name string) (os.FileInfo, error)
	IsNotExistFunc func(err error) bool
}

func (m MockFileSystem) WriteFile(filename string, data []byte, perm os.FileMode) error {
	return m.WriteFileFunc(filename, data, perm)
}

func (m MockFileSystem) ReadFile(filename string) ([]byte, error) {
	return m.ReadFileFunc(filename)
}

func (m MockFileSystem) Stat(name string) (os.FileInfo, error) {
	return m.StatFileFunc(name)
}

func (m MockFileSystem) IsNotExist(err error) bool {
	return m.IsNotExistFunc(err)
}
