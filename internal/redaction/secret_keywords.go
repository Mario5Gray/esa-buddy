package redaction

import (
	"regexp"
	"strings"
)

const KindSecretKeywords = "builtin/secret-keywords"

var secretKeywordPatterns = []string{
	`(?i)(\"?(?:api[_-]?key|access[_-]?token|refresh[_-]?token|token|secret|password|passwd|authorization|bearer)\"?\s*:\s*)(\"[^\"]*\"|'[^']*'|[^,\s}]+)`,
	`(?i)((?:api[_-]?key|access[_-]?token|refresh[_-]?token|token|secret|password|passwd|authorization|bearer)\s*[:=]\s*)([^\r\n]+)`,
}

type secretKeywordsPolicy struct {
	name     string
	patterns []*regexp.Regexp
}

func init() {
	RegisterPolicyBuilder(KindSecretKeywords, newSecretKeywordsPolicy)
}

func newSecretKeywordsPolicy(_ Config) (Policy, error) {
	patterns := make([]*regexp.Regexp, 0, len(secretKeywordPatterns))
	for _, pattern := range secretKeywordPatterns {
		patterns = append(patterns, regexp.MustCompile(pattern))
	}
	return &secretKeywordsPolicy{
		name:     KindSecretKeywords,
		patterns: patterns,
	}, nil
}

func (p *secretKeywordsPolicy) Name() string {
	return p.name
}

func (p *secretKeywordsPolicy) Redact(_ Context, text string) (string, error) {
	redacted := text
	for _, pattern := range p.patterns {
		redacted = pattern.ReplaceAllStringFunc(redacted, func(match string) string {
			loc := pattern.FindStringSubmatchIndex(match)
			if len(loc) < 6 {
				return "[REDACTED]"
			}
			prefix := match[loc[2]:loc[3]]
			value := match[loc[4]:loc[5]]
			replacement := DefaultRedactText
			if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
				replacement = "\"" + DefaultRedactText + "\""
			} else if strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") {
				replacement = "'" + DefaultRedactText + "'"
			}
			return prefix + replacement
		})
	}
	return redacted, nil
}
