package policy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParse_EmptyRules(t *testing.T) {
	p, err := Parse([]byte{})
	if err != nil {
		t.Fatalf("Parse empty: %v", err)
	}
	allowed, _ := p.Allow("a", "test.event")
	if allowed {
		t.Error("expected empty policy to deny all")
	}
}

func TestParse_InvalidYAML(t *testing.T) {
	_, err := Parse([]byte(`{{{invalid}}}`))
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestParse_MissingCaller(t *testing.T) {
	_, err := Parse([]byte(`
- event_types: ["test.*"]
`))
	if err == nil {
		t.Fatal("expected error for missing caller")
	}
}

func TestParse_MissingEventTypes(t *testing.T) {
	_, err := Parse([]byte(`
- caller: "mod-a"
`))
	if err == nil {
		t.Fatal("expected error for missing event_types")
	}
}

func TestAllow_ExactMatch(t *testing.T) {
	p, err := Parse([]byte(`
- caller: "mod-a"
  event_types: ["test.completed"]
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if allowed, _ := p.Allow("mod-a", "test.completed"); !allowed {
		t.Error("expected exact match to be allowed")
	}
	if allowed, _ := p.Allow("mod-a", "test.started"); allowed {
		t.Error("expected non-matching event to be denied")
	}
	if allowed, _ := p.Allow("mod-b", "test.completed"); allowed {
		t.Error("expected non-matching caller to be denied")
	}
}

func TestAllow_GlobMatch(t *testing.T) {
	p, err := Parse([]byte(`
- caller: "downloader"
  event_types: ["download.*"]
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	for _, et := range []string{"download.completed", "download.started", "download.progress", "download"} {
		if allowed, _ := p.Allow("downloader", et); !allowed {
			t.Errorf("expected glob match for %q", et)
		}
	}
	if allowed, _ := p.Allow("downloader", "media.imported"); allowed {
		t.Error("expected non-matching event to be denied")
	}
}

func TestAllow_WildcardCaller(t *testing.T) {
	p, err := Parse([]byte(`
- caller: "*"
  event_types: ["health.*"]
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if allowed, _ := p.Allow("any-module", "health.check"); !allowed {
		t.Error("expected wildcard caller to match any module")
	}
}

func TestAllow_WildcardEventType(t *testing.T) {
	p, err := Parse([]byte(`
- caller: "admin"
  event_types: ["*"]
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	for _, et := range []string{"anything", "a.b.c", "module.registered"} {
		if allowed, _ := p.Allow("admin", et); !allowed {
			t.Errorf("expected wildcard event_type to match %q", et)
		}
	}
}

func TestAllow_FirstMatchWins(t *testing.T) {
	p, err := Parse([]byte(`
- caller: "a"
  event_types: ["test.*"]
- caller: "*"
  event_types: ["test.*"]
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if allowed, _ := p.Allow("a", "test.x"); !allowed {
		t.Error("expected first rule to match")
	}
}

func TestAllow_CatchAll(t *testing.T) {
	p, err := Parse([]byte(`
- caller: "*"
  event_types: ["*"]
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if allowed, _ := p.Allow("anything", "anything"); !allowed {
		t.Error("expected catch-all to allow everything")
	}
}

func TestAllow_MultipleRules(t *testing.T) {
	p, err := Parse([]byte(`
- caller: "downloader"
  event_types: ["download.*"]
- caller: "transcoder"
  event_types: ["transcode.*"]
- caller: "notifier"
  event_types: ["notification.*", "alert.*"]
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	tests := []struct {
		caller    string
		eventType string
		allowed   bool
	}{
		{"downloader", "download.completed", true},
		{"downloader", "transcode.started", false},
		{"transcoder", "transcode.completed", true},
		{"transcoder", "download.completed", false},
		{"notifier", "notification.sent", true},
		{"notifier", "alert.critical", true},
		{"notifier", "download.completed", false},
	}
	for _, tt := range tests {
		allowed, _ := p.Allow(tt.caller, tt.eventType)
		if allowed != tt.allowed {
			t.Errorf("Allow(%q, %q) = %v, want %v", tt.caller, tt.eventType, allowed, tt.allowed)
		}
	}
}

func TestReplaceRules(t *testing.T) {
	p, err := Parse([]byte(`
- caller: "a"
  event_types: ["test.*"]
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	newP, err := Parse([]byte(`
- caller: "x"
  event_types: ["other.*"]
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	p.ReplaceRules(newP)

	if allowed, _ := p.Allow("a", "test.x"); allowed {
		t.Error("expected old rules to be replaced")
	}
	if allowed, _ := p.Allow("x", "other.x"); !allowed {
		t.Error("expected new rules to apply")
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/policy.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestLoad_ValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yaml")
	content := []byte("- caller: \"a\"\n  event_types: [\"test.*\"]\n")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	p, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if allowed, _ := p.Allow("a", "test.x"); !allowed {
		t.Error("expected loaded policy to allow matching publish")
	}
}

func TestMatchEventType_Exact(t *testing.T) {
	if !matchEventType("test.event", "test.event") {
		t.Error("expected exact match")
	}
	if matchEventType("test.event", "other.event") {
		t.Error("expected exact mismatch")
	}
}

func TestMatchEventType_Glob(t *testing.T) {
	tests := []struct {
		pattern   string
		eventType string
		match     bool
	}{
		{"download.*", "download.completed", true},
		{"download.*", "download.started", true},
		{"download.*", "download", true},
		{"download.*", "transcode.started", false},
		{"media.*", "media.imported", true},
		{"media.*", "media", true},
		{"media.*", "media.library.deleted", true},
		{"*", "anything", true},
		{".*", "anything", true},
	}
	for _, tt := range tests {
		got := matchEventType(tt.pattern, tt.eventType)
		if got != tt.match {
			t.Errorf("matchEventType(%q, %q) = %v, want %v", tt.pattern, tt.eventType, got, tt.match)
		}
	}
}

func TestAllow_RateLimit(t *testing.T) {
	p, err := Parse([]byte(`
- caller: "a"
  event_types: ["e.x"]
  rate_limit_per_min: 1
`))
	if err != nil {
		t.Fatal(err)
	}
	fixed := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	p.now = func() time.Time { return fixed }
	if ok, _ := p.Allow("a", "e.x"); !ok {
		t.Fatal("first allow")
	}
	if ok, _ := p.Allow("a", "e.x"); ok {
		t.Fatal("second should deny")
	}
}

func TestAllow_PayloadKeysAndMax(t *testing.T) {
	p, err := Parse([]byte(`
- caller: "a"
  event_types: ["e.x"]
  payload_max_bytes: 64
  payload_require_keys: ["id"]
`))
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := p.AllowWithPayload("a", "e.x", []byte(`{"id":"1"}`)); !ok {
		t.Fatal("expected allow")
	}
	if ok, _ := p.AllowWithPayload("a", "e.x", []byte(`{}`)); ok {
		t.Fatal("missing key should deny")
	}
	big := make([]byte, 100)
	for i := range big {
		big[i] = 'x'
	}
	if ok, _ := p.AllowWithPayload("a", "e.x", big); ok {
		t.Fatal("oversized should deny")
	}
}

func TestAllow_RequiredCapability(t *testing.T) {
	p, err := Parse([]byte(`
- caller: "a"
  event_types: ["*"]
  required_capability: "download"
`))
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := p.Allow("a", "capability.download"); !ok {
		t.Fatal("event type capability match")
	}
	if ok, _ := p.AllowWithPayload("a", "job.done", []byte(`{"capability":"download"}`)); !ok {
		t.Fatal("payload capability match")
	}
	if ok, _ := p.Allow("a", "other.event"); ok {
		t.Fatal("should deny without capability")
	}
}

func TestAuditExport(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")
	p, err := Parse([]byte(`
- caller: "a"
  event_types: ["ok.*"]
`))
	if err != nil {
		t.Fatal(err)
	}
	p.SetAuditPath(path)
	p.Allow("a", "ok.one")
	p.Allow("b", "ok.one")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"allowed":true`) || !strings.Contains(string(data), `"allowed":false`) {
		t.Fatalf("unexpected audit content: %s", data)
	}
}
