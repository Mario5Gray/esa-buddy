package redaction

import (
	"errors"
	"testing"
)

type errorPolicy struct{}

func (errorPolicy) Name() string { return "test/error" }

func (errorPolicy) Redact(_ Context, text string) (string, error) {
	return text, errors.New("redaction failed")
}

func TestBuildPolicyUsesLegacyName(t *testing.T) {
	policy, name, err := BuildPolicy(Config{}, PolicyNone)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if name != PolicyNone {
		t.Fatalf("expected policy name %q, got %q", PolicyNone, name)
	}
	if policy == nil || policy.Name() != PolicyNone {
		t.Fatalf("expected noop policy, got %v", policy)
	}
}

func TestBuildPolicyFailOpenWrapper(t *testing.T) {
	RegisterPolicyBuilder("test/error", func(Config) (Policy, error) {
		return errorPolicy{}, nil
	})

	policy, _, err := BuildPolicy(Config{Kind: "test/error", FailOpen: true}, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	result, err := policy.Redact(Context{}, "keep-this")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result != "keep-this" {
		t.Fatalf("expected fail-open to return original text, got %q", result)
	}
}
