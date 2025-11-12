# Goal 6 — Multi-Validator Proposal Rotation

**Goal:** Introduce multiple validators taking turns proposing blocks on a fixed schedule.

## Features
1. Define `validators.json` with validator IDs and public keys.
2. Implement round-robin proposer selection based on block height.
3. Handle proposer timeout by rotating to the next validator.

## Deliverables
- Validator registry loader shared with consensus engine
- Scheduler for proposer rotation with timeout handling
- Tests verifying proposer order and timeout-driven rotation
