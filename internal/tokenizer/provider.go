package tokenizer

// CounterProvider returns a Counter based on provider name.
type CounterProvider interface {
	CounterFor(provider string) Counter
}

// MapProvider selects counters by provider name with a default fallback.
type MapProvider struct {
	Default Counter
	byKey   map[string]Counter
}

func NewMapProvider(defaultCounter Counter) *MapProvider {
	return &MapProvider{
		Default: defaultCounter,
		byKey:   make(map[string]Counter),
	}
}

func (m *MapProvider) Set(provider string, counter Counter) {
	if provider == "" || counter == nil {
		return
	}
	m.byKey[provider] = counter
}

func (m *MapProvider) CounterFor(provider string) Counter {
	if m == nil {
		return nil
	}
	if provider != "" {
		if c, ok := m.byKey[provider]; ok {
			return c
		}
	}
	return m.Default
}
