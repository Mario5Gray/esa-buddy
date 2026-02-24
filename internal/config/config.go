package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/meain/esa/internal/utils"
)

// DefaultConfigPath is the default location for the global config file
const DefaultConfigPath = "~/.config/esa/config.toml"

// Settings represents global settings that can be overridden by CLI flags
type Settings struct {
	ShowCommands                bool            `toml:"show_commands"`
	ShowToolCalls               bool            `toml:"show_tool_calls"`
	DefaultModel                string          `toml:"default_model"`
	PromptCompaction            bool            `toml:"prompt_compaction"`
	CompactionMaxMessages       int             `toml:"compaction_max_messages"`
	CompactionKeepLast          int             `toml:"compaction_keep_last"`
	CompactionMaxChars          int             `toml:"compaction_max_chars"`
	CompactionTokenThresholdPct int             `toml:"compaction_token_threshold_pct"`
	CompactionRedactionPolicy   string          `toml:"compaction_redaction_policy"`
	CompactionRedaction         RedactionConfig `toml:"compaction_redaction"`
	RetryMaxAttempts            int             `toml:"retry_max_attempts"`
	RetryBaseDelayMs            int             `toml:"retry_base_delay_ms"`
	RetryMaxDelayMs             int             `toml:"retry_max_delay_ms"`
}

type RedactionConfig struct {
	Kind       string                  `toml:"kind"`
	ConfigFile string                  `toml:"config_file"`
	FailOpen   bool                    `toml:"fail_open"`
	Options    map[string]any          `toml:"options"`
	External   ExternalRedactionConfig `toml:"external"`
}

type ExternalRedactionConfig struct {
	URL       string `toml:"url"`
	TimeoutMs int    `toml:"timeout_ms"`
}

// Config represents the global configuration structure
type Config struct {
	ModelAliases        map[string]string         `toml:"model_aliases"`
	ModelContextWindows map[string]int            `toml:"model_context_windows"`
	Providers           map[string]ProviderConfig `toml:"providers"`
	Settings            Settings                  `toml:"settings"`
	ModelStrategy       ModelStrategy             `toml:"model_strategy"`
	Logging             LoggingConfig             `toml:"logging"`
}

type LoggingConfig struct {
	Level      string `toml:"level"`
	Format     string `toml:"format"`
	File       string `toml:"file"`
	ToStdout   bool   `toml:"to_stdout"`
	ToFile     bool   `toml:"to_file"`
	MaxAgeDays int    `toml:"max_age_days"`
	MaxSizeMB  int    `toml:"max_size_mb"`
	MaxBackups int    `toml:"max_backups"`
}

// ModelStrategy defines optional model selection by purpose/tool.
type ModelStrategy struct {
	Chat        string            `toml:"chat"`
	Summarize   string            `toml:"summarize"`
	ToolDefault string            `toml:"tool_default"`
	Tool        map[string]string `toml:"tool"`
}

// ProviderConfig represents the configuration for a model provider
type ProviderConfig struct {
	BaseURL           string            `toml:"base_url"`
	APIKeyEnvar       string            `toml:"api_key_envar"`
	AdditionalHeaders map[string]string `toml:"additional_headers"`
}

// LoadConfig loads the configuration from the specified path
func LoadConfig(configPath string) (*Config, error) {
	config := &Config{
		ModelAliases:        make(map[string]string),
		ModelContextWindows: make(map[string]int),
		Providers:           make(map[string]ProviderConfig),
	}

	// Expand home directory if needed
	if configPath == "" {
		configPath = DefaultConfigPath
	}
	configPath = utils.ExpandHomePath(configPath)

	// Create default config directory if it doesn't exist
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, err
	}

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Create default config file
		defaultConfig := Config{
			ModelAliases:        map[string]string{},
			ModelContextWindows: map[string]int{},
			Providers:           map[string]ProviderConfig{},
			Settings: Settings{
				ShowCommands:                false,
				ShowToolCalls:               false,
				DefaultModel:                "",
				PromptCompaction:            true,
				CompactionMaxMessages:       40,
				CompactionKeepLast:          12,
				CompactionMaxChars:          20000,
				CompactionTokenThresholdPct: 75,
				RetryMaxAttempts:            6,
				RetryBaseDelayMs:            1000,
				RetryMaxDelayMs:             60000,
			},
			Logging: LoggingConfig{
				Level:      "info",
				Format:     "text",
				File:       "",
				ToStdout:   false,
				ToFile:     true,
				MaxAgeDays: 30,
				MaxSizeMB:  50,
				MaxBackups: 0,
			},
		}
		file, err := os.Create(configPath)
		if err != nil {
			return nil, err
		}
		defer file.Close()

		if err := toml.NewEncoder(file).Encode(defaultConfig); err != nil {
			return nil, err
		}
		return &defaultConfig, nil
	}

	// Load existing config file
	if _, err := toml.DecodeFile(configPath, config); err != nil {
		return nil, err
	}

	return config, nil
}
