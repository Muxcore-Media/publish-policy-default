# publish-policy-default — Remaining Work

### Operational
- [x] Audit logging of denied publishes — core fire-and-forget at bus enforcement (`event.publish.denied`); module keeps counters/`slog`
- [x] Health endpoint (gRPC health check)

### Advanced
- [x] Dynamic capability-based matching — `registry_capability_matching` + mesh `ListAll` bootstrap and `module.registered` / `module.unregistered` sync
- [x] Payload-level checks — YAML `payload_max_bytes` / `payload_require_keys`; core forwards payload via `ResourcePublishPolicyProvider` (`SidecarPublishPolicy.CanPublishEvent`)
- [x] Event rate limiting per caller — `rate_limit_per_min` in policy rules
- [x] Policy audit log export — JSONL via `PUBLISH_POLICY_AUDIT_PATH`
