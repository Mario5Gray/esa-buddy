package llm

import "strings"

// ModelContextTools provides model context window lookups.
type ModelContextTools struct {
	Overrides map[string]map[string]int
}

var defaultModelContextWindows = map[string]map[string]int{}

// ContextWindowTokens returns the context window size in tokens for the given
// provider and model. Overrides (from config model_context_windows) take
// precedence over built-in defaults.
//
// The TOML config uses nested tables to avoid bare-key restrictions on '/':
//
//	[model_context_windows.tabby]
//	"Qwen3-32B-EXL3-4.0bpw" = 32768
func (t ModelContextTools) ContextWindowTokens(provider, model string) (int, bool) {
	provider = strings.TrimSpace(provider)
	model = strings.TrimSpace(model)
	if provider == "" || model == "" {
		return 0, false
	}
	if t.Overrides != nil {
		if models, ok := t.Overrides[provider]; ok {
			if n, ok := models[model]; ok && n > 0 {
				return n, true
			}
		}
	}
	if models, ok := defaultModelContextWindows[provider]; ok {
		if n, ok := models[model]; ok && n > 0 {
			return n, true
		}
	}
	return 0, false
}
