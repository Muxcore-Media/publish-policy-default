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
| `MUXCORE_INSECURE_DISABLE_TLS` | unset | Set to `true` for insecure mesh registration (dev) |

Mesh registration also uses the module SDK (`MUXCORE_GRPC_ADDR`,
`MUXCORE_MODULE_ID`, `--muxcore-mesh-addr`, `--muxcore-module-id`).


### Advanced rules (v0.2+)

```yaml
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

Set `PUBLISH_POLICY_AUDIT_PATH` to append JSONL allow/deny records.

### Hot-Reload

SIGHUP reloads the policy file without restarting the module.

## Implementation

- Registers with capability: `"publish.policy"`
- Implements `contracts.PublishPolicyProvider` (gRPC `PolicyService.AllowPublish`)
- Denied publishes are audited by **core** at bus enforcement (not via module `AuditLogger`); module keeps counters/`slog`
- Registers `grpc_health_v1` (SERVING) on the module gRPC server
- Exposes metrics: `publish_policy_allowed_total`, `publish_policy_denied_total`
