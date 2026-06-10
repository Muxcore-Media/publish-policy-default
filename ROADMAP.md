# publish-policy-default — Implementation Roadmap

**Priority:** P0 — Required before any module can emit events.

## Phases

### Phase 1: Glob-Based Matching (minimum viable) ✅
- [x] Project scaffold
- [x] `go mod init` with core dependency
- [x] YAML policy file parser with glob matching (`internal/policy`)
- [x] gRPC PolicyService server for core queries (`internal/server`)
- [x] Sidecar entry point (`cmd/module/main.go`)
- [x] SIGHUP hot-reload for policy file
- [x] `--allow-all` flag
- [x] Sample `policies.yaml`
- [x] Unit tests for policy matching (25+ test cases)
- [x] Unit tests for gRPC server (5 test cases)
- [x] Core: re-wire policies after sidecar module spawns

### Phase 2: Operational
- [ ] Audit logging of denied publishes
- [ ] Prometheus metrics (allowed/denied counters)
- [ ] Health endpoint (gRPC health check)
- [ ] Integration test with running muxcored
- [ ] GitHub CI (build + lint + test)

### Phase 3: Advanced
- [ ] Dynamic capability-based matching (auto-detect event types from registry)
- [ ] Payload-level checks (ResourcePublishPolicyProvider)
- [ ] Event rate limiting per caller
- [ ] Policy audit log export

## Design Decisions

1. **Capability-based by default** — Matches the registry's capability system.
   No separate configuration file needed for basic operation.
2. **Also implements ResourcePublishPolicyProvider** — Gives core's event bus
   the option to do payload-level checks in the future.
3. **Simple, no external deps** — Pure Go, no YAML/TOML parser needed for MVP.
