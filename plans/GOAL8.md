# Goal 8 — Validator Governance Transactions

**Goal:** Enable on-chain management of the validator set through signed governance actions.

## Features
1. Define governance transaction types for adding and removing validators.
2. Require quorum approval before applying registry changes.
3. Persist governance events in the chain for auditability.

## Deliverables
- Governance transaction schema and validation rules
- State transition logic updating `validators.json` or equivalent store
- Tests covering add/remove flows and rejection of unauthorized proposals
