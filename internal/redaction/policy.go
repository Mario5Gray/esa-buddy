package redaction

import (
	"strings"
	"sync"
)

const (
	PolicyNone = "none"
)

// Policy redacts sensitive information from text.
type Policy interface {
	Name() string
	Redact(text string) (string, error)
}

type registry struct {
	mu       sync.RWMutex
	policies map[string]Policy
}

var globalRegistry = newRegistry()

func newRegistry() *registry {
	r := &registry{
		policies: map[string]Policy{},
	}
	r.register(NoopPolicy{})
	return r
}

func (r *registry) register(policy Policy) {
	if policy == nil {
		return
	}
	name := strings.TrimSpace(policy.Name())
	if name == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.policies[name] = policy
}

func (r *registry) policyByName(name string) Policy {
	name = strings.TrimSpace(name)
	if name == "" {
		name = PolicyNone
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if policy, ok := r.policies[name]; ok {
		return policy
	}
	return r.policies[PolicyNone]
}

// RegisterPolicy registers a policy globally for compaction redaction.
func RegisterPolicy(policy Policy) {
	globalRegistry.register(policy)
}

// PolicyByName returns a registered policy by name, falling back to "none".
func PolicyByName(name string) Policy {
	return globalRegistry.policyByName(name)
}

// NoopPolicy performs no redaction.
type NoopPolicy struct{}

func (NoopPolicy) Name() string { return PolicyNone }

func (NoopPolicy) Redact(text string) (string, error) { return text, nil }
