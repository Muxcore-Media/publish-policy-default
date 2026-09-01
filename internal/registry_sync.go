package internal

import (
	"context"
	"encoding/json"
	"fmt"
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
	if m.mc != nil {
		_ = m.mc.Close()
	}
	m.mc = c
	slog.Info("publish-policy: connected to core for registry capability sync", "addr", addr)
}

func (m *Module) syncRegistryFromDiscovery(ctx context.Context) error {
	if m.mc == nil || m.policy == nil || !m.policy.RegistryMatchingEnabled() {
		return nil
	}
	resp, err := m.mc.Discovery.Raw().ListAll(ctx, &discoveryv1.ListAllRequest{})
	if err != nil {
		return fmt.Errorf("ListAll: %w", err)
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
	return nil
}

func registrySyncDelay() time.Duration {
	delay := 5 * time.Second
	if v := os.Getenv("PUBLISH_POLICY_REGISTRY_SYNC_DELAY"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			delay = d
		}
	}
	return delay
}

func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func (m *Module) subscribeRegistryEvents() {
	if m.policy == nil {
		return
	}

	m.registryMu.Lock()
	if m.registrySyncStarted {
		m.registryMu.Unlock()
		if m.policy.RegistryMatchingEnabled() {
			if m.mc == nil {
				m.dialCoreForRegistry()
			}
			if m.mc != nil {
				_ = m.syncRegistryFromDiscovery(context.Background())
			}
		}
		return
	}
	m.registrySyncStarted = true
	ctx, cancel := context.WithCancel(context.Background())
	m.registryCancel = cancel
	done := make(chan struct{})
	m.registryDone = done
	m.registryMu.Unlock()

	go m.runRegistrySyncLoop(ctx, done)
}

func (m *Module) runRegistrySyncLoop(ctx context.Context, done chan struct{}) {
	defer close(done)

	if !m.policy.RegistryMatchingEnabled() {
		slog.Info("publish-policy: registry capability matching disabled")
	}

	retryDelay := registrySyncDelay()
	for {
		if ctx.Err() != nil {
			return
		}
		if m.policy == nil || !m.policy.RegistryMatchingEnabled() {
			if err := sleepContext(ctx, time.Second); err != nil {
				return
			}
			continue
		}

		if err := sleepContext(ctx, retryDelay); err != nil {
			return
		}
		if err := m.registrySyncSession(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Warn("publish-policy: registry sync failed, retrying", "error", err)
			if err := sleepContext(ctx, 5*time.Second); err != nil {
				return
			}
			continue
		}
		retryDelay = registrySyncDelay()
	}
}

func (m *Module) registrySyncSession(ctx context.Context) error {
	if m.mc == nil {
		m.dialCoreForRegistry()
	}
	if m.mc == nil {
		return fmt.Errorf("no mesh client")
	}
	if err := m.syncRegistryFromDiscovery(ctx); err != nil {
		return err
	}

	type stream struct {
		eventType string
		ch        <-chan *eventsv1.Event
		cancel    context.CancelFunc
	}
	var streams []stream
	defer func() {
		for _, s := range streams {
			s.cancel()
		}
	}()

	for _, et := range []string{contracts.EventModuleRegistered, contracts.EventModuleUnregistered} {
		ch, cancel, err := m.mc.Events.Subscribe(ctx, et)
		if err != nil {
			return fmt.Errorf("subscribe %s: %w", et, err)
		}
		streams = append(streams, stream{eventType: et, ch: ch, cancel: cancel})
		slog.Info("publish-policy: subscribed for registry capability sync", "type", et)
	}

	errCh := make(chan error, len(streams))
	for _, s := range streams {
		go func(st stream) {
			errCh <- m.handleRegistryEventStream(ctx, st.eventType, st.ch)
		}(s)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

func (m *Module) handleRegistryEventStream(ctx context.Context, eventType string, ch <-chan *eventsv1.Event) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case evt, ok := <-ch:
			if !ok {
				return fmt.Errorf("registry event stream closed: %s", eventType)
			}
			switch eventType {
			case contracts.EventModuleRegistered:
				m.applyModuleRegistered(evt)
			case contracts.EventModuleUnregistered:
				m.applyModuleUnregistered(evt)
			}
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
