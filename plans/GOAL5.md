# Goal 5 — Startup Validation and Auditing

**Goal:** Ensure persisted state remains trustworthy through automated verification routines.

## Features
1. Re-verify Merkle roots and block hashes during node startup.
2. Validate device signature history for the latest N blocks.
3. Emit audit logs detailing any detected inconsistencies.

## Deliverables
- Startup validator that halts on chain integrity failures
- Configurable audit depth for historical checks
- Tests simulating corrupted storage and invalid signatures
