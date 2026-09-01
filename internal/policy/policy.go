package policy

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Rule defines a single publish policy rule.
type Rule struct {
	Caller string `yaml:"caller"`
	// CallerGroup references a named group from the policy document.
	CallerGroup string `yaml:"caller_group"`
	// EventTypes is the list of event type globs allowed. Supports "*" and "prefix.*".
	EventTypes []string `yaml:"event_types"`
	// RequiredCapability, when set, requires the event type to match capability.<name>
	// or the payload JSON field "capability" to equal this value (dynamic capability matching).
	RequiredCapability string `yaml:"required_capability"`
	// PayloadMaxBytes rejects oversized payloads (0 = unlimited).
	PayloadMaxBytes int `yaml:"payload_max_bytes"`
	// PayloadRequireKeys requires these top-level JSON object keys when payload is JSON.
	PayloadRequireKeys []string `yaml:"payload_require_keys"`
	// RateLimitPerMin caps matching publishes per caller+event_type per rolling minute.
	RateLimitPerMin int `yaml:"rate_limit_per_min"`
}

type policyDoc struct {
	RegistryCapabilityMatching *bool               `yaml:"registry_capability_matching"`
	Groups                     map[string][]string `yaml:"groups"`
	Rules                      []Rule              `yaml:"rules"`
}

// Policy holds the complete set of publish policy rules.
type Policy struct {
	mu     sync.RWMutex
	rules  []Rule
	groups map[string][]string

	registryMu      sync.RWMutex
	registryEnabled bool
	registryCaps    map[string][]string

	rateMu sync.Mutex
	rates  map[string]*rateWindow

	auditMu   sync.Mutex
	auditPath string

	now func() time.Time
}

type rateWindow struct {
	start time.Time
	count int
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
	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return nil, fmt.Errorf("parse policy: %w", err)
	}
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		node = *node.Content[0]
	}

	var rules []Rule
	var groups map[string][]string
	var registryMatching *bool

	switch node.Kind {
	case yaml.SequenceNode, 0:
		if err := yaml.Unmarshal(data, &rules); err != nil {
			return nil, fmt.Errorf("parse policy: %w", err)
		}
	case yaml.MappingNode:
		var doc policyDoc
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return nil, fmt.Errorf("parse policy: %w", err)
		}
		rules = doc.Rules
		groups = doc.Groups
		registryMatching = doc.RegistryCapabilityMatching
	default:
		rules = nil
	}

	for i, r := range rules {
		if r.Caller == "" && r.CallerGroup == "" {
			return nil, fmt.Errorf("rule %d: caller or caller_group is required", i)
		}
		if r.Caller != "" && r.CallerGroup != "" {
			return nil, fmt.Errorf("rule %d: set caller or caller_group, not both", i)
		}
		if r.CallerGroup != "" {
			if _, ok := groups[r.CallerGroup]; !ok {
				return nil, fmt.Errorf("rule %d: unknown caller_group %q", i, r.CallerGroup)
			}
		}
		if len(r.EventTypes) == 0 {
			return nil, fmt.Errorf("rule %d: at least one event_type is required", i)
		}
		if r.RateLimitPerMin < 0 {
			return nil, fmt.Errorf("rule %d: rate_limit_per_min must be >= 0", i)
		}
		if r.PayloadMaxBytes < 0 {
			return nil, fmt.Errorf("rule %d: payload_max_bytes must be >= 0", i)
		}
	}
	p := &Policy{
		rules:        rules,
		groups:       groups,
		registryCaps: make(map[string][]string),
		rates:        make(map[string]*rateWindow),
		now:          time.Now,
	}
	if registryMatching != nil {
		p.registryEnabled = *registryMatching
	}
	return p, nil
}

// SetAuditPath enables JSONL audit export of allow/deny decisions.
func (p *Policy) SetAuditPath(path string) {
	p.auditMu.Lock()
	defer p.auditMu.Unlock()
	p.auditPath = path
}

// Allow checks whether a caller may publish the given event type (no payload).
func (p *Policy) Allow(caller, eventType string) (bool, string) {
	return p.AllowWithPayload(caller, eventType, nil)
}

// AllowWithPayload checks publish permission with optional event payload bytes.
func (p *Policy) AllowWithPayload(caller, eventType string, payload []byte) (bool, string) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	now := time.Now()
	if p.now != nil {
		now = p.now()
	}

	for i, r := range p.rules {
		if !p.callerMatches(r, caller) {
			continue
		}
		if !matchEventTypes(r.EventTypes, eventType) {
			continue
		}
		if r.RequiredCapability != "" && !capabilityMatches(r.RequiredCapability, eventType, payload) {
			continue
		}
		if r.PayloadMaxBytes > 0 && len(payload) > r.PayloadMaxBytes {
			reason := fmt.Sprintf("payload exceeds max bytes (%d > %d)", len(payload), r.PayloadMaxBytes)
			p.audit(caller, eventType, false, reason)
			return false, reason
		}
		if len(r.PayloadRequireKeys) > 0 {
			if err := requireJSONKeys(payload, r.PayloadRequireKeys); err != nil {
				reason := err.Error()
				p.audit(caller, eventType, false, reason)
				return false, reason
			}
		}
		if r.RateLimitPerMin > 0 {
			key := fmt.Sprintf("%d|%s|%s", i, caller, eventType)
			if !p.allowRate(key, r.RateLimitPerMin, now) {
				reason := fmt.Sprintf("rate limit exceeded (%d/min) for caller=%q event_type=%q",
					r.RateLimitPerMin, caller, eventType)
				p.audit(caller, eventType, false, reason)
				return false, reason
			}
		}
		p.audit(caller, eventType, true, "")
		return true, ""
	}
	if p.allowViaRegistry(caller, eventType, payload) {
		p.audit(caller, eventType, true, "registry capability match")
		return true, ""
	}
	reason := fmt.Sprintf("no policy rule matches caller=%q event_type=%q", caller, eventType)
	p.audit(caller, eventType, false, reason)
	return false, reason
}

func (p *Policy) callerMatches(r Rule, caller string) bool {
	if r.CallerGroup != "" {
		for _, m := range p.groups[r.CallerGroup] {
			if matchWildcard(m, caller) {
				return true
			}
		}
		return false
	}
	return matchWildcard(r.Caller, caller)
}

func (p *Policy) allowRate(key string, limit int, now time.Time) bool {
	p.rateMu.Lock()
	defer p.rateMu.Unlock()
	w, ok := p.rates[key]
	if !ok || now.Sub(w.start) >= time.Minute {
		p.rates[key] = &rateWindow{start: now, count: 1}
		return true
	}
	if w.count >= limit {
		return false
	}
	w.count++
	return true
}

func (p *Policy) audit(caller, eventType string, allowed bool, reason string) {
	p.auditMu.Lock()
	path := p.auditPath
	p.auditMu.Unlock()
	if path == "" {
		return
	}
	rec := map[string]any{
		"ts":         time.Now().UTC().Format(time.RFC3339Nano),
		"caller":     caller,
		"event_type": eventType,
		"allowed":    allowed,
	}
	if reason != "" {
		rec["reason"] = reason
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		slog.Warn("publish-policy audit write failed", "path", path, "error", err)
		return
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Write(append(b, '\n')); err != nil {
		slog.Warn("publish-policy audit write failed", "path", path, "error", err)
	}
}

// ReplaceRules atomically replaces static YAML rules. Registry capability state is preserved.
func (p *Policy) ReplaceRules(src *Policy) {
	src.mu.RLock()
	rules := make([]Rule, len(src.rules))
	copy(rules, src.rules)
	groups := cloneGroups(src.groups)
	registryEnabled := src.registryEnabled
	src.mu.RUnlock()

	p.mu.Lock()
	p.rules = rules
	p.groups = groups
	p.registryEnabled = registryEnabled
	p.mu.Unlock()

	p.rateMu.Lock()
	p.rates = make(map[string]*rateWindow)
	p.rateMu.Unlock()
}

func cloneGroups(in map[string][]string) map[string][]string {
	if in == nil {
		return nil
	}
	out := make(map[string][]string, len(in))
	for k, v := range in {
		out[k] = append([]string(nil), v...)
	}
	return out
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
	if strings.HasSuffix(pattern, ".*") {
		prefix := strings.TrimSuffix(pattern, ".*")
		if prefix == "" {
			return true
		}
		return strings.HasPrefix(eventType, prefix+".") || eventType == prefix
	}
	return false
}

func capabilityMatches(required, eventType string, payload []byte) bool {
	if eventType == "capability."+required || strings.HasPrefix(eventType, "capability."+required+".") {
		return true
	}
	if len(payload) == 0 {
		return false
	}
	var obj map[string]any
	if err := json.Unmarshal(payload, &obj); err != nil {
		return false
	}
	if v, ok := obj["capability"].(string); ok && v == required {
		return true
	}
	return false
}

func requireJSONKeys(payload []byte, keys []string) error {
	if len(payload) == 0 {
		return fmt.Errorf("payload required for key checks")
	}
	var obj map[string]any
	if err := json.Unmarshal(payload, &obj); err != nil {
		return fmt.Errorf("payload is not JSON object: %w", err)
	}
	for _, k := range keys {
		if _, ok := obj[k]; !ok {
			return fmt.Errorf("payload missing required key %q", k)
		}
	}
	return nil
}
