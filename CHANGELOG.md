# Changelog

## [0.3.0] — 2026-08-20

### Added

- Registry capability matching: auto-allow event types derived from registered module capabilities (`registry_capability_matching`, `PUBLISH_POLICY_REGISTRY_MATCH`)
- Mesh bootstrap via Discovery `ListAll` plus live sync on `module.registered` / `module.unregistered`
- Admin setting `registry_capability_matching`

## [0.2.2] — 2026-08-10

### Added

- Advertise `settings` capability so admin-ui discovers SettingsProvider without ListAll probing.

## [0.2.0] — 2026-08-09

### Added

- Payload checks: `payload_max_bytes`, `payload_require_keys`
- Dynamic capability matching via `required_capability`
- Event rate limiting: `rate_limit_per_min`
- Caller groups (`groups` + `caller_group`)
- JSONL audit export via `PUBLISH_POLICY_AUDIT_PATH`
- Document form `{ groups, rules }` alongside legacy rule lists

## [0.1.1]

### Changed

- Core pin / CI updates

## [0.1.0]

### Added

- Static YAML publish policy with SIGHUP reload
