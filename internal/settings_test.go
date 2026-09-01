package internal

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	policyv1 "github.com/Muxcore-Media/core/proto/gen/muxcore/policy/v1"
)

func TestSettingsPolicyAndAudit(t *testing.T) {
	dir := t.TempDir()
	p1 := filepath.Join(dir, "a.yaml")
	p2 := filepath.Join(dir, "b.yaml")
	audit := filepath.Join(dir, "audit.jsonl")
	body := "rules:\n  - caller: \"*\"\n    event_types: [\"*\"]\n"
	if err := os.WriteFile(p1, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p2, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	m := NewModule(Config{FilePath: p1, GRPCAddr: "127.0.0.1:0"})
	if err := m.Init(t.Context()); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = m.Stop(t.Context()) }()

	if err := m.UpdateSetting("policy_file", p2); err != nil {
		t.Fatal(err)
	}
	if got := m.Settings()[0].Value; got != p2 {
		t.Fatalf("path=%q", got)
	}
	if err := m.UpdateSetting("audit_path", audit); err != nil {
		t.Fatal(err)
	}
	if got := m.Settings()[2].Value; got != audit {
		t.Fatalf("audit=%q", got)
	}
	if err := m.UpdateSetting("policy_file", ""); err == nil {
		t.Fatal("expected error")
	}
}

func TestSettingsRegistryMatchingToggle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policies.yaml")
	body := "registry_capability_matching: false\nrules: []\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	m := NewModule(Config{FilePath: path, GRPCAddr: "127.0.0.1:0"})
	if err := m.Init(t.Context()); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = m.Stop(t.Context()) }()

	if err := m.UpdateSetting("registry_capability_matching", "true"); err != nil {
		t.Fatal(err)
	}
	if !m.policy.RegistryMatchingEnabled() {
		t.Fatal("expected registry matching enabled")
	}
	if err := m.UpdateSetting("registry_capability_matching", "false"); err != nil {
		t.Fatal(err)
	}
	if m.policy.RegistryMatchingEnabled() {
		t.Fatal("expected registry matching disabled")
	}
}

func TestSettingsReloadPolicy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policies.yaml")
	deny := "rules:\n  - caller: \"a\"\n    event_types: [\"deny.*\"]\n"
	allow := "rules:\n  - caller: \"a\"\n    event_types: [\"allow.*\"]\n"
	if err := os.WriteFile(path, []byte(deny), 0o600); err != nil {
		t.Fatal(err)
	}
	m := NewModule(Config{FilePath: path, GRPCAddr: "127.0.0.1:0"})
	if err := m.Init(t.Context()); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = m.Stop(t.Context()) }()

	if ok, _ := m.policy.Allow("a", "allow.x"); ok {
		t.Fatal("expected deny before reload")
	}
	if err := os.WriteFile(path, []byte(allow), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := m.UpdateSetting("reload_policy", "reload"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := m.policy.Allow("a", "allow.x"); !ok {
		t.Fatal("expected allow after reload")
	}
}

func TestSettingsPublishMetrics(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policies.yaml")
	body := "rules:\n  - caller: \"a\"\n    event_types: [\"ok.*\"]\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	m := NewModule(Config{FilePath: path, GRPCAddr: "127.0.0.1:0"})
	ctx := context.Background()
	if err := m.Init(ctx); err != nil {
		t.Fatal(err)
	}
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = m.Stop(ctx) }()

	if _, err := m.srv.AllowPublish(ctx, &policyv1.AllowPublishRequest{
		CallerModuleId: "a",
		EventType:      "ok.one",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.srv.AllowPublish(ctx, &policyv1.AllowPublishRequest{
		CallerModuleId: "a",
		EventType:      "nope",
	}); err != nil {
		t.Fatal(err)
	}

	settings := m.Settings()
	var allowedStr, deniedStr string
	for _, s := range settings {
		switch s.Key {
		case "allowed_total":
			allowedStr = s.Value
		case "denied_total":
			deniedStr = s.Value
		}
	}
	allowed, _ := strconv.ParseInt(allowedStr, 10, 64)
	denied, _ := strconv.ParseInt(deniedStr, 10, 64)
	if allowed != 1 {
		t.Fatalf("allowed_total=%d", allowed)
	}
	if denied != 1 {
		t.Fatalf("denied_total=%d", denied)
	}
	if err := m.UpdateSetting("allowed_total", "0"); err == nil {
		t.Fatal("expected read-only error")
	}
}
