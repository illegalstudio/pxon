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
	Endpoint                string        `mapstructure:"endpoint"`
	TokenID                 string        `mapstructure:"token_id"`
	TokenSecret             string        `mapstructure:"token_secret"`
	Insecure                bool          `mapstructure:"insecure"`
	DefaultStorage          string        `mapstructure:"default_storage"`
	DefaultImage            string        `mapstructure:"default_image"`
	DefaultDiskSize         string        `mapstructure:"default_disk_size"`
	DefaultPassword         string        `mapstructure:"default_password"`
	DefaultSSHPublicKeyPath string        `mapstructure:"default_ssh_public_key_path"`
	DefaultNet0             string        `mapstructure:"default_net0"`
	Network                 NetworkConfig `mapstructure:"network"`
}

type NetworkConfig struct {
	Mode       string `mapstructure:"mode"`
	Bridge     string `mapstructure:"bridge"`
	Gateway    string `mapstructure:"gateway"`
	Netmask    string `mapstructure:"netmask"`
	CIDR       int    `mapstructure:"cidr"`
	RangeStart string `mapstructure:"range_start"`
	RangeEnd   string `mapstructure:"range_end"`
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
		v.AddConfigPath(filepath.Dir(ConfigFilePathFromHome(home)))
		v.AddConfigPath(filepath.Dir(LegacyConfigFilePathFromHome(home)))
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
				"configuration not found or incomplete: %w; provide %s or set PXON_ENDPOINT, PXON_TOKEN_ID and PXON_TOKEN_SECRET",
				err,
				ConfigFilePath(),
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
		"default_storage",
		"default_image",
		"default_disk_size",
		"default_password",
		"default_ssh_public_key_path",
		"default_net0",
		"network.mode",
		"network.bridge",
		"network.gateway",
		"network.netmask",
		"network.cidr",
		"network.range_start",
		"network.range_end",
	}

	for _, key := range keys {
		if err := v.BindEnv(key); err != nil {
			return err
		}
	}

	return nil
}

func Save(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("missing configuration")
	}

	configPath := ConfigFilePath()
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")
	v.Set("endpoint", cfg.Endpoint)
	v.Set("token_id", cfg.TokenID)
	v.Set("token_secret", cfg.TokenSecret)
	v.Set("insecure", cfg.Insecure)
	v.Set("default_storage", cfg.DefaultStorage)
	v.Set("default_image", cfg.DefaultImage)
	v.Set("default_disk_size", cfg.DefaultDiskSize)
	v.Set("default_password", cfg.DefaultPassword)
	v.Set("default_ssh_public_key_path", cfg.DefaultSSHPublicKeyPath)
	v.Set("default_net0", cfg.DefaultNet0)
	v.Set("network.mode", cfg.Network.Mode)
	v.Set("network.bridge", cfg.Network.Bridge)
	v.Set("network.gateway", cfg.Network.Gateway)
	v.Set("network.netmask", cfg.Network.Netmask)
	v.Set("network.cidr", cfg.Network.CIDR)
	v.Set("network.range_start", cfg.Network.RangeStart)
	v.Set("network.range_end", cfg.Network.RangeEnd)

	if err := v.WriteConfigAs(configPath); err != nil {
		return fmt.Errorf("write config file %s: %w", configPath, err)
	}

	return nil
}

func ConfigFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".config", "pxon", "config.yaml")
	}

	return ConfigFilePathFromHome(home)
}

func ConfigFilePathFromHome(home string) string {
	return filepath.Join(home, ".config", "pxon", "config.yaml")
}

func LegacyConfigFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".pxon", "config.yaml")
	}

	return LegacyConfigFilePathFromHome(home)
}

func LegacyConfigFilePathFromHome(home string) string {
	return filepath.Join(home, ".pxon", "config.yaml")
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
