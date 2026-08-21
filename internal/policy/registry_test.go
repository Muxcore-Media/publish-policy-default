package policy

import "testing"

func TestRegistryAllowsCapabilities(t *testing.T) {
	p, err := Parse([]byte(`
registry_capability_matching: true
rules:
  - caller: "blocked-only"
    event_types: ["blocked.*"]
`))
	if err != nil {
		t.Fatal(err)
	}
	p.UpsertRegistryModule("media-movies", []string{"media.movies", "settings"})

	if ok, _ := p.Allow("media-movies", "media.imported"); !ok {
		t.Fatal("expected media.* from media.movies capability")
	}
	if ok, _ := p.Allow("media-movies", "module.registered"); !ok {
		t.Fatal("expected module.* from settings capability")
	}
	if ok, _ := p.Allow("media-movies", "download.completed"); ok {
		t.Fatal("download should not match media capabilities")
	}
	if ok, _ := p.Allow("unknown-module", "media.imported"); ok {
		t.Fatal("unknown module should deny")
	}
}

func TestRegistryCapabilityPayloadMatch(t *testing.T) {
	p, err := Parse([]byte(`registry_capability_matching: true`))
	if err != nil {
		t.Fatal(err)
	}
	p.UpsertRegistryModule("dl", []string{"download"})
	if ok, _ := p.AllowWithPayload("dl", "job.done", []byte(`{"capability":"download"}`)); !ok {
		t.Fatal("payload capability field should match")
	}
}

func TestRegistryDisabledByDefault(t *testing.T) {
	p, err := Parse([]byte(`rules: []`))
	if err != nil {
		t.Fatal(err)
	}
	p.UpsertRegistryModule("media-movies", []string{"media.movies"})
	if ok, _ := p.Allow("media-movies", "media.imported"); ok {
		t.Fatal("registry matching should be off by default")
	}
}

func TestRegistrySurvivesReplaceRules(t *testing.T) {
	p, _ := Parse([]byte(`registry_capability_matching: true`))
	p.UpsertRegistryModule("a", []string{"media.movies"})
	newP, _ := Parse([]byte(`registry_capability_matching: true`))
	p.ReplaceRules(newP)
	if p.RegistryModuleCount() != 1 {
		t.Fatalf("registry map should survive reload, count=%d", p.RegistryModuleCount())
	}
}

func TestEventPatternsForRegistryCapability(t *testing.T) {
	patterns := eventPatternsForRegistryCapability("media.movies")
	if len(patterns) == 0 {
		t.Fatal("expected patterns")
	}
	foundMedia := false
	for _, pat := range patterns {
		if pat == "media.*" {
			foundMedia = true
		}
	}
	if !foundMedia {
		t.Fatalf("expected media.* in %v", patterns)
	}
	infra := eventPatternsForRegistryCapability("settings")
	if len(infra) != 1 || infra[0] != "module.*" {
		t.Fatalf("settings infra patterns = %v", infra)
	}
}
