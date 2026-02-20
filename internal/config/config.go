// Package config handles cburn configuration loading, saving, and pricing.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config holds all cburn configuration.
type Config struct {
	General    GeneralConfig    `toml:"general"`
	AdminAPI   AdminAPIConfig   `toml:"admin_api"`
	ClaudeAI   ClaudeAIConfig   `toml:"claude_ai"`
	Budget     BudgetConfig     `toml:"budget"`
	Appearance AppearanceConfig `toml:"appearance"`
	Pricing    PricingOverrides `toml:"pricing"`
}

// GeneralConfig holds general preferences.
type GeneralConfig struct {
	DefaultDays      int    `toml:"default_days"`
	IncludeSubagents bool   `toml:"include_subagents"`
	ClaudeDir        string `toml:"claude_dir,omitempty"`
}

// AdminAPIConfig holds Anthropic Admin API settings.
type AdminAPIConfig struct {
	APIKey  string `toml:"api_key,omitempty"` //nolint:gosec // config field, not a secret
	BaseURL string `toml:"base_url,omitempty"`
}

// ClaudeAIConfig holds claude.ai session key settings for subscription data.
type ClaudeAIConfig struct {
	SessionKey string `toml:"session_key,omitempty"` //nolint:gosec // config field, not a secret
	OrgID      string `toml:"org_id,omitempty"`      // auto-cached after first fetch
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

// Dir returns the XDG-compliant config directory.
func Dir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "cburn")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "cburn")
}

// Path returns the full path to the config file.
func Path() string {
	return filepath.Join(Dir(), "config.toml")
}

// Load reads the config file, returning defaults if it doesn't exist.
func Load() (Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(Path())
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
	dir := Dir()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	f, err := os.OpenFile(Path(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("creating config file: %w", err)
	}
	enc := toml.NewEncoder(f)
	if err := enc.Encode(cfg); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// GetAdminAPIKey returns the API key from env var or config, in that order.
func GetAdminAPIKey(cfg Config) string {
	if key := os.Getenv("ANTHROPIC_ADMIN_KEY"); key != "" {
		return key
	}
	return cfg.AdminAPI.APIKey
}

// GetSessionKey returns the session key from env var or config, in that order.
func GetSessionKey(cfg Config) string {
	if key := os.Getenv("CLAUDE_SESSION_KEY"); key != "" {
		return key
	}
	return cfg.ClaudeAI.SessionKey
}

// Exists returns true if a config file exists on disk.
func Exists() bool {
	_, err := os.Stat(Path())
	return err == nil
}
