# publish-policy-default — Remaining Work

### Operational
- [x] Audit logging of denied publishes — core fire-and-forget at bus enforcement (`event.publish.denied`); module keeps counters/`slog`
- [x] Health endpoint (gRPC health check)

### Advanced
- [ ] Dynamic capability-based matching (auto-detect event types from registry)
- [ ] Payload-level checks (ResourcePublishPolicyProvider)
- [ ] Event rate limiting per caller
- [ ] Policy audit log export
