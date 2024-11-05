package config

import (
	"errors"
	"log"

	"github.com/spf13/viper"
)

type Config struct {
	Ntfy  NtfyConfig  `mapstructure:"nfty"`
	Slack SlackConfig `mapstructure:"slack"`
	SMTP  SMTPConfig  `mapstructure:"smtp"`
}

type NtfyConfig struct {
	URL      string `mapstructure:"url"`
	Topic    string `mapstructure:"topic"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Token    string `mapstructure:"token"`
}

func (nc *NtfyConfig) Validate() error {
	if nc.Token != "" && (nc.Username != "" || nc.Password != "") {
		return errors.New("either Token or Username/Password should be specified, not both")
	}
	return nil
}

type SlackConfig struct {
	WebhookURL string `mapstructure:"webhook_url"`
}

type SMTPConfig struct {
	Server   string `mapstructure:"server"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

type StdoutConfig struct {
}

func LoadConfig(configFile string) (*Config, error) {
	viper.SetConfigFile(configFile)
	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}
	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}
	if err := config.Ntfy.Validate(); err != nil {
		return nil, err
	}
	log.Println("Configuration loaded successfully")
	return &config, nil
}
