package redaction

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExternalHTTPRedaction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		_ = json.NewDecoder(r.Body).Decode(&struct{}{})
		_ = json.NewEncoder(w).Encode(map[string]string{
			"redacted_text": "REDACTED",
		})
	}))
	defer server.Close()

	policy, _, err := BuildPolicy(Config{
		Kind: KindExternalHTTP,
		External: ExternalConfig{
			URL: server.URL,
		},
	}, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	output, err := policy.Redact(Context{ResourceType: "conversation"}, "secret")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if output != "REDACTED" {
		t.Fatalf("expected redacted response, got %q", output)
	}
}
