# Compatibility

## Core Version

Requires MuxCore v0.4.0 or later.

## Capabilities

Registers with capabilities:

- `publish.policy` — gRPC `PolicyService.AllowPublish` enforcement
- `settings` — admin-ui settings provider (policy file, audit path, registry matching, reload, metrics counters)

## v0.3 Registry Matching

When `registry_capability_matching` is enabled (YAML or `PUBLISH_POLICY_REGISTRY_MATCH=1`), static YAML rules are evaluated first. If none match, publishes are allowed when the caller's registered mesh capabilities imply the event type (for example `media.movies` → `media.*`, `settings` → `module.*`). Payload JSON `capability` fields do **not** grant registry matches — only YAML `required_capability` rules use payload matching after caller and event type already matched.

Registry state bootstraps via Discovery `ListAll` and live sync on `module.registered` / `module.unregistered` (see `PUBLISH_POLICY_REGISTRY_SYNC_DELAY`).

## Contract Dependencies

- `github.com/Muxcore-Media/core/pkg/contracts` — PublishPolicyProvider interface
- gRPC ModuleRegistration service for sidecar registration
- gRPC DiscoveryService for module discovery
