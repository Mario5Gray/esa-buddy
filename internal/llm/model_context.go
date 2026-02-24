package llm

import "strings"

// ModelContextTools provides model context window lookups.
type ModelContextTools struct {
	Overrides map[string]int
}

var defaultModelContextWindows = map[string]int{}

// ContextWindowTokens returns the context window for the given model string
// in provider/model format. Overrides take precedence over defaults.
func (t ModelContextTools) ContextWindowTokens(modelString string) (int, bool) {
	modelString = strings.TrimSpace(modelString)
	if modelString == "" {
		return 0, false
	}
	if t.Overrides != nil {
		if n, ok := t.Overrides[modelString]; ok && n > 0 {
			return n, true
		}
	}
	if n, ok := defaultModelContextWindows[modelString]; ok && n > 0 {
		return n, true
	}
	return 0, false
}
