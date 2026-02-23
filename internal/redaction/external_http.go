package redaction

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

const KindExternalHTTP = "external/http"

type externalHTTPPolicy struct {
	name    string
	url     string
	timeout time.Duration
	client  *http.Client
}

type externalRequest struct {
	Text         string `json:"text"`
	ResourcePath string `json:"resource_path,omitempty"`
	ResourceType string `json:"resource_type,omitempty"`
}

type externalResponse struct {
	RedactedText string `json:"redacted_text"`
	Text         string `json:"text"`
}

func init() {
	RegisterPolicyBuilder(KindExternalHTTP, newExternalHTTPPolicy)
}

func newExternalHTTPPolicy(cfg Config) (Policy, error) {
	url := strings.TrimSpace(cfg.External.URL)
	if url == "" {
		return nil, errors.New("external redaction url is required")
	}
	timeout := cfg.External.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return &externalHTTPPolicy{
		name:    KindExternalHTTP,
		url:     url,
		timeout: timeout,
		client: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

func (p *externalHTTPPolicy) Name() string { return p.name }

func (p *externalHTTPPolicy) Redact(ctx Context, text string) (string, error) {
	reqBody := externalRequest{
		Text:         text,
		ResourcePath: ctx.ResourcePath,
		ResourceType: ctx.ResourceType,
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return text, err
	}
	req, err := http.NewRequest(http.MethodPost, p.url, bytes.NewReader(payload))
	if err != nil {
		return text, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return text, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return text, errors.New("external redaction service returned non-2xx")
	}
	var out externalResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return text, err
	}
	if out.RedactedText != "" {
		return out.RedactedText, nil
	}
	if out.Text != "" {
		return out.Text, nil
	}
	return text, nil
}
