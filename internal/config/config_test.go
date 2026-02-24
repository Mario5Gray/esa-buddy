package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary directory for test config
	tmpDir, err := os.MkdirTemp("", "esa-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.toml")

	// Test loading default config when file doesn't exist
	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Test loading custom config
	customConfig := `
model_aliases = { "custom" = "custom/model" }
[providers]
[providers.custom]
base_url = "https://custom.api/v1"
api_key_envar = "CUSTOM_API_KEY"
`
	if err := os.WriteFile(configPath, []byte(customConfig), 0644); err != nil {
		t.Fatalf("Failed to write custom config: %v", err)
	}

	config, err = LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed for custom config: %v", err)
	}

	// Verify custom model alias
	if config.ModelAliases["custom"] != "custom/model" {
		t.Errorf("Expected custom alias to be custom/model, got %s", config.ModelAliases["custom"])
	}

	// Verify custom provider
	custom, exists := config.Providers["custom"]
	if !exists {
		t.Error("Expected custom provider to exist")
	}
	if custom.BaseURL != "https://custom.api/v1" {
		t.Errorf("Expected custom BaseURL to be https://custom.api/v1, got %s", custom.BaseURL)
	}
	if custom.APIKeyEnvar != "CUSTOM_API_KEY" {
		t.Errorf("Expected custom APIKeyEnvar to be CUSTOM_API_KEY, got %s", custom.APIKeyEnvar)
	}
}

func TestLoadConfig_model_context_windows_nested_table(t *testing.T) {
	// TOML bare keys disallow '/'. The model_context_windows section must use
	// nested tables: [model_context_windows.<provider>] with model name keys.
	// This test asserts the config struct accepts that shape.
	tmpDir, err := os.MkdirTemp("", "esa-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.toml")
	tomlContent := `
[model_context_windows.tabby]
"Qwen3-32B-EXL3-4.0bpw" = 32768

[model_context_windows.openai]
"gpt-4o-mini" = 128000
`
	if err := os.WriteFile(configPath, []byte(tomlContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	tabby, ok := config.ModelContextWindows["tabby"]
	if !ok {
		t.Fatal("expected provider 'tabby' in model_context_windows")
	}
	if tabby["Qwen3-32B-EXL3-4.0bpw"] != 32768 {
		t.Errorf("expected 32768 for Qwen3-32B-EXL3-4.0bpw, got %d", tabby["Qwen3-32B-EXL3-4.0bpw"])
	}

	openai, ok := config.ModelContextWindows["openai"]
	if !ok {
		t.Fatal("expected provider 'openai' in model_context_windows")
	}
	if openai["gpt-4o-mini"] != 128000 {
		t.Errorf("expected 128000 for gpt-4o-mini, got %d", openai["gpt-4o-mini"])
	}
}
