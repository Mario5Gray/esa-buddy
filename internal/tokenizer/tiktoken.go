package tokenizer

import (
	"sync"

	"github.com/pkoukk/tiktoken-go"
)

// TiktokenCounter estimates tokens using tiktoken encodings.
type TiktokenCounter struct {
	mu  sync.Mutex
	enc map[string]*tiktoken.Tiktoken
}

func NewTiktokenCounter() *TiktokenCounter {
	return &TiktokenCounter{enc: make(map[string]*tiktoken.Tiktoken)}
}

func (t *TiktokenCounter) Name() string {
	return "tiktoken"
}

func (t *TiktokenCounter) Estimate(text string, model string) int {
	if text == "" {
		return 0
	}
	enc, err := t.getEncoding(model)
	if err != nil || enc == nil {
		return 0
	}
	tokens := enc.Encode(text, nil, nil)
	return len(tokens)
}

func (t *TiktokenCounter) EstimateChars(chars int, model string) int {
	// tiktoken needs real text to be accurate; return 0 to signal fallback.
	return 0
}

func (t *TiktokenCounter) getEncoding(model string) (*tiktoken.Tiktoken, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if enc, ok := t.enc[model]; ok {
		return enc, nil
	}

	enc, err := tiktoken.EncodingForModel(model)
	if err != nil {
		// Fallback to a common encoding for newer models.
		enc, err = tiktoken.GetEncoding("cl100k_base")
		if err != nil {
			return nil, err
		}
	}

	t.enc[model] = enc
	return enc, nil
}
