package policy

import (
	"path/filepath"
	"runtime"
	"testing"
)

func shippedPoliciesPath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "policies.yaml")
}

func TestShippedPoliciesYAML(t *testing.T) {
	p, err := Load(shippedPoliciesPath(t))
	if err != nil {
		t.Fatal(err)
	}

	allowCases := []struct {
		caller    string
		eventType string
	}{
		{"notification-default", "notification.sent"},
		{"jellyfin", "playback.started"},
		{"emby", "playback.paused"},
		{"plex", "playback.stopped"},
		{"playback-guard", "playback.guard.violation"},
		{"media-list-sync", "importlist.synced"},
		{"health-monitor", "health.degraded"},
		{"downloader-sabnzbd", "download.completed"},
		{"downloader-native-usenet", "download.failed"},
		{"downloader-debrid", "download.queued"},
		{"indexer-torznab", "indexer.result"},
		{"indexer-piratebay", "indexer.search"},
		{"media-movies", "media.imported"},
		{"unknown-module", "module.registered"},
	}
	for _, tc := range allowCases {
		if ok, reason := p.Allow(tc.caller, tc.eventType); !ok {
			t.Errorf("Allow(%q, %q) denied: %s", tc.caller, tc.eventType, reason)
		}
	}

	denyCases := []struct {
		caller    string
		eventType string
	}{
		{"unknown-module", "download.completed"},
		{"unknown-module", "media.imported"},
		{"media-list-sync", "media.imported"},
		{"media-list-sync", "media.updated"},
		{"config-watcher", "importlist.synced"},
		{"config-watcher", "playback.started"},
		{"config-watcher", "health.degraded"},
		{"auth-local", "playback.started"},
		{"auth-local", "health.degraded"},
	}
	for _, tc := range denyCases {
		if ok, _ := p.Allow(tc.caller, tc.eventType); ok {
			t.Errorf("Allow(%q, %q) should be denied", tc.caller, tc.eventType)
		}
	}
}
