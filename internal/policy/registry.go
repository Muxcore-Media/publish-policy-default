package policy

import (
	"strings"
)

// infraCapabilities only grant module lifecycle event prefixes.
var infraCapabilities = map[string]bool{
	"settings":       true,
	"health":         true,
	"publish.policy": true,
	"call.policy":    true,
	"audit":          true,
}

func capabilityEventPrefix(cap string) string {
	cap = strings.TrimSpace(cap)
	if cap == "" {
		return ""
	}
	if i := strings.Index(cap, "."); i > 0 {
		return cap[:i]
	}
	return cap
}

func eventPatternsForRegistryCapability(cap string) []string {
	cap = strings.TrimSpace(cap)
	if cap == "" {
		return nil
	}
	if infraCapabilities[cap] {
		return []string{"module.*"}
	}
	prefix := capabilityEventPrefix(cap)
	out := []string{
		prefix + ".*",
		"capability." + cap,
	}
	if strings.Contains(cap, ".") {
		out = append(out, "capability."+cap+".*")
	} else {
		out = append(out, "capability."+cap+".*")
	}
	return out
}

func registryAllowsCapabilities(caps []string, eventType string) bool {
	for _, cap := range caps {
		for _, pattern := range eventPatternsForRegistryCapability(cap) {
			if matchEventType(pattern, eventType) {
				return true
			}
		}
	}
	return false
}

func (p *Policy) SetRegistryMatching(enabled bool) {
	p.mu.Lock()
	p.registryEnabled = enabled
	p.mu.Unlock()
}

func (p *Policy) RegistryMatchingEnabled() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.registryEnabled
}

func (p *Policy) UpsertRegistryModule(moduleID string, capabilities []string) {
	moduleID = strings.TrimSpace(moduleID)
	if moduleID == "" {
		return
	}
	caps := make([]string, 0, len(capabilities))
	for _, c := range capabilities {
		c = strings.TrimSpace(c)
		if c != "" {
			caps = append(caps, c)
		}
	}
	p.registryMu.Lock()
	defer p.registryMu.Unlock()
	if p.registryCaps == nil {
		p.registryCaps = make(map[string][]string)
	}
	p.registryCaps[moduleID] = caps
}

func (p *Policy) RemoveRegistryModule(moduleID string) {
	moduleID = strings.TrimSpace(moduleID)
	if moduleID == "" {
		return
	}
	p.registryMu.Lock()
	defer p.registryMu.Unlock()
	delete(p.registryCaps, moduleID)
}

func (p *Policy) RegistryModuleCount() int {
	p.registryMu.RLock()
	defer p.registryMu.RUnlock()
	return len(p.registryCaps)
}

func (p *Policy) allowViaRegistry(caller, eventType string, payload []byte) bool {
	p.mu.RLock()
	enabled := p.registryEnabled
	p.mu.RUnlock()
	if !enabled {
		return false
	}
	p.registryMu.RLock()
	caps := p.registryCaps[caller]
	p.registryMu.RUnlock()
	if len(caps) == 0 {
		return false
	}
	return registryAllowsCapabilities(caps, eventType)
}
