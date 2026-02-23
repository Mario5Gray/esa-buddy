package redaction

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/bmatcuk/doublestar/v4"
)

const (
	KindMarkerRegex   = "builtin/marker-regex"
	DefaultRedactText = "[REDACTED]"
)

type markerRegexConfig struct {
	Paths pathScope    `toml:"paths"`
	Rules []ruleConfig `toml:"rules"`
}

type pathScope struct {
	Include []string `toml:"include"`
	Exclude []string `toml:"exclude"`
}

type ruleConfig struct {
	Name          string   `toml:"name"`
	Type          string   `toml:"type"`
	Open          string   `toml:"open"`
	Close         string   `toml:"close"`
	Pattern       string   `toml:"pattern"`
	Replacement   string   `toml:"replacement"`
	Scope         []string `toml:"scope"`
	ResourceTypes []string `toml:"resource_types"`
}

type compiledRule struct {
	name          string
	kind          string
	open          *regexp.Regexp
	close         *regexp.Regexp
	pattern       *regexp.Regexp
	replacement   string
	scope         []string
	resourceTypes []string
}

func init() {
	RegisterPolicyBuilder(KindMarkerRegex, newMarkerRegexPolicy)
}

func newMarkerRegexPolicy(cfg Config) (Policy, error) {
	path := strings.TrimSpace(cfg.ConfigFile)
	if path == "" {
		path = "esa.redaction.toml"
	}
	if !filepath.IsAbs(path) {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		path = filepath.Join(cwd, path)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var parsed markerRegexConfig
	if _, err := toml.Decode(string(content), &parsed); err != nil {
		return nil, err
	}

	compiled, err := compileRules(parsed.Rules)
	if err != nil {
		return nil, err
	}
	return &compiledMarkerRegexPolicy{
		name:  KindMarkerRegex,
		paths: parsed.Paths,
		rules: compiled,
	}, nil
}

type compiledMarkerRegexPolicy struct {
	name  string
	paths pathScope
	rules []compiledRule
}

func (p *compiledMarkerRegexPolicy) Name() string { return p.name }

func (p *compiledMarkerRegexPolicy) Redact(ctx Context, text string) (string, error) {
	if !p.paths.matches(ctx.ResourcePath) {
		return text, nil
	}
	redacted := text
	for _, rule := range p.rules {
		if !rule.matches(ctx) {
			continue
		}
		var err error
		switch rule.kind {
		case "marker":
			redacted, err = redactMarkers(redacted, rule)
		case "regex":
			redacted = rule.pattern.ReplaceAllString(redacted, rule.replacement)
		default:
			continue
		}
		if err != nil {
			return text, err
		}
	}
	return redacted, nil
}

func compileRules(rules []ruleConfig) ([]compiledRule, error) {
	compiled := make([]compiledRule, 0, len(rules))
	for _, rule := range rules {
		kind := strings.TrimSpace(rule.Type)
		if kind == "" {
			return nil, errors.New("redaction rule type is required")
		}
		name := strings.TrimSpace(rule.Name)
		if name == "" {
			name = kind
		}
		replacement := strings.TrimSpace(rule.Replacement)
		if replacement == "" {
			replacement = DefaultRedactText
		}

		switch kind {
		case "marker":
			openPattern := strings.TrimSpace(rule.Open)
			closePattern := strings.TrimSpace(rule.Close)
			if openPattern == "" || closePattern == "" {
				return nil, fmt.Errorf("marker rule %q requires open and close patterns", name)
			}
			openRegex, err := regexp.Compile(openPattern)
			if err != nil {
				return nil, err
			}
			closeRegex, err := regexp.Compile(closePattern)
			if err != nil {
				return nil, err
			}
			compiled = append(compiled, compiledRule{
				name:          name,
				kind:          kind,
				open:          openRegex,
				close:         closeRegex,
				replacement:   replacement,
				scope:         rule.Scope,
				resourceTypes: rule.ResourceTypes,
			})
		case "regex":
			pattern := strings.TrimSpace(rule.Pattern)
			if pattern == "" {
				return nil, fmt.Errorf("regex rule %q requires a pattern", name)
			}
			re, err := regexp.Compile(pattern)
			if err != nil {
				return nil, err
			}
			compiled = append(compiled, compiledRule{
				name:          name,
				kind:          kind,
				pattern:       re,
				replacement:   replacement,
				scope:         rule.Scope,
				resourceTypes: rule.ResourceTypes,
			})
		default:
			return nil, fmt.Errorf("unsupported redaction rule type: %s", kind)
		}
	}
	return compiled, nil
}

func (p pathScope) matches(resourcePath string) bool {
	resourcePath = filepath.ToSlash(strings.TrimSpace(resourcePath))
	if resourcePath == "" {
		if len(p.Include) == 0 && len(p.Exclude) == 0 {
			return true
		}
		return false
	}
	if len(p.Include) > 0 && !matchAny(p.Include, resourcePath) {
		return false
	}
	if len(p.Exclude) > 0 && matchAny(p.Exclude, resourcePath) {
		return false
	}
	return true
}

func matchAny(patterns []string, resourcePath string) bool {
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		ok, err := doublestar.Match(pattern, resourcePath)
		if err != nil {
			continue
		}
		if ok {
			return true
		}
	}
	return false
}

func (r compiledRule) matches(ctx Context) bool {
	if len(r.resourceTypes) > 0 {
		ok := false
		for _, t := range r.resourceTypes {
			if strings.EqualFold(strings.TrimSpace(t), ctx.ResourceType) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if len(r.scope) == 0 {
		return true
	}
	return matchAny(r.scope, filepath.ToSlash(ctx.ResourcePath))
}

func redactMarkers(text string, rule compiledRule) (string, error) {
	if rule.open.MatchString("") || rule.close.MatchString("") {
		return text, errors.New("marker rule patterns must not match empty string")
	}
	output := text
	for {
		loc := rule.open.FindStringIndex(output)
		if loc == nil {
			break
		}
		searchStart := loc[1]
		closeLoc := rule.close.FindStringIndex(output[searchStart:])
		if closeLoc == nil {
			return text, fmt.Errorf("marker rule %q close pattern not found", rule.name)
		}
		end := searchStart + closeLoc[1]
		output = output[:loc[0]] + rule.replacement + output[end:]
	}
	return output, nil
}
