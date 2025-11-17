package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds application configuration.
type Config struct {
	App         App    `mapstructure:"app"`
	Retry       Retry  `mapstructure:"retry"`
	DatabaseURL string `mapstructure:"database_url"`
}

// App contains general application settings.
type App struct {
	Port            string        `mapstructure:"port"`             // HTTP server port
	MirgationDir    string        `mapstructure:"migration_dir"`    // Directory for DB migrations
	LogLevel        string        `mapstructure:"log_level"`        // Log level (e.g., debug, info, error)
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"` // Timeout for graceful shutdown
}

// Retry holds retry strategy configuration.
type Retry struct {
	Backoff     string        `mapstructure:"backoff"`      // Backoff type: fixed, linear, exponential
	Base        time.Duration `mapstructure:"base"`         // Base duration for backoff
	Factor      float64       `mapstructure:"factor"`       // Exponential factor
	Max         time.Duration `mapstructure:"max"`          // Maximum wait duration
	MaxAttempts int           `mapstructure:"max_attempts"` // Max retry attempts
	Jitter      float64       `mapstructure:"jitter"`       // Random jitter fraction
}

// Load reads configuration from file or environment variables.
// Config file is optional; environment variables override file values.
func Load(configFilePath string) (*Config, error) {
	v := viper.New()

	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.BindEnv("database_url")
	v.BindEnv("app.migration_dir")

	if configFilePath != "" {
		v.SetConfigFile(configFilePath)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	v.SetDefault("app.port", "8080")
	v.SetDefault("app.shutdown_timeout", "5s")
	v.SetDefault("retry.max_attempts", 3)
	v.SetDefault("retry.backoff", "fixed")
	v.SetDefault("retry.jitter", 0.0)

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}
