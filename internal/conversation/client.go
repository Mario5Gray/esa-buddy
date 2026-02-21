package conversation

import (
	"fmt"
	"math"
	"net/http"
	"os"
	"time"

	"github.com/meain/esa/internal/agent"
	"github.com/meain/esa/internal/config"
	"github.com/sashabaranov/go-openai"
)

func setupOpenAIClient(modelStr string, agentCfg agent.Agent, cfg *config.Config) (*openai.Client, error) {
	_, _, info := parseModel(modelStr, agentCfg, cfg)

	configuredAPIKey := os.Getenv(info.APIKeyEnvar)
	// Key name can be empty if we don't need any keys
	if info.APIKeyEnvar != "" && configuredAPIKey == "" && !info.APIKeyCanBeEmpty {
		return nil, fmt.Errorf(info.APIKeyEnvar + " env not found")
	}

	llmConfig := openai.DefaultConfig(configuredAPIKey)
	llmConfig.BaseURL = info.BaseURL

	if len(info.AdditionalHeaders) != 0 {
		httpClient := &http.Client{
			Transport: &transportWithCustomHeaders{
				headers: info.AdditionalHeaders,
				base:    http.DefaultTransport,
			},
		}

		llmConfig.HTTPClient = httpClient
	}

	client := openai.NewClientWithConfig(llmConfig)

	return client, nil
}

type transportWithCustomHeaders struct {
	headers map[string]string
	base    http.RoundTripper
}

func (t *transportWithCustomHeaders) RoundTrip(req *http.Request) (*http.Response, error) {
	for key, value := range t.headers {
		req.Header.Set(key, value)
	}
	return t.base.RoundTrip(req)
}

// calculateRetryDelay calculates exponential backoff delay with jitter
func calculateRetryDelay(attempt int) time.Duration {
	// Exponential backoff: baseDelay * 2^attempt
	delay := time.Duration(float64(baseRetryDelay) * math.Pow(2, float64(attempt)))

	// Cap the delay
	if delay > maxRetryDelay {
		delay = maxRetryDelay
	}

	return delay
}
