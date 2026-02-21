package tokenizer

// Counter estimates tokens for a given text and model.
type Counter interface {
	Estimate(text string, model string) int
	EstimateChars(chars int, model string) int
	Name() string
}

// FallbackCounter uses a simple chars-per-token heuristic.
type FallbackCounter struct {
	CharsPerToken int
}

func (f FallbackCounter) Name() string {
	return "fallback"
}

func (f FallbackCounter) Estimate(text string, model string) int {
	chars := len(text)
	if chars <= 0 {
		return 0
	}
	cpt := f.CharsPerToken
	if cpt <= 0 {
		cpt = 4
	}
	return (chars + cpt - 1) / cpt
}

func (f FallbackCounter) EstimateChars(chars int, model string) int {
	if chars <= 0 {
		return 0
	}
	cpt := f.CharsPerToken
	if cpt <= 0 {
		cpt = 4
	}
	return (chars + cpt - 1) / cpt
}
