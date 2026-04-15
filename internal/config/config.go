package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

var ErrConfigNotLoaded = errors.New("configuration not loaded")

type Config struct {
	Endpoint    string `mapstructure:"endpoint"`
	TokenID     string `mapstructure:"token_id"`
	TokenSecret string `mapstructure:"token_secret"`
	Insecure    bool   `mapstructure:"insecure"`
}

func Load() (*Config, error) {
	v := viper.New()

	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.SetEnvPrefix("PXON")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	v.SetDefault("insecure", false)

	if err := bindEnv(v); err != nil {
		return nil, fmt.Errorf("bind environment variables: %w", err)
	}

	if home, err := os.UserHomeDir(); err == nil {
		v.AddConfigPath(filepath.Join(home, ".pxon"))
	}

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return nil, fmt.Errorf("read config file: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("decode configuration: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		if v.ConfigFileUsed() == "" {
			return nil, fmt.Errorf(
				"configuration not found or incomplete: %w; provide ~/.pxon/config.yaml or set PXON_ENDPOINT, PXON_TOKEN_ID and PXON_TOKEN_SECRET",
				err,
			)
		}

		return nil, fmt.Errorf("invalid configuration in %s: %w", v.ConfigFileUsed(), err)
	}

	return &cfg, nil
}

func bindEnv(v *viper.Viper) error {
	keys := []string{
		"endpoint",
		"token_id",
		"token_secret",
		"insecure",
	}

	for _, key := range keys {
		if err := v.BindEnv(key); err != nil {
			return err
		}
	}

	return nil
}

func (c Config) Validate() error {
	var missing []string

	if strings.TrimSpace(c.Endpoint) == "" {
		missing = append(missing, "endpoint")
	}

	if strings.TrimSpace(c.TokenID) == "" {
		missing = append(missing, "token_id")
	}

	if strings.TrimSpace(c.TokenSecret) == "" {
		missing = append(missing, "token_secret")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required values: %s", strings.Join(missing, ", "))
	}

	return nil
}
