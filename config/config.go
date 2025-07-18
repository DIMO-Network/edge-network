package config

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"time"

	"github.com/DIMO-Network/edge-network/internal/util/retry"
	"github.com/rs/zerolog"

	"github.com/DIMO-Network/shared"
)

// Config represents the configuration for the edge-network
type Config struct {
	Mqtt     Mqtt     `yaml:"mqtt"`
	Services Services `yaml:"services"`
}

type Mqtt struct {
	Broker Broker `yaml:"broker"`
	Topics Topics `yaml:"topics"`
	Client Client `yaml:"client"`
}

type Broker struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
	TLS  TLS    `yaml:"tls"`
}

type TLS struct {
	Enabled bool `yaml:"enabled"`
}

type Topics struct {
	Status      string `yaml:"status"`
	Network     string `yaml:"network"`
	Logs        string `yaml:"logs"`
	Fingerprint string `yaml:"fingerprint"`
}

type Client struct {
	Buffering Buffering `yaml:"buffering"`
}

type Buffering struct {
	FileStore            string `yaml:"fileStore"`
	CleanSession         bool   `yaml:"cleanSession"`
	ConnectRetryInterval int    `yaml:"connectRetryInterval"`
	Limit                int    `yaml:"limit"`
}

type Services struct {
	Auth     Auth     `yaml:"auth"`
	Ca       Ca       `yaml:"ca"`
	Identity Identity `yaml:"identity"`
	Vehicle  Vehicle  `yaml:"vehicle"`
}

type Auth struct {
	Host                 string `yaml:"host"`
	ClientID             string `yaml:"clientId"`
	ClientSecret         string `yaml:"clientSecret"`
	GenerateChallengeURI string `yaml:"generateChallengeURI"`
	SubmitChallengeURI   string `yaml:"submitChallengeURI"`
}

type Ca struct {
	Host           string `yaml:"host"`
	CertPath       string `yaml:"certPath"`
	PrivateKeyPath string `yaml:"privateKeyPath"`
	CaFingerprint  string `yaml:"caFingerprint"`
}

type Identity struct {
	Host string `yaml:"host"`
}

type Vehicle struct {
	Host string `yaml:"host"`
}

// ReadConfig reads the configuration file from the embedded file system (configFiles),
// fetches remote configuration from the provided URL (configURL), and merges them.
func ReadConfig(logger zerolog.Logger, configFiles embed.FS, configURL, configFileName string) (*Config, error) {
	// read config file from embed.FS
	data, err := fs.ReadFile(configFiles, configFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	configPathOnDisk := "/opt/autopi/config.yaml"
	err = os.WriteFile(configPathOnDisk, data, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to write config file: %w", err)
	}

	config, err := ReadConfigFromPath(configPathOnDisk)

	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Get secrets from remote config
	remoteConfigPathOnDisk := "/opt/autopi/remote-config.json"
	// Retry for about 1 hour
	remoteConfig, err := retry.Retry[Config](11, 1*time.Second, logger, func() (interface{}, error) {
		return GetRemoteConfig(configURL, remoteConfigPathOnDisk)
	})

	if err != nil {
		return nil, fmt.Errorf("failed to read secrets file: %w", err)
	}

	config.Services.Auth.ClientID = remoteConfig.Services.Auth.ClientID
	config.Services.Auth.ClientSecret = remoteConfig.Services.Auth.ClientSecret
	config.Services.Ca.CaFingerprint = remoteConfig.Services.Ca.CaFingerprint

	return config, nil
}

// GetRemoteConfig sends a GET request to fetch the configuration and saves it to filePathOnDisk
func GetRemoteConfig(configURL string, filePathOnDisk string) (*Config, error) {

	// Check if the file already exists
	_, err := os.Stat(filePathOnDisk)
	if !os.IsNotExist(err) && err == nil {
		return ReadConfigFromPath(filePathOnDisk)
	}

	if configURL == "" {
		return nil, fmt.Errorf("configURL is empty")
	}

	// Send GET request
	resp, err := http.Get(configURL)
	if err != nil {
		return nil, fmt.Errorf("failed to send GET request: %w", err)
	}
	defer resp.Body.Close()

	// Check HTTP response status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET request failed with status code: %d", resp.StatusCode)
	}

	// Read response body
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Write response body to filePathOnDisk
	err = os.WriteFile(filePathOnDisk, data, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to write config file: %w", err)
	}

	return ReadConfigFromPath(filePathOnDisk)
}

// ReadConfigFromPath ReadConfig reads the config file from the given path
func ReadConfigFromPath(filePath string) (*Config, error) {
	// read config file
	config, err := shared.LoadConfig[Config](filePath)

	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	return &config, nil
}
