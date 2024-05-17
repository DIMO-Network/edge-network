package config

import (
	"fmt"
	"github.com/spf13/viper"
)

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

// edge-network binary without viper lib is ~ 8.1 Mb
// with viper lib - ~ 8.3 Mb
// todo consider to remove viper lib and use yaml.Unmarshal
func ReadConfig(profile string, confPath string) (*Config, error) {
	// read config file

	// shared repo/ loadConfig yaml// we need optimize binary size
	viper.SetConfigName("config") // name of config file (without extension)
	viper.AddConfigPath(confPath) // path to look for the config file in

	err := viper.ReadInConfig() // Find and read the config file
	if err != nil {             // Handle errors reading the config file
		return nil, fmt.Errorf("Fatal error config file: %s \n", err)
	}

	// Unmarshal the configuration into the Config struct
	var config Config
	err = viper.UnmarshalKey(profile, &config)
	if err != nil {
		return nil, fmt.Errorf("Unable to decode into struct, %v", err)
	}
	return &config, nil
}
