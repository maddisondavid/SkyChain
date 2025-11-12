# Goal 4 — Inclusion Proof API

**Goal:** Expose a proof endpoint so clients can verify event inclusion via Merkle branches.

## Features
1. Provide `GET /proof?txid=` endpoint returning Merkle branch and block metadata.
2. Validate requested transaction exists before generating proof.
3. Include lightweight client verifier example for proof checking.

## Deliverables
- HTTP handler that builds proofs from stored blocks
- Proof serialization format documented in API reference
- Tests covering proof generation and invalid transaction lookup
