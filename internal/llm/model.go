package llm

import (
	"log"
	"os"
	"strings"

	"github.com/meain/esa/internal/agent"
	"github.com/meain/esa/internal/config"
)

// DefaultModel is the fallback model when none is specified.
const DefaultModel = "openai/gpt-5.2-2025-12-11"

// ProviderInfo contains provider-specific configuration.
type ProviderInfo struct {
	BaseURL           string
	APIKeyEnvar       string
	APIKeyCanBeEmpty  bool
	AdditionalHeaders map[string]string
}

func ParseModel(modelStr string, agentCfg agent.Agent, cfg *config.Config) (provider string, model string, info ProviderInfo) {
	if modelStr == "" {
		if agentCfg.DefaultModel != "" {
			modelStr = agentCfg.DefaultModel
		} else if cfg.Settings.DefaultModel != "" {
			modelStr = cfg.Settings.DefaultModel
		} else {
			// Fallback to default model if nothing is specified
			modelStr = DefaultModel
		}
	}

	// Check if the model string is an alias
	if cfg != nil {
		if aliasedModel, ok := cfg.ModelAliases[modelStr]; ok {
			modelStr = aliasedModel
		}
	}

	parts := strings.SplitN(modelStr, "/", 2)
	if len(parts) != 2 {
		log.Fatalf("invalid model format %q - must be provider/model", modelStr)
	}

	provider = parts[0]
	model = parts[1]

	// Start with default provider info
	switch provider {
	case "openai":
		info = ProviderInfo{
			BaseURL:     "https://api.openai.com/v1",
			APIKeyEnvar: "OPENAI_API_KEY",
		}
	case "ollama":
		host := os.Getenv("OLLAMA_HOST")
		if host == "" {
			host = "http://localhost:11434"
		}

		if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
			host = "http://" + host
		}

		if !strings.HasSuffix(host, "/v1") {
			host = strings.TrimSuffix(host, "/") + "/v1"
		}

		info = ProviderInfo{
			BaseURL:          host,
			APIKeyEnvar:      "OLLAMA_API_KEY",
			APIKeyCanBeEmpty: true, // Ollama does not require an API key by default
		}
	case "openrouter":
		info = ProviderInfo{
			BaseURL:     "https://openrouter.ai/api/v1",
			APIKeyEnvar: "OPENROUTER_API_KEY",
		}
	case "groq":
		info = ProviderInfo{
			BaseURL:     "https://api.groq.com/openai/v1",
			APIKeyEnvar: "GROQ_API_KEY",
		}
	case "github":
		info = ProviderInfo{
			BaseURL:     "https://models.inference.ai.azure.com",
			APIKeyEnvar: "GITHUB_MODELS_API_KEY",
		}
	case "copilot":
		info = ProviderInfo{
			BaseURL:     "https://api.githubcopilot.com",
			APIKeyEnvar: "COPILOT_API_KEY",
			AdditionalHeaders: map[string]string{
				"Content-Type":           "application/json",
				"Copilot-Integration-Id": "vscode-chat",
			},
		}
	}

	// Override with config if present
	if cfg != nil {
		if providerCfg, ok := cfg.Providers[provider]; ok {
			// Only override non-empty values
			if providerCfg.BaseURL != "" {
				info.BaseURL = providerCfg.BaseURL
			}

			if providerCfg.APIKeyEnvar != "" {
				info.APIKeyEnvar = providerCfg.APIKeyEnvar
			}

			if len(providerCfg.AdditionalHeaders) > 0 {
				if info.AdditionalHeaders == nil {
					info.AdditionalHeaders = make(map[string]string)
				}

				for key, value := range providerCfg.AdditionalHeaders {
					info.AdditionalHeaders[key] = value
				}
			}
		}
	}

	return provider, model, info
}
