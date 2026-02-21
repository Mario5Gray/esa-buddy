package main

import (
	"log"
	"os"
	"strings"
)

func parseModel(modelStr string, agent Agent, config *Config) (provider string, model string, info providerInfo) {
	if modelStr == "" {
		if agent.DefaultModel != "" {
			modelStr = agent.DefaultModel
		} else if config.Settings.DefaultModel != "" {
			modelStr = config.Settings.DefaultModel
		} else {
			// Fallback to default model if nothing is specified
			modelStr = defaultModel
		}
	}

	// Check if the model string is an alias
	if config != nil {
		if aliasedModel, ok := config.ModelAliases[modelStr]; ok {
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
		info = providerInfo{
			baseURL:     "https://api.openai.com/v1",
			apiKeyEnvar: "OPENAI_API_KEY",
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

		info = providerInfo{
			baseURL:          host,
			apiKeyEnvar:      "OLLAMA_API_KEY",
			apiKeyCanBeEmpty: true, // Ollama does not require an API key by default
		}
	case "openrouter":
		info = providerInfo{
			baseURL:     "https://openrouter.ai/api/v1",
			apiKeyEnvar: "OPENROUTER_API_KEY",
		}
	case "groq":
		info = providerInfo{
			baseURL:     "https://api.groq.com/openai/v1",
			apiKeyEnvar: "GROQ_API_KEY",
		}
	case "github":
		info = providerInfo{
			baseURL:     "https://models.inference.ai.azure.com",
			apiKeyEnvar: "GITHUB_MODELS_API_KEY",
		}
	case "copilot":
		info = providerInfo{
			baseURL:     "https://api.githubcopilot.com",
			apiKeyEnvar: "COPILOT_API_KEY",
			additionalHeaders: map[string]string{
				"Content-Type":           "application/json",
				"Copilot-Integration-Id": "vscode-chat",
			},
		}
	}

	// Override with config if present
	if config != nil {
		if providerCfg, ok := config.Providers[provider]; ok {
			// Only override non-empty values
			if providerCfg.BaseURL != "" {
				info.baseURL = providerCfg.BaseURL
			}

			if providerCfg.APIKeyEnvar != "" {
				info.apiKeyEnvar = providerCfg.APIKeyEnvar
			}

			if len(providerCfg.AdditionalHeaders) > 0 {
				if info.additionalHeaders == nil {
					info.additionalHeaders = make(map[string]string)
				}

				for key, value := range providerCfg.AdditionalHeaders {
					info.additionalHeaders[key] = value
				}
			}
		}
	}

	return provider, model, info
}
