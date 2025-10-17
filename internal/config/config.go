package config

import (
	"fmt"
	"strings"

	"github.com/igodwin/notifier/internal/domain"
	"github.com/igodwin/notifier/internal/notifier"
	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	Server       ServerConfig       `mapstructure:"server"`
	Queue        domain.QueueConfig `mapstructure:"queue"`
	Notifiers    NotifiersConfig    `mapstructure:"notifiers"`
	Logging      LoggingConfig      `mapstructure:"logging"`
	Metrics      MetricsConfig      `mapstructure:"metrics"`
	HealthCheck  HealthCheckConfig  `mapstructure:"health_check"`
}

// ServerConfig contains server configuration
type ServerConfig struct {
	GRPCPort int    `mapstructure:"grpc_port"`
	RESTPort int    `mapstructure:"rest_port"`
	Host     string `mapstructure:"host"`
	Mode     string `mapstructure:"mode"` // "both", "grpc", "rest"
}

// NotifiersConfig contains configuration for all notifier types
type NotifiersConfig struct {
	SMTP   *notifier.SMTPConfig  `mapstructure:"smtp"`
	Slack  *notifier.SlackConfig `mapstructure:"slack"`
	Ntfy   *notifier.NtfyConfig  `mapstructure:"ntfy"`
	Stdout bool                  `mapstructure:"stdout"` // Enable stdout notifier
}

// LoggingConfig contains logging configuration
type LoggingConfig struct {
	Level      string `mapstructure:"level"`       // debug, info, warn, error
	Format     string `mapstructure:"format"`      // json, text
	OutputPath string `mapstructure:"output_path"` // stdout, stderr, or file path
}

// MetricsConfig contains metrics/observability configuration
type MetricsConfig struct {
	Enabled        bool   `mapstructure:"enabled"`
	Port           int    `mapstructure:"port"`
	Path           string `mapstructure:"path"`
	PrometheusEnabled bool `mapstructure:"prometheus_enabled"`
}

// HealthCheckConfig contains health check configuration
type HealthCheckConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Port     int    `mapstructure:"port"`
	Path     string `mapstructure:"path"`
	Interval int    `mapstructure:"interval"` // seconds
}

// Load loads configuration from file and environment variables
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set default values
	setDefaults(v)

	// Configure viper
	v.SetConfigName("config")
	v.SetConfigType("yaml")

	if configPath != "" {
		v.AddConfigPath(configPath)
	}

	// Also look in common locations
	v.AddConfigPath(".")
	v.AddConfigPath("./config")
	v.AddConfigPath("/etc/notifier")
	v.AddConfigPath("$HOME/.notifier")

	// Environment variable support
	v.SetEnvPrefix("NOTIFIER")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		// Config file is optional if environment variables are set
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// setDefaults sets default configuration values
func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("server.grpc_port", 50051)
	v.SetDefault("server.rest_port", 8080)
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.mode", "both")

	// Queue defaults
	v.SetDefault("queue.type", "local")
	v.SetDefault("queue.max_size", 10000)
	v.SetDefault("queue.worker_count", 10)
	v.SetDefault("queue.retry_attempts", 3)
	v.SetDefault("queue.retry_backoff", "exponential")

	// Local queue defaults
	v.SetDefault("queue.local.buffer_size", 1000)
	v.SetDefault("queue.local.persist_to_disk", false)

	// Logging defaults
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
	v.SetDefault("logging.output_path", "stdout")

	// Metrics defaults
	v.SetDefault("metrics.enabled", true)
	v.SetDefault("metrics.port", 9090)
	v.SetDefault("metrics.path", "/metrics")
	v.SetDefault("metrics.prometheus_enabled", true)

	// Health check defaults
	v.SetDefault("health_check.enabled", true)
	v.SetDefault("health_check.port", 8081)
	v.SetDefault("health_check.path", "/health")
	v.SetDefault("health_check.interval", 30)

	// Notifier defaults
	v.SetDefault("notifiers.stdout", true)
	v.SetDefault("notifiers.smtp.port", 587)
	v.SetDefault("notifiers.smtp.use_tls", true)
	v.SetDefault("notifiers.ntfy.server_url", "https://ntfy.sh")
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate server config
	if c.Server.GRPCPort < 1 || c.Server.GRPCPort > 65535 {
		return fmt.Errorf("invalid gRPC port: %d", c.Server.GRPCPort)
	}

	if c.Server.RESTPort < 1 || c.Server.RESTPort > 65535 {
		return fmt.Errorf("invalid REST port: %d", c.Server.RESTPort)
	}

	validModes := map[string]bool{"both": true, "grpc": true, "rest": true}
	if !validModes[c.Server.Mode] {
		return fmt.Errorf("invalid server mode: %s (must be both, grpc, or rest)", c.Server.Mode)
	}

	// Validate queue config
	validQueueTypes := map[string]bool{"local": true, "kafka": true}
	if !validQueueTypes[c.Queue.Type] {
		return fmt.Errorf("invalid queue type: %s (must be local or kafka)", c.Queue.Type)
	}

	if c.Queue.Type == "kafka" && c.Queue.Kafka == nil {
		return fmt.Errorf("Kafka queue type selected but no Kafka configuration provided")
	}

	// Validate at least one notifier is configured
	if !c.HasAnyNotifier() {
		return fmt.Errorf("at least one notifier must be configured")
	}

	return nil
}

// HasAnyNotifier checks if at least one notifier is configured
func (c *Config) HasAnyNotifier() bool {
	return c.Notifiers.Stdout ||
		c.Notifiers.SMTP != nil ||
		c.Notifiers.Slack != nil ||
		c.Notifiers.Ntfy != nil
}

// GetEnabledNotifiers returns a list of enabled notifier types
func (c *Config) GetEnabledNotifiers() []domain.NotificationType {
	var enabled []domain.NotificationType

	if c.Notifiers.Stdout {
		enabled = append(enabled, domain.TypeStdout)
	}
	if c.Notifiers.SMTP != nil {
		enabled = append(enabled, domain.TypeEmail)
	}
	if c.Notifiers.Slack != nil {
		enabled = append(enabled, domain.TypeSlack)
	}
	if c.Notifiers.Ntfy != nil {
		enabled = append(enabled, domain.TypeNtfy)
	}

	return enabled
}
