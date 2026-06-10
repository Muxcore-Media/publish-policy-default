# publish-policy-default — Implementation Roadmap

**Priority:** P0 — Required before any module can emit events.

## Phases

### Phase 1: Capability Matching (minimum viable)
- [x] Project scaffold (this repo)
- [ ] `go mod init` with core dependency
- [ ] `PublishPolicyProvider` implementation (capability-based matching)
- [ ] `ResourcePublishPolicyProvider` implementation (payload-level checks)
- [ ] Sidecar entry point (`cmd/module/main.go`)
- [ ] `--allow-all` flag
- [ ] Unit tests for policy matching (30+ test cases)
- [ ] Integration test with a running muxcored
- [ ] GitHub CI (build + lint + test)

### Phase 2: Operational
- [ ] Audit logging of denied publishes
- [ ] Prometheus metrics (allowed/denied counters)
- [ ] Health endpoint (gRPC health check)
- [ ] Published events for policy violations
- [ ] Configurable event-type pattern (glob, prefix, regex)

### Phase 3: Advanced
- [ ] Dynamic policy based on event payload content
- [ ] Rate-limited event publication
- [ ] Per-event-type allow/deny overrides via config file
- [ ] Dead-letter integration for denied events

## Design Decisions

1. **Capability-based by default** — Matches the registry's capability system.
   No separate configuration file needed for basic operation.
2. **Also implements ResourcePublishPolicyProvider** — Gives core's event bus
   the option to do payload-level checks in the future.
3. **Simple, no external deps** — Pure Go, no YAML/TOML parser needed for MVP.
