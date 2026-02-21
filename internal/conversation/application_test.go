package conversation

import (
	"strings"
	"testing"

	"github.com/meain/esa/internal/agent"
	"github.com/meain/esa/internal/config"
	"github.com/meain/esa/internal/options"
)

func TestParseModel(t *testing.T) {
	tests := []struct {
		name         string
		modelFlag    string
		config       *config.Config
		agent        agent.Agent
		wantProvider string
		wantModel    string
		wantInfo     ProviderInfo
	}{
		{
			name:         "OpenAI default model",
			modelFlag:    "openai/gpt-4",
			config:       nil,
			agent:        agent.Agent{},
			wantProvider: "openai",
			wantModel:    "gpt-4",
			wantInfo: ProviderInfo{
				BaseURL:     "https://api.openai.com/v1",
				APIKeyEnvar: "OPENAI_API_KEY",
			},
		},
		{
			name:      "Custom provider from config",
			modelFlag: "custom/model-1",
			config: &config.Config{
				Providers: map[string]config.ProviderConfig{
					"custom": {
						BaseURL:     "https://custom.api/v1",
						APIKeyEnvar: "CUSTOM_API_KEY",
					},
				},
			},
			agent:        agent.Agent{},
			wantProvider: "custom",
			wantModel:    "model-1",
			wantInfo: ProviderInfo{
				BaseURL:     "https://custom.api/v1",
				APIKeyEnvar: "CUSTOM_API_KEY",
			},
		},
		{
			name:      "Partial provider override in config",
			modelFlag: "openai/gpt-4",
			config: &config.Config{
				Providers: map[string]config.ProviderConfig{
					"openai": {
						BaseURL: "https://custom-openai.api/v2",
						// APIKeyEnvar not specified, should use default
					},
				},
			},
			agent:        agent.Agent{},
			wantProvider: "openai",
			wantModel:    "gpt-4",
			wantInfo: ProviderInfo{
				BaseURL:     "https://custom-openai.api/v2",
				APIKeyEnvar: "OPENAI_API_KEY", // Should keep default
			},
		},
		{
			name:      "Custom provider with builtin still available",
			modelFlag: "custom/model-1",
			config: &config.Config{
				Providers: map[string]config.ProviderConfig{
					"custom": {
						BaseURL:     "https://custom.api/v1",
						APIKeyEnvar: "CUSTOM_API_KEY",
					},
				},
			},
			agent:        agent.Agent{},
			wantProvider: "custom",
			wantModel:    "model-1",
			wantInfo: ProviderInfo{
				BaseURL:     "https://custom.api/v1",
				APIKeyEnvar: "CUSTOM_API_KEY",
			},
		},
		{
			name:         "Built-in provider unchanged",
			modelFlag:    "ollama/llama2",
			config:       &config.Config{}, // Empty config
			agent:        agent.Agent{},
			wantProvider: "ollama",
			wantModel:    "llama2",
			wantInfo: ProviderInfo{
				BaseURL:     "http://localhost:11434/v1",
				APIKeyEnvar: "OLLAMA_API_KEY",
			},
		},
		{
			name:      "Agent default model used when no CLI model provided",
			modelFlag: "",
			config:    nil,
			agent: agent.Agent{
				DefaultModel: "groq/llama3-8b",
			},
			wantProvider: "groq",
			wantModel:    "llama3-8b",
			wantInfo: ProviderInfo{
				BaseURL:     "https://api.groq.com/openai/v1",
				APIKeyEnvar: "GROQ_API_KEY",
			},
		},
		{
			name:      "CLI model overrides agent default model",
			modelFlag: "openai/gpt-4",
			config:    nil,
			agent: agent.Agent{
				DefaultModel: "groq/llama3-8b",
			},
			wantProvider: "openai",
			wantModel:    "gpt-4",
			wantInfo: ProviderInfo{
				BaseURL:     "https://api.openai.com/v1",
				APIKeyEnvar: "OPENAI_API_KEY",
			},
		},
		{
			name:      "Agent default model overrides config default model",
			modelFlag: "",
			config: &config.Config{
				Settings: config.Settings{
					DefaultModel: "ollama/codellama",
				},
			},
			agent: agent.Agent{
				DefaultModel: "groq/llama3-8b",
			},
			wantProvider: "groq",
			wantModel:    "llama3-8b",
			wantInfo: ProviderInfo{
				BaseURL:     "https://api.groq.com/openai/v1",
				APIKeyEnvar: "GROQ_API_KEY",
			},
		},
		{
			name:      "Agent default model overrides global fallback",
			modelFlag: "",
			config:    nil,
			agent: agent.Agent{
				DefaultModel: "ollama/llama3.2",
			},
			wantProvider: "ollama",
			wantModel:    "llama3.2",
			wantInfo: ProviderInfo{
				BaseURL:     "http://localhost:11434/v1",
				APIKeyEnvar: "OLLAMA_API_KEY",
			},
		},
		{
			name:      "Config default model used when agent has no default",
			modelFlag: "",
			config: &config.Config{
				Settings: config.Settings{
					DefaultModel: "ollama/codellama",
				},
			},
			agent:        agent.Agent{}, // No default model
			wantProvider: "ollama",
			wantModel:    "codellama",
			wantInfo: ProviderInfo{
				BaseURL:     "http://localhost:11434/v1",
				APIKeyEnvar: "OLLAMA_API_KEY",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &Application{
				modelFlag: tt.modelFlag,
				config:    tt.config,
				agent:     tt.agent,
			}
			gotProvider, gotModel, gotInfo := app.parseModel()
			if gotProvider != tt.wantProvider {
				t.Errorf("parseModel() provider = %v, want %v", gotProvider, tt.wantProvider)
			}
			if gotModel != tt.wantModel {
				t.Errorf("parseModel() model = %v, want %v", gotModel, tt.wantModel)
			}
			if gotInfo.APIKeyEnvar != tt.wantInfo.APIKeyEnvar {
				t.Errorf("parseModel() info.APIKeyEnvar = %+v, want %+v", gotInfo.APIKeyEnvar, tt.wantInfo.APIKeyEnvar)
			}
			if gotInfo.BaseURL != tt.wantInfo.BaseURL {
				t.Errorf("parseModel() info.BaseURL = %+v, want %+v", gotInfo.BaseURL, tt.wantInfo.BaseURL)
			}
			if gotInfo.BaseURL == "" {
				t.Errorf("parseModel() info baseURL should not be empty")
			}
			// Improved additionalHeaders test: check keys and values
			if len(gotInfo.AdditionalHeaders) != len(tt.wantInfo.AdditionalHeaders) {
				t.Errorf("parseModel() info.AdditionalHeaders len = %d, want %d", len(gotInfo.AdditionalHeaders), len(tt.wantInfo.AdditionalHeaders))
			}
			for k, v := range tt.wantInfo.AdditionalHeaders {
				gotVal, ok := gotInfo.AdditionalHeaders[k]
				if !ok {
					t.Errorf("parseModel() info.AdditionalHeaders missing key %q", k)
				} else if gotVal != v {
					t.Errorf("parseModel() info.AdditionalHeaders[%q] = %q, want %q", k, gotVal, v)
				}
			}
			for k := range gotInfo.AdditionalHeaders {
				if _, ok := tt.wantInfo.AdditionalHeaders[k]; !ok {
					t.Errorf("parseModel() info.AdditionalHeaders has unexpected key %q", k)
				}
			}
		})
	}

}

func TestProviderAdditionalHeadersMerging(t *testing.T) {
	cfg := &config.Config{
		Providers: map[string]config.ProviderConfig{
			"copilot": {
				AdditionalHeaders: map[string]string{
					"Copilot-Integration-Id": "custom-override",
					"X-Extra":                "foo",
				},
			},
		},
	}
	app := &Application{
		modelFlag: "copilot/some-model",
		config:    cfg,
		agent:     agent.Agent{},
	}
	_, _, info := app.parseModel()
	expected := map[string]string{
		"Content-Type":           "application/json",
		"Copilot-Integration-Id": "custom-override", // overridden
		"X-Extra":                "foo",
	}
	if len(info.AdditionalHeaders) != len(expected) {
		t.Errorf("additionalHeaders len = %d, want %d", len(info.AdditionalHeaders), len(expected))
	}
	for k, v := range expected {
		got, ok := info.AdditionalHeaders[k]
		if !ok {
			t.Errorf("missing additionalHeader %q", k)
		} else if got != v {
			t.Errorf("additionalHeader[%q] = %q, want %q", k, got, v)
		}
	}
}

func TestOllamaHostFromEnvironment(t *testing.T) {
	tests := []struct {
		name      string
		hostEnv   string
		wantHost  string
		needsTrim bool
	}{
		{
			name:     "Default host when env not set",
			hostEnv:  "",
			wantHost: "http://localhost:11434/v1",
		},
		{
			name:      "Custom host from environment",
			hostEnv:   "http://192.168.1.100:11434",
			wantHost:  "http://192.168.1.100:11434/v1",
			needsTrim: false,
		},
		{
			name:      "Custom host with trailing slash",
			hostEnv:   "http://ollama-server:11434/",
			wantHost:  "http://ollama-server:11434/v1",
			needsTrim: true,
		},
		{
			name:      "Host with existing /v1 path",
			hostEnv:   "http://custom-ollama:8000/v1",
			wantHost:  "http://custom-ollama:8000/v1",
			needsTrim: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("OLLAMA_HOST", tt.hostEnv)

			app := &Application{modelFlag: "ollama/llama2"}

			_, _, info := app.parseModel()

			if info.BaseURL != tt.wantHost {
				t.Errorf("Expected Ollama host %q, got %q", tt.wantHost, info.BaseURL)
			}
		})
	}
}

func TestEmptyApiKeyAcceptance(t *testing.T) {
	t.Setenv("OLLAMA_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")

	tests := []struct {
		name             string
		modelStr         string
		expectError      bool
		errorDescription string
	}{
		{
			name:             "Ollama accepts empty API key",
			modelStr:         "ollama/llama2",
			expectError:      false,
			errorDescription: "",
		},
		{
			name:             "OpenAI requires API key",
			modelStr:         "openai/gpt-4",
			expectError:      true,
			errorDescription: "OPENAI_API_KEY env not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := setupOpenAIClient(tt.modelStr, agent.Agent{}, &config.Config{})

			if (err != nil) != tt.expectError {
				t.Errorf("Expected error: %v, got: %v", tt.expectError, err)
			}

			if tt.expectError && err != nil && err.Error() != tt.errorDescription {
				t.Errorf("Expected error message %q, got %q", tt.errorDescription, err.Error())
			}
		})
	}
}

func TestFilterThinkTags(t *testing.T) {
	tests := []struct {
		name   string
		chunks []string
		want   string
	}{
		{
			name:   "Empty think block",
			chunks: []string{"<think>\n</think>\nHello!"},
			want:   "Hello!",
		},
		{
			name:   "Think block with content",
			chunks: []string{"<think>\nI should greet the user.\n</think>\nHello!"},
			want:   "Hello!",
		},
		{
			name:   "No think block",
			chunks: []string{"Hello, world!"},
			want:   "Hello, world!",
		},
		{
			name:   "Think tag split across chunks",
			chunks: []string{"<thi", "nk>\nthinking...\n</thi", "nk>\nHello!"},
			want:   "Hello!",
		},
		{
			name:   "Content before and after think block",
			chunks: []string{"Before ", "<think>inner</think>", " After"},
			want:   "Before  After",
		},
		{
			name:   "Multiple think blocks",
			chunks: []string{"A<think>x</think>B<think>y</think>C"},
			want:   "ABC",
		},
		{
			name:   "Think block streamed char by char",
			chunks: []string{"<", "t", "h", "i", "n", "k", ">", "x", "<", "/", "t", "h", "i", "n", "k", ">", "Hi"},
			want:   "Hi",
		},
		{
			name:   "Partial open tag that is not a think tag",
			chunks: []string{"<the", "n> done"},
			want:   "<then> done",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf strings.Builder
			inThink := false
			var thinkBuf strings.Builder
			var result strings.Builder

			for _, chunk := range tt.chunks {
				out := filterThinkTags(chunk, &inThink, &thinkBuf)
				result.WriteString(out)
			}

			// Flush remaining buffer (same logic as handleStreamResponse)
			if thinkBuf.Len() > 0 && !inThink {
				result.WriteString(thinkBuf.String())
			}
			_ = buf

			if got := result.String(); got != tt.want {
				t.Errorf("filterThinkTags() = %q, want %q", got, tt.want)
			}
		})
	}
}

func boolPtr(b bool) *bool { return &b }

func TestShouldThink(t *testing.T) {
	tests := []struct {
		name       string
		cliThink   *bool // app.thinkEnabled
		agentThink *bool // agent.Think
		want       bool
	}{
		{
			name: "Default: think enabled when nothing set",
			want: true,
		},
		{
			name:       "Agent sets think=false",
			agentThink: boolPtr(false),
			want:       false,
		},
		{
			name:       "Agent sets think=true",
			agentThink: boolPtr(true),
			want:       true,
		},
		{
			name:       "CLI --think overrides agent think=false",
			cliThink:   boolPtr(true),
			agentThink: boolPtr(false),
			want:       true,
		},
		{
			name:       "CLI --no-think overrides agent think=true",
			cliThink:   boolPtr(false),
			agentThink: boolPtr(true),
			want:       false,
		},
		{
			name:     "CLI --no-think with no agent config",
			cliThink: boolPtr(false),
			want:     false,
		},
		{
			name:     "CLI --think with no agent config",
			cliThink: boolPtr(true),
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &Application{
				agent:        agent.Agent{Think: tt.agentThink},
				thinkEnabled: tt.cliThink,
			}
			if got := app.shouldThink(); got != tt.want {
				t.Errorf("shouldThink() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoThinkAppendsToSystemPrompt(t *testing.T) {
	app := &Application{
		agent:        agent.Agent{SystemPrompt: "You are helpful."},
		thinkEnabled: boolPtr(false),
	}
	prompt, err := app.getSystemPrompt()
	if err != nil {
		t.Fatalf("getSystemPrompt error: %v", err)
	}
	if !strings.HasSuffix(prompt, "\n/no_think") {
		t.Errorf("Expected prompt to end with /no_think, got: %q", prompt)
	}
}

func TestThinkDoesNotAppendToSystemPrompt(t *testing.T) {
	app := &Application{
		agent:        agent.Agent{SystemPrompt: "You are helpful."},
		thinkEnabled: boolPtr(true),
	}
	prompt, err := app.getSystemPrompt()
	if err != nil {
		t.Fatalf("getSystemPrompt error: %v", err)
	}
	if strings.Contains(prompt, "/no_think") {
		t.Errorf("Expected prompt to NOT contain /no_think, got: %q", prompt)
	}
}

func TestSystemPromptOverrideFromCLI(t *testing.T) {
	// Agent with default system prompt
	agent := agent.Agent{
		SystemPrompt: "Default system prompt",
	}
	opts := &options.CLIOptions{
		SystemPrompt: "CLI system prompt override",
	}
	app := &Application{
		agent: agent,
	}
	// Simulate NewApplication logic
	if opts.SystemPrompt != "" {
		app.agent.SystemPrompt = opts.SystemPrompt
	}
	prompt, err := app.getSystemPrompt()
	if err != nil {
		t.Fatalf("getSystemPrompt error: %v", err)
	}
	if prompt != "CLI system prompt override" {
		t.Errorf("Expected system prompt to be overridden by CLI, got: %q", prompt)
	}
}
