# Goal 1 — Device Registry and Key Management

**Goal:** Establish trusted device identity with a managed public key registry.

## Features
1. Maintain `devices.json` whitelist mapping `device_id` to ED25519 public key.
2. Provide tooling to add, update, or revoke device entries with audit trail.
3. Load registry on startup and hot-reload when the file changes.

## Deliverables
- Registry loader with validation of key formatting and duplicates
- File watcher or reload command for runtime updates
- Tests covering registry parsing and invalid entry handling
