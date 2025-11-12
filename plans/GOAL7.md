# Goal 7 — Quorum Signing and Finalization

**Goal:** Require a quorum of validator signatures before committing blocks.

## Features
1. Collect countersignatures from validators after a proposal is broadcast.
2. Define configurable quorum threshold enforced per block height.
3. Reject conflicting signatures and record equivocation evidence.

## Deliverables
- Consensus message flows for propose, vote, and commit stages
- Signature aggregation structure stored in block headers
- Tests for quorum success, insufficient signatures, and equivocation detection
