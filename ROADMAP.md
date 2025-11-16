# SkyChain Development Roadmap

This document outlines the evolutionary development path for SkyChain, breaking down the project into incremental, feature-focused milestones. Each step builds upon the last, progressively adding security, functionality, and robustness.

The full system specification can be found in [`plans/SPEC.md`](plans/SPEC.md).

---

### Phase 1: MVP — Core Ledger

The Minimum Viable Product establishes the foundational single-node blockchain.

*   **What's Added:** A single validator node that accepts events via an HTTP API, groups them into blocks at a regular interval, and persists the chain to a local JSON file. Block signing uses a simple shared secret (HMAC).

---

### Phase 2: Device Identity & Security

This phase introduces cryptographic identity for IoT devices, moving from an open system to a permissioned one.

*   **Goal 1: Device Registry:** A `devices.json` file is introduced to act as a whitelist, mapping device IDs to their public keys. The node loads this registry on startup.
*   **Goal 2: Signature Validation:** The node's `/event` endpoint is hardened to enforce that incoming events are cryptographically signed. It now rejects any event from an unknown device or with an invalid signature.

---

### Phase 3: Chain Integrity & Verification

This phase adds core blockchain features for compact verification and data integrity proofs.

*   **Goal 3: Merkle Trees:** Block headers are enhanced to include a Merkle root calculated from the block's transactions. This provides a compact cryptographic summary of the block's contents.
*   **Goal 4: Inclusion Proofs:** A new API endpoint (`/proof`) is created. It allows clients to request a cryptographic proof that a specific transaction is included in a block, without needing to download the entire block.
*   **Goal 5: Startup Audits:** The node gains the ability to validate the integrity of the entire blockchain on startup, checking all block hashes and signatures to detect any corruption or tampering.

---

### Phase 4: Decentralization — Multi-Validator Consensus

This is the major step toward a true Proof-of-Authority network, moving from a single validator to a committee.

*   **Goal 6: Block Proposal Rotation:** The system now supports a defined set of validators. A deterministic rotation scheme is implemented to decide which validator is the designated "proposer" for each block height.
*   **Goal 7: Quorum Signing:** The consensus logic is upgraded. A proposed block is only considered final (committed) after it has been received and co-signed by a quorum (e.g., 2/3+) of the validator set.
*   **Goal 8: Validator Governance:** The foundation is laid for managing the validator set itself via special on-chain transactions, allowing for validators to be added or removed.

---

### Phase 5: Observability

This phase focuses on making the network's behavior transparent and easy to monitor.

*   **Goal 9: Metrics:** The node begins exposing key operational metrics (e.g., block height, pending events, signature errors) in a Prometheus-compatible format.
*   **Goal 10: Web Dashboard:** A simple, built-in web UI is added to provide a human-readable view of the chain's status, recent blocks, and validator activity.