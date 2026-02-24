package llm

import "testing"

func TestContextWindowTokens_returns_override_for_known_provider_model(t *testing.T) {
	tools := ModelContextTools{
		Overrides: map[string]map[string]int{
			"tabby": {"Qwen3-32B-EXL3-4.0bpw": 32768},
		},
	}
	n, ok := tools.ContextWindowTokens("tabby", "Qwen3-32B-EXL3-4.0bpw")
	if !ok {
		t.Fatal("expected match, got not found")
	}
	if n != 32768 {
		t.Errorf("expected 32768, got %d", n)
	}
}

func TestContextWindowTokens_returns_false_for_unknown_model(t *testing.T) {
	tools := ModelContextTools{
		Overrides: map[string]map[string]int{
			"tabby": {"Qwen3-32B-EXL3-4.0bpw": 32768},
		},
	}
	_, ok := tools.ContextWindowTokens("tabby", "unknown-model")
	if ok {
		t.Error("expected not found for unknown model")
	}
}

func TestContextWindowTokens_returns_false_for_unknown_provider(t *testing.T) {
	tools := ModelContextTools{
		Overrides: map[string]map[string]int{
			"tabby": {"Qwen3-32B-EXL3-4.0bpw": 32768},
		},
	}
	_, ok := tools.ContextWindowTokens("openai", "gpt-4o-mini")
	if ok {
		t.Error("expected not found for unknown provider")
	}
}

func TestContextWindowTokens_returns_false_for_empty_args(t *testing.T) {
	tools := ModelContextTools{}
	_, ok := tools.ContextWindowTokens("", "")
	if ok {
		t.Error("expected not found for empty args")
	}
}

func TestContextWindowTokens_nil_overrides_returns_false(t *testing.T) {
	tools := ModelContextTools{}
	_, ok := tools.ContextWindowTokens("tabby", "some-model")
	if ok {
		t.Error("expected not found when overrides is nil")
	}
}
