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
	if m.policy != nil {
		registryOn = m.policy.RegistryMatchingEnabled()
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
		if on && m.mc == nil {
			go m.subscribeRegistryEvents()
		}
		return nil
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
