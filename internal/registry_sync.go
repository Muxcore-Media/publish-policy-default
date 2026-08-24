package internal

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/Muxcore-Media/core/pkg/contracts"
	discoveryv1 "github.com/Muxcore-Media/core/proto/gen/muxcore/discovery/v1"
	eventsv1 "github.com/Muxcore-Media/core/proto/gen/muxcore/events/v1"
	"github.com/Muxcore-Media/core/sdk/go/client"
)

func envRegistryMatchingOverride() *bool {
	v := strings.TrimSpace(os.Getenv("PUBLISH_POLICY_REGISTRY_MATCH"))
	if v == "" {
		return nil
	}
	on := envTruthy(v)
	return &on
}

func envTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func (m *Module) applyRegistryMatchingDefaults() {
	if override := envRegistryMatchingOverride(); override != nil {
		m.policy.SetRegistryMatching(*override)
	}
}

func (m *Module) dialCoreForRegistry() {
	addr := strings.TrimSpace(os.Getenv("MUXCORE_GRPC_ADDR"))
	if addr == "" {
		return
	}
	var opts []client.Option
	if os.Getenv("MUXCORE_INSECURE_DISABLE_TLS") == "true" || os.Getenv("MUXCORE_GRPC_INSECURE") == "true" {
		opts = append(opts, client.WithInsecure())
	}
	c, err := client.Dial(addr, opts...)
	if err != nil {
		slog.Warn("publish-policy: dial core for registry sync", "error", err)
		return
	}
	m.mc = c
	slog.Info("publish-policy: connected to core for registry capability sync", "addr", addr)
}

func (m *Module) syncRegistryFromDiscovery(ctx context.Context) {
	if m.mc == nil || m.policy == nil || !m.policy.RegistryMatchingEnabled() {
		return
	}
	resp, err := m.mc.Discovery.Raw().ListAll(ctx, &discoveryv1.ListAllRequest{})
	if err != nil {
		slog.Debug("publish-policy: ListAll failed", "error", err)
		return
	}
	count := 0
	for _, entry := range resp.GetEntries() {
		info := entry.GetInfo()
		if info == nil || info.GetId() == "" {
			continue
		}
		m.policy.UpsertRegistryModule(info.GetId(), info.GetCapabilities())
		count++
	}
	if count > 0 {
		slog.Info("publish-policy: registry capability map bootstrapped", "modules", count)
	}
}

func (m *Module) subscribeRegistryEvents() {
	if m.policy == nil || !m.policy.RegistryMatchingEnabled() {
		slog.Info("publish-policy: registry capability matching disabled")
		return
	}

	m.registryMu.Lock()
	alreadyStarted := m.registrySyncStarted
	if !alreadyStarted {
		m.registrySyncStarted = true
	}
	m.registryMu.Unlock()

	if alreadyStarted {
		if m.mc == nil {
			m.dialCoreForRegistry()
		}
		if m.mc != nil {
			m.syncRegistryFromDiscovery(context.Background())
		}
		return
	}

	delay := 5 * time.Second
	if v := os.Getenv("PUBLISH_POLICY_REGISTRY_SYNC_DELAY"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			delay = d
		}
	}
	if delay > 0 {
		time.Sleep(delay)
	}
	if m.mc == nil {
		m.dialCoreForRegistry()
	}
	if m.mc == nil {
		slog.Warn("publish-policy: no mesh client; registry capability sync disabled")
		return
	}

	ctx := context.Background()
	m.syncRegistryFromDiscovery(ctx)

	for _, et := range []string{contracts.EventModuleRegistered, contracts.EventModuleUnregistered} {
		ch, cancel, err := m.mc.Events.Subscribe(ctx, et)
		if err != nil {
			slog.Warn("publish-policy: registry subscribe failed", "type", et, "error", err)
			continue
		}
		go m.handleRegistryEventStream(et, ch, cancel)
		slog.Info("publish-policy: subscribed for registry capability sync", "type", et)
	}
}

func (m *Module) handleRegistryEventStream(eventType string, ch <-chan *eventsv1.Event, cancel context.CancelFunc) {
	defer cancel()
	for evt := range ch {
		switch eventType {
		case contracts.EventModuleRegistered:
			m.applyModuleRegistered(evt)
		case contracts.EventModuleUnregistered:
			m.applyModuleUnregistered(evt)
		}
	}
}

func (m *Module) applyModuleRegistered(evt *eventsv1.Event) {
	if m.policy == nil || evt == nil {
		return
	}
	var payload contracts.ModuleRegisteredPayload
	if len(evt.Payload) > 0 {
		if err := json.Unmarshal(evt.Payload, &payload); err != nil {
			slog.Debug("publish-policy: module.registered payload parse failed", "error", err)
		}
	}
	moduleID := strings.TrimSpace(payload.ModuleID)
	if moduleID == "" {
		moduleID = strings.TrimSpace(evt.Source)
	}
	if moduleID == "" {
		return
	}
	caps := m.resolveModuleCapabilities(context.Background(), moduleID)
	m.policy.UpsertRegistryModule(moduleID, caps)
	slog.Debug("publish-policy: registry module updated", "module", moduleID, "capabilities", len(caps))
}

func (m *Module) applyModuleUnregistered(evt *eventsv1.Event) {
	if m.policy == nil || evt == nil {
		return
	}
	var payload contracts.ModuleUnregisteredPayload
	if len(evt.Payload) > 0 {
		_ = json.Unmarshal(evt.Payload, &payload)
	}
	moduleID := strings.TrimSpace(payload.ModuleID)
	if moduleID == "" {
		moduleID = strings.TrimSpace(evt.Source)
	}
	if moduleID == "" {
		return
	}
	m.policy.RemoveRegistryModule(moduleID)
	slog.Debug("publish-policy: registry module removed", "module", moduleID)
}

func (m *Module) resolveModuleCapabilities(ctx context.Context, moduleID string) []string {
	if m.mc == nil {
		return nil
	}
	info, err := m.mc.Discovery.Resolve(ctx, moduleID)
	if err != nil || info == nil {
		return nil
	}
	return info.GetCapabilities()
}
