package internal

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/Muxcore-Media/core/pkg/contracts"
	"github.com/Muxcore-Media/publish-policy-default/internal/policy"
)

func (m *Module) Settings() []contracts.SettingDef {
	return m.settingsDefs()
}

func (m *Module) UpdateSetting(key, value string) error {
	return m.updateSetting(key, value)
}

func (m *Module) settingsDefs() []contracts.SettingDef {
	m.cfgMu.RLock()
	defer m.cfgMu.RUnlock()
	registryOn := false
	var allowed, denied int64
	if m.policy != nil {
		registryOn = m.policy.RegistryMatchingEnabled()
	}
	if m.srv != nil {
		allowed = m.srv.AllowedTotal()
		denied = m.srv.DeniedTotal()
	}
	return []contracts.SettingDef{
		{
			Key:         "policy_file",
			Label:       "Publish Policy File",
			Type:        contracts.SettingTypeString,
			Value:       m.filePath,
			Default:     "policies.yaml",
			Description: "Path to YAML publish policy (PUBLISH_POLICY_FILE); updates reload immediately",
			Group:       "Policy",
		},
		{
			Key:         "reload_policy",
			Label:       "Reload Policy",
			Type:        contracts.SettingTypeString,
			Value:       "",
			Description: "Set to reload (any non-empty value) to re-read the current policy_file without changing its path",
			Group:       "Policy",
		},
		{
			Key:         "audit_path",
			Label:       "Audit Export Path",
			Type:        contracts.SettingTypeString,
			Value:       m.auditPath,
			Description: "JSONL audit log path (PUBLISH_POLICY_AUDIT_PATH); empty disables export",
			Group:       "Audit",
		},
		{
			Key:         "registry_capability_matching",
			Label:       "Registry Capability Matching",
			Type:        contracts.SettingTypeBool,
			Value:       fmt.Sprintf("%t", registryOn),
			Description: "When enabled, modules may publish event types derived from their registered mesh capabilities (PUBLISH_POLICY_REGISTRY_MATCH)",
			Group:       "Policy",
		},
		{
			Key:         "allowed_total",
			Label:       "Allowed Publish Total",
			Type:        contracts.SettingTypeInt,
			Value:       fmt.Sprintf("%d", allowed),
			Description: "Read-only counter of event publishes allowed by policy (publish_policy_allowed_total)",
			Group:       "Metrics",
		},
		{
			Key:         "denied_total",
			Label:       "Denied Publish Total",
			Type:        contracts.SettingTypeInt,
			Value:       fmt.Sprintf("%d", denied),
			Description: "Read-only counter of event publishes denied by policy (publish_policy_denied_total)",
			Group:       "Metrics",
		},
	}
}

func (m *Module) updateSetting(key, value string) error {
	value = strings.TrimSpace(value)
	switch key {
	case "policy_file", "PUBLISH_POLICY_FILE":
		if value == "" {
			return fmt.Errorf("policy_file must not be empty")
		}
		m.cfgMu.Lock()
		m.filePath = value
		m.cfgMu.Unlock()
		if m.policy == nil {
			return nil
		}
		return m.ReloadPolicy()
	case "reload_policy":
		if value == "" {
			return fmt.Errorf("reload_policy requires a non-empty trigger value")
		}
		if m.policy == nil {
			return fmt.Errorf("policy not loaded")
		}
		return m.ReloadPolicy()
	case "audit_path", "PUBLISH_POLICY_AUDIT_PATH":
		m.cfgMu.Lock()
		m.auditPath = value
		m.cfgMu.Unlock()
		if m.policy != nil {
			m.policy.SetAuditPath(value)
		}
		return nil
	case "registry_capability_matching", "PUBLISH_POLICY_REGISTRY_MATCH":
		on := strings.EqualFold(value, "true") || value == "1"
		if m.policy != nil {
			m.policy.SetRegistryMatching(on)
		}
		if on {
			go m.subscribeRegistryEvents()
		}
		return nil
	case "allowed_total", "denied_total":
		return fmt.Errorf("setting %q is read-only", key)
	default:
		return fmt.Errorf("unknown setting %q", key)
	}
}

// ReloadPolicy reloads the YAML policy file and replaces static rules.
func (m *Module) ReloadPolicy() error {
	m.cfgMu.RLock()
	path := m.filePath
	audit := m.auditPath
	m.cfgMu.RUnlock()
	newP, err := policy.Load(path)
	if err != nil {
		return err
	}
	m.policy.ReplaceRules(newP)
	if audit != "" {
		m.policy.SetAuditPath(audit)
	}
	slog.Info("publish-policy reloaded", "file", path)
	return nil
}
