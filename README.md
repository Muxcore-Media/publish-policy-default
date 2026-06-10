# Publish Policy Default

Default event publication access control for MuxCore.

Without this module, every event publish is **denied by default**. Core's event
bus refuses all `bus.Publish()` calls until a module implementing
`PublishPolicyProvider` registers with capability `"publish.policy"`.

## How It Works

The module matches event types against the publisher's declared capabilities.
A module that declares capability `"downloader.torrent"` is allowed to publish
events matching the pattern `"download.*"`. This capability-to-event-type
mapping is the default policy.

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

### CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--allow-all` | `false` | Permit all event publication (development) |
| `--deny-all` | `false` | Deny all event publication (locked down) |

### Default Capability Mapping

Without a mapping file, the module allows publishes when the event type
matches the caller's declared capability as a prefix:

```
Capability "downloader.torrent" → allows "download.*" events
Capability "downloader.*"       → allows "download.*" events
Capability "media_manager"      → allows "media.*" events
Capability "*"                  → allows all events
```

## Implementation

- Registers with capability: `"publish.policy"`
- Implements `contracts.PublishPolicyProvider`
- Also implements `contracts.ResourcePublishPolicyProvider` for payload-level checks
- Audits denied publishes
- Exposes metrics: `publish_policy_allowed_total`, `publish_policy_denied_total`
