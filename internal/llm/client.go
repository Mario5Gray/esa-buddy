package llm

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/meain/esa/internal/agent"
	"github.com/meain/esa/internal/config"
	"github.com/sashabaranov/go-openai"
)

// Client is the minimal interface the conversation layer needs from an LLM
// backend. *openai.Client satisfies it; tests can provide a fake.
type Client interface {
	CreateChatCompletion(context.Context, openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error)
	CreateChatCompletionStream(context.Context, openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error)
}

func SetupOpenAIClient(modelStr string, agentCfg agent.Agent, cfg *config.Config) (*openai.Client, error) {
	_, _, info := ParseModel(modelStr, agentCfg, cfg)

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
