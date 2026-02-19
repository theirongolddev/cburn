package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config holds all cburn configuration.
type Config struct {
	General  GeneralConfig  `toml:"general"`
	AdminAPI AdminAPIConfig `toml:"admin_api"`
	Budget   BudgetConfig   `toml:"budget"`
	Appearance AppearanceConfig `toml:"appearance"`
	Pricing  PricingOverrides `toml:"pricing"`
}

// GeneralConfig holds general preferences.
type GeneralConfig struct {
	DefaultDays      int    `toml:"default_days"`
	IncludeSubagents bool   `toml:"include_subagents"`
	ClaudeDir        string `toml:"claude_dir,omitempty"`
}

// AdminAPIConfig holds Anthropic Admin API settings.
type AdminAPIConfig struct {
	APIKey  string `toml:"api_key,omitempty"`
	BaseURL string `toml:"base_url,omitempty"`
}

// BudgetConfig holds budget tracking settings.
type BudgetConfig struct {
	MonthlyUSD *float64 `toml:"monthly_usd,omitempty"`
}

// AppearanceConfig holds theme settings.
type AppearanceConfig struct {
	Theme string `toml:"theme"`
}

// PricingOverrides allows user-defined pricing for specific models.
type PricingOverrides struct {
	Overrides map[string]ModelPricingOverride `toml:"overrides,omitempty"`
}

// ModelPricingOverride holds per-model pricing overrides.
type ModelPricingOverride struct {
	InputPerMTok        *float64 `toml:"input_per_mtok,omitempty"`
	OutputPerMTok       *float64 `toml:"output_per_mtok,omitempty"`
	CacheWrite5mPerMTok *float64 `toml:"cache_write_5m_per_mtok,omitempty"`
	CacheWrite1hPerMTok *float64 `toml:"cache_write_1h_per_mtok,omitempty"`
	CacheReadPerMTok    *float64 `toml:"cache_read_per_mtok,omitempty"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		General: GeneralConfig{
			DefaultDays:      30,
			IncludeSubagents: true,
		},
		Appearance: AppearanceConfig{
			Theme: "flexoki-dark",
		},
	}
}

// ConfigDir returns the XDG-compliant config directory.
func ConfigDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "cburn")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "cburn")
}

// ConfigPath returns the full path to the config file.
func ConfigPath() string {
	return filepath.Join(ConfigDir(), "config.toml")
}

// Load reads the config file, returning defaults if it doesn't exist.
func Load() (Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(ConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("reading config: %w", err)
	}

	if err := toml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parsing config: %w", err)
	}

	return cfg, nil
}

// Save writes the config to disk.
func Save(cfg Config) error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	f, err := os.OpenFile(ConfigPath(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("creating config file: %w", err)
	}
	defer f.Close()

	enc := toml.NewEncoder(f)
	return enc.Encode(cfg)
}

// GetAdminAPIKey returns the API key from env var or config, in that order.
func GetAdminAPIKey(cfg Config) string {
	if key := os.Getenv("ANTHROPIC_ADMIN_KEY"); key != "" {
		return key
	}
	return cfg.AdminAPI.APIKey
}

// Exists returns true if a config file exists on disk.
func Exists() bool {
	_, err := os.Stat(ConfigPath())
	return err == nil
}
