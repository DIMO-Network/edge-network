package config

import (
	"embed"
	"fmt"
	"github.com/DIMO-Network/shared"
	"io/fs"
	"os"
)

// Config represents the configuration for the edge-network
type Config struct {
	Mqtt     Mqtt     `yaml:"mqtt"`
	Services Services `yaml:"services"`
}

type Mqtt struct {
	Broker Broker `yaml:"broker"`
	Topics Topics `yaml:"topics"`
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
	Status  string `yaml:"status"`
	Network string `yaml:"network"`
	Logs    string `yaml:"logs"`
}

type Services struct {
	Auth     Auth     `yaml:"auth"`
	Ca       Ca       `yaml:"ca"`
	Identity Identity `yaml:"identity"`
	Vehicle  Vehicle  `yaml:"vehicle"`
}

type Auth struct {
	Host          string `yaml:"host"`
	ClientId      string `yaml:"clientId"`
	ClientSecret  string `yaml:"clientSecret"`
	CaFingerprint string `yaml:"caFingerprint"`
}

type Ca struct {
	Host string `yaml:"host"`
}

type Identity struct {
	Host string `yaml:"host"`
}

type Vehicle struct {
	Host string `yaml:"host"`
}

// ReadConfig reads the config file from the given path
func ReadConfig(configFiles embed.FS, configFileName string) (*Config, error) {
	// read config file from embed.FS
	data, err := fs.ReadFile(configFiles, configFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	filePathOnDisk := "/opt/autopi/config.yaml"
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
