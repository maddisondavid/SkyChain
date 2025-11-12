# Goal 3 — Merkle Roots for Block Integrity

**Goal:** Build Merkle trees over block transactions to enable compact verification.

## Features
1. Compute Merkle root for each block’s transaction list using deterministic hashing.
2. Persist Merkle root in block header alongside block hash.
3. Provide helper to recompute root for verification routines.

## Deliverables
- Merkle tree utility supporting even/odd leaf counts
- Updated block structures to store `MerkleRoot`
- Tests confirming root stability across reloads and order variations
