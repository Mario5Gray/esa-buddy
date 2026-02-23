package redaction

import (
	"errors"
	"strings"
	"sync"
	"time"
)

const (
	PolicyNone = "none"

	ResourceTypeCompactionInput   = "compaction_input"
	ResourceTypeCompactionSummary = "compaction_summary"
)

// Context carries resource metadata for resource-scoped redaction.
type Context struct {
	ResourcePath string
	ResourceType string
}

// Config controls redaction policy construction.
type Config struct {
	Kind       string
	ConfigFile string
	FailOpen   bool
	Options    map[string]any
	External   ExternalConfig
}

// ExternalConfig defines settings for external redaction adapters.
type ExternalConfig struct {
	URL     string
	Timeout time.Duration
}

// Policy redacts sensitive information from text.
type Policy interface {
	Name() string
	Redact(ctx Context, text string) (string, error)
}

// Builder constructs a Policy using a Config.
type Builder func(cfg Config) (Policy, error)

type registry struct {
	mu       sync.RWMutex
	policies map[string]Policy
	builders map[string]Builder
}

var globalRegistry = newRegistry()

func newRegistry() *registry {
	r := &registry{
		policies: map[string]Policy{},
		builders: map[string]Builder{},
	}
	r.register(NoopPolicy{})
	r.registerBuilder(PolicyNone, func(Config) (Policy, error) { return NoopPolicy{}, nil })
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

func (r *registry) registerBuilder(name string, builder Builder) {
	if builder == nil {
		return
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.builders[name] = builder
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

func (r *registry) builderByName(name string) Builder {
	name = strings.TrimSpace(name)
	if name == "" {
		name = PolicyNone
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.builders[name]
}

// RegisterPolicy registers a policy globally for compaction redaction.
func RegisterPolicy(policy Policy) {
	globalRegistry.register(policy)
}

// RegisterPolicyBuilder registers a policy builder globally for compaction redaction.
func RegisterPolicyBuilder(name string, builder Builder) {
	globalRegistry.registerBuilder(name, builder)
}

// PolicyByName returns a registered policy by name, falling back to "none".
func PolicyByName(name string) Policy {
	return globalRegistry.policyByName(name)
}

// BuildPolicy constructs a policy using config, falling back to legacyName when cfg.Kind is empty.
func BuildPolicy(cfg Config, legacyName string) (Policy, string, error) {
	kind := strings.TrimSpace(cfg.Kind)
	if kind == "" {
		kind = strings.TrimSpace(legacyName)
	}
	if kind == "" {
		kind = PolicyNone
	}

	builder := globalRegistry.builderByName(kind)
	if builder != nil {
		policy, err := builder(cfg)
		if err != nil {
			return nil, "", err
		}
		if cfg.FailOpen {
			return failOpenPolicy{inner: policy}, kind, nil
		}
		return policy, kind, nil
	}

	policy := globalRegistry.policyByName(kind)
	if policy == nil {
		return nil, "", errors.New("redaction policy not found")
	}
	if cfg.FailOpen {
		return failOpenPolicy{inner: policy}, kind, nil
	}
	return policy, kind, nil
}

type failOpenPolicy struct {
	inner Policy
}

func (p failOpenPolicy) Name() string {
	if p.inner == nil {
		return PolicyNone
	}
	return p.inner.Name()
}

func (p failOpenPolicy) Redact(ctx Context, text string) (string, error) {
	if p.inner == nil {
		return text, nil
	}
	redacted, err := p.inner.Redact(ctx, text)
	if err != nil {
		return text, nil
	}
	return redacted, nil
}

// NoopPolicy performs no redaction.
type NoopPolicy struct{}

func (NoopPolicy) Name() string { return PolicyNone }

func (NoopPolicy) Redact(_ Context, text string) (string, error) { return text, nil }
