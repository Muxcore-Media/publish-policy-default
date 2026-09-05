# Publish Policy Default

Default event publication access control for MuxCore.

Without this module, every event publish is **denied by default**. Core's event
bus refuses all `bus.Publish()` calls until a module implementing
`PublishPolicyProvider` registers with capability `"publish.policy"`.

## How It Works

The module loads a static YAML policy (`policies.yaml`) that declares which
callers may publish which event types. The event bus consults this module
before delivering every publish.

```
Downloader publishes "download.completed"
        │
        ▼
MemoryBus.Publish()
        │
        ▼
publish-policy-default.CanPublish("downloader-qbittorrent", "download.completed")
        │
        ▼
  allowed? ───yes──→ deliver to subscribers
    │
   no
    │
    ▼
  return "publish denied" error
```

## Configuration

### Policy File (`policies.yaml`)

Rules are evaluated in order; the first match wins. If no rule matches, the
publish is denied. `caller: "*"` matches any module. Event types support
glob matching (`"download.*"`, `"media.*"`, `"*"`).

```yaml
# Allow a module to publish download-related events
- caller: "downloader-qbittorrent"
  event_types: ["download.*"]

# Allow any module to publish lifecycle events
- caller: "*"
  event_types: ["module.*", "cluster.*"]

# Development mode: allow all (uncomment only for local use)
# - caller: "*"
#   event_types: ["*"]
```

If the policy file is missing or invalid at startup, Init fails. Ship and
maintain an explicit `policies.yaml` (the repo includes a starter file).

### Environment

| Variable | Default | Description |
|----------|---------|-------------|
| `PUBLISH_POLICY_FILE` | `policies.yaml` | Path to policy YAML file |
| `PUBLISH_POLICY_GRPC_ADDR` | `127.0.0.1:9102` | Listen address for this module's gRPC server |
| `PUBLISH_POLICY_AUDIT_PATH` | unset | JSONL audit log for allow/deny decisions |
| `PUBLISH_POLICY_REGISTRY_MATCH` | unset | `1`/`true` enables registry capability matching (overrides YAML default) |
| `PUBLISH_POLICY_REGISTRY_SYNC_DELAY` | `5s` | Delay before initial registry bootstrap |
| `MUXCORE_INSECURE_DISABLE_TLS` | unset | Set to `true` for plaintext gRPC (dev only; also rewrites `:port` binds to loopback) |
| `MUXCORE_GRPC_INSECURE` | unset | Alias for `MUXCORE_INSECURE_DISABLE_TLS` |
| `PUBLISH_POLICY_TLS_CERT` / `PUBLISH_POLICY_TLS_KEY` | auto-generated | Server TLS certificate and key (falls back to `MUXCORE_TLS_CERT` / `MUXCORE_TLS_KEY`) |
| `PUBLISH_POLICY_TLS_CA` | auto-generated | Client CA bundle for optional mTLS (falls back to `MUXCORE_TLS_CA`) |
| `PUBLISH_POLICY_TLS_DIR` | `~/.muxcore/tls/publish-policy-default` | Directory for auto-generated dev certificates |

Mesh registration also uses the module SDK (`MUXCORE_GRPC_ADDR`,
`MUXCORE_MODULE_ID`, `--muxcore-mesh-addr`, `--muxcore-module-id`).


### Advanced rules (v0.2+)

```yaml
registry_capability_matching: true   # auto-allow event types from mesh module capabilities

groups:
  downloaders:
    - downloader-qbittorrent
rules:
  - caller_group: downloaders
    event_types: ["download.*"]
    rate_limit_per_min: 120
    payload_max_bytes: 65536
    payload_require_keys: ["id"]
    required_capability: "download"   # event type capability.download* or payload.capability
```

When `registry_capability_matching` is enabled (or `PUBLISH_POLICY_REGISTRY_MATCH=1`), static YAML rules are evaluated first. If none match, the module allows publishes when the caller's registered capabilities imply the event type (for example capability `media.movies` → `media.*`, `settings` → `module.*`).

Set `PUBLISH_POLICY_AUDIT_PATH` to append JSONL allow/deny records.

### Hot-Reload

SIGHUP reloads the policy file without restarting the module.

## Implementation

- Registers with capability: `"publish.policy"`
- Implements `contracts.PublishPolicyProvider` (gRPC `PolicyService.AllowPublish`)
- Denied publishes are audited by **core** at bus enforcement (not via module `AuditLogger`); module keeps counters/`slog`
- Registers `grpc_health_v1` (SERVING) on the module gRPC server
- Exposes metrics: `publish_policy_allowed_total`, `publish_policy_denied_total`
