# Goal 2 — Device Signatures and Gateway Validation

**Goal:** Require signed IoT events and enforce verification before queueing.

## Features
1. Devices sign each event payload with their ED25519 key from the registry.
2. Gateway verifies signatures and rejects events from unknown devices.
3. Validator re-verifies signatures before block inclusion for defense in depth.

## Deliverables
- Signature verification library with deterministic encoding
- Gateway validation pipeline enforcing whitelist and signature checks
- Tests for valid, invalid, and replayed signatures
