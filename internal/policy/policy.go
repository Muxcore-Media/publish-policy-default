package policy

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Rule defines a single publish policy rule.
type Rule struct {
	// Caller is the module ID publishing the event. Supports "*" for any module.
	Caller string `yaml:"caller"`
	// EventTypes is the list of event type globs allowed. Supports "*" for all events.
	// Patterns may use glob-style matching: "download.*", "media.*", "module.*".
	EventTypes []string `yaml:"event_types"`
}

// Policy holds the complete set of publish policy rules.
type Policy struct {
	mu    sync.RWMutex
	rules []Rule
}

// Load parses a YAML policy file and returns a Policy.
func Load(path string) (*Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read policy file: %w", err)
	}
	return Parse(data)
}

// Parse parses YAML policy data and returns a Policy.
func Parse(data []byte) (*Policy, error) {
	var rules []Rule
	if err := yaml.Unmarshal(data, &rules); err != nil {
		return nil, fmt.Errorf("parse policy: %w", err)
	}
	for i, r := range rules {
		if r.Caller == "" {
			return nil, fmt.Errorf("rule %d: caller is required", i)
		}
		if len(r.EventTypes) == 0 {
			return nil, fmt.Errorf("rule %d: at least one event_type is required", i)
		}
	}
	return &Policy{rules: rules}, nil
}

// Allow checks whether a caller may publish the given event type.
// Rules are evaluated in order; the first matching rule decides.
func (p *Policy) Allow(caller, eventType string) (bool, string) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, r := range p.rules {
		if !matchWildcard(r.Caller, caller) {
			continue
		}
		if !matchEventTypes(r.EventTypes, eventType) {
			continue
		}
		return true, ""
	}
	return false, fmt.Sprintf("no policy rule matches caller=%q event_type=%q", caller, eventType)
}

// ReplaceRules atomically replaces all rules with those from another Policy.
func (p *Policy) ReplaceRules(src *Policy) {
	src.mu.RLock()
	rules := make([]Rule, len(src.rules))
	copy(rules, src.rules)
	src.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()
	p.rules = rules
}

func matchWildcard(pattern, value string) bool {
	return pattern == "*" || pattern == value
}

func matchEventTypes(patterns []string, eventType string) bool {
	for _, pattern := range patterns {
		if matchEventType(pattern, eventType) {
			return true
		}
	}
	return false
}

func matchEventType(pattern, eventType string) bool {
	if pattern == "*" {
		return true
	}
	if pattern == eventType {
		return true
	}
	// Glob: "download.*" matches "download.completed", "download.started", etc.
	if strings.HasSuffix(pattern, ".*") {
		prefix := strings.TrimSuffix(pattern, ".*")
		if prefix == "" {
			return true // ".*" matches everything
		}
		return strings.HasPrefix(eventType, prefix+".") || eventType == prefix
	}
	return false
}
