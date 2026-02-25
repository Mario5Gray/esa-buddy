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

// Stream is the minimal interface the conversation layer needs to consume a
// streaming chat response. *openai.ChatCompletionStream satisfies it.
type Stream interface {
	Recv() (openai.ChatCompletionStreamResponse, error)
	Close() error
}

// Client is the minimal interface the conversation layer needs from an LLM
// backend. *openai.Client satisfies it; tests can provide a fake.
type Client interface {
	CreateChatCompletion(context.Context, openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error)
	CreateChatCompletionStream(context.Context, openai.ChatCompletionRequest) (Stream, error)
}

// openAIClientAdapter wraps *openai.Client to satisfy llm.Client.
// The only gap is CreateChatCompletionStream: the real client returns the
// concrete *openai.ChatCompletionStream; the adapter narrows it to llm.Stream.
type openAIClientAdapter struct{ c *openai.Client }

func (a *openAIClientAdapter) CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	return a.c.CreateChatCompletion(ctx, req)
}

func (a *openAIClientAdapter) CreateChatCompletionStream(ctx context.Context, req openai.ChatCompletionRequest) (Stream, error) {
	return a.c.CreateChatCompletionStream(ctx, req)
}

func SetupOpenAIClient(modelStr string, agentCfg agent.Agent, cfg *config.Config) (Client, error) {
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

	return &openAIClientAdapter{c: openai.NewClientWithConfig(llmConfig)}, nil
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
