package redaction

import (
	"strings"
	"testing"
)

func TestSecretKeywordsPolicyRedactsJSONPairs(t *testing.T) {
	policy, _, err := BuildPolicy(Config{Kind: KindSecretKeywords}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	input := `{"api_key":"secret","token":"abc123","nested":{"password":"p@ss"}}`
	got, err := policy.Redact(Context{}, input)
	if err != nil {
		t.Fatalf("unexpected redact error: %v", err)
	}
	if got == input {
		t.Fatalf("expected redaction, got %q", got)
	}
	if containsSensitive(got, []string{"secret", "abc123", "p@ss"}) {
		t.Fatalf("expected sensitive values to be redacted, got %q", got)
	}
}

func TestSecretKeywordsPolicyRedactsKeyValuePairs(t *testing.T) {
	policy, _, err := BuildPolicy(Config{Kind: KindSecretKeywords}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	input := "API_KEY=secret Authorization: Bearer token123 password: letmein"
	got, err := policy.Redact(Context{}, input)
	if err != nil {
		t.Fatalf("unexpected redact error: %v", err)
	}
	if containsSensitive(got, []string{"secret", "token123", "letmein"}) {
		t.Fatalf("expected sensitive values to be redacted, got %q", got)
	}
}

func containsSensitive(text string, values []string) bool {
	for _, value := range values {
		if value != "" && strings.Contains(text, value) {
			return true
		}
	}
	return false
}
