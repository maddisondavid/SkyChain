# SkyChain SPEC — IoT Proof-of-Authority Blockchain

This document is the **authoritative specification** for SkyChain, a single-binary Go application that implements a modular IoT blockchain using **Proof-of-Authority (PoA)** consensus. It provides everything an agentic coder needs to build, test, and extend the system.

---

## 1. Problem Statement & Goals

**Problem:** Collect IoT readings from many devices and record them in an **append-only, tamper-evident ledger** that readers can verify independently without trusting a single database.

**Primary Goals**
- Deterministic, scheduled block production (no mining, no tokens).
- Permissioned validators sign blocks; gateways/light clients submit events.
- Pluggable storage backend (JSON for MVP, scalable later).
- Compact verification (Merkle roots & inclusion proofs).
- Simple HTTP API; single Go binary with role-based runtime flags.

**Non-Goals (v1)**
- Public, permissionless participation.
- Economic incentives, tokenomics.
- Smart contracts, VM execution.
- Heavy BFT (PBFT/Tendermint) — pragmatic PoA path.

---

## 2. System Roles

- **Device (producer):** Signs events `{device_id, nonce, ts, payload, sig}` and sends to a gateway.
- **Gateway (light node):** Verifies device signatures, buffers events, submits to validators. Can operate offline and resync.
- **Validator (core):** Proposes blocks on a fixed interval, verifies & signs blocks, maintains the canonical chain.
- **Reader/Archive (optional):** Full history node for analytics; no consensus duties.

Run-time roles via `--role=validator|gateway|reader`.

---

## 3. Data Model

### 3.1 Transaction (IoT Event)
```jsonc
{
  "id": "sha256(payload + device_id + nonce + ts)",
  "device_id": "string",
  "nonce": 1,
  "ts": 1710000000,
  "payload": "opaque string or base64 bytes",
  "sig": "base64"
}

Validation:


device_id is in DeviceRegistry; signature verifies with its pubkey.


nonce > last seen per device (anti-replay). MVP can keep in memory.


ts within ±drift window; late data allowed but marked.


### 3.2 Block
{
  "header": {
    "height": 1234,
    "prev_hash": "hex32",
    "merkle_root": "hex32",
    "timestamp": 1710000010,
    "proposer_id": "validator-A",
    "signatures": { "validator-A": "base64" }
  },
  "txs": [ /* array of Tx */ ],
  "hash": "hex32"
}

Hashing:


SHA-256 over canonical encodings (sorted keys, no whitespace).


Block.hash = sha256(encode(header) || encode(tx_root)) (MVP may hash full tx array).


4. Consensus
4.1 MVP — PoA-Lite (Single Validator)


Single validator ticks every --interval (default 10s).


Collect pending events → build block → sign header → append to store.


4.2 Goals 6-8 — Multi-Validator PoA (Quorum)


Static ValidatorRegistry lists allowed validators and quorum (e.g., n=5, quorum=3).


Proposer rotation by height; peers verify and countersign.


Commit when quorum signatures obtained; timeout/view-change rotates leader.


One-signature-per-(height,round); equivocation evidence triggers revocation.


Messages (HTTP/gRPC):


Proposal{header, tx_root} / Vote{height,round,block_hash,sig} / Commit{header, sigmap}.


5. APIs (JSON over HTTP)
Public:


POST /event → {device_id, nonce, ts, payload, sig?} → {"accepted":true,"txid":"..."}


GET /head → {"height":N,"hash":"...","timestamp":...,"proposer_id":"...","sig_count":K}


GET /chain?from=H&to=H2 → bounded block stream


GET /proof?txid=... (Goal 4) → Merkle branch


GET /health → {"ok":true}


Validator RPC (mTLS):


POST /consensus/propose, /vote, /commit


6. Storage & Modularity
Interfaces:
type ChainReader interface{ Head(ctx context.Context)(core.Block,error); GetByHeight(ctx context.Context,h uint64)(core.Block,error); Iterate(ctx context.Context,from,to uint64,fn func(core.Block) error) error }
type ChainWriter interface{ Append(ctx context.Context,b core.Block) error; ReplaceHead(ctx context.Context,from uint64,bs []core.Block) error }
type ChainStore interface{ ChainReader; ChainWriter; Close() error }
type TxQueue interface{ Enqueue(ctx context.Context,t core.Tx) error; Drain(max int)([]core.Tx,error) }

MVP driver: JSONL (data/chain.jsonl) + fsync; in-memory head cache + offsets.
Future: Badger/RocksDB, S3, SQL, Kafka.

7. Crypto & Registries


Device keys: ED25519; devices.json → {device_id: base64_pubkey}.


Validator keys: ED25519; validators.json → {id: base64_pubkey, ..., "quorum":3}.


Rotation & revocation in Goals 6-8 via signed TXs.


8. Security & Abuse Controls


mTLS for validator RPC; API keys or mTLS for gateway→validator.


Per-device nonce, timestamp windows, rate limits.


Reject unknown devices; log invalid signatures.


Reject proposals with mismatched previous_hash.


One-signature-per-height; collect equivocation evidence.


9. Configuration Flags
--role=validator|gateway|reader
--listen=:8080
--data=./data/chain.jsonl
--interval=10s
--devices=./config/devices.json
--validators=./config/validators.json
--quorum=3
--proposal-timeout=3s
--enable-metrics
--ui


10. Metrics (Prometheus)


skychain_block_height (gauge)


skychain_block_duration_seconds (summary)


skychain_pending_events (gauge)


skychain_invalid_signatures_total (counter)


skychain_quorum_latency_seconds (summary, Goals 6-8)


skychain_gateway_backlog (gauge)


11. Web UI (Goal 10)


/ui static page: head height, recent blocks, validator set, event rate, submit test event.


12. Testing & Acceptance
Unit:


Block continuity, genesis, reload.


Signature verify (valid/invalid), nonce monotonic.


Merkle root + inclusion proofs (Goals 3-4).


Integration:


Submit events → next tick → block appended → /chain reflects state.


Offline gateway store-and-forward.


Multi-validator: conflicting proposals don’t commit; quorum commit succeeds (Goals 6-8).


Acceptance (MVP):


Start validator; POST events; scheduled blocks appear; state persists and reloads.


/head height increases; README demo works on fresh machine.


13. Layout
/cmd/skychain
/internal/core
/internal/storage
/internal/consensus
/internal/api
/internal/keys
/internal/merkle
/internal/metrics
/plans

14. Build & Run
go run ./cmd/skychain --role=validator --interval=10s --data=./data/chain.jsonl
curl -sX POST localhost:8080/event -H 'Content-Type: application/json' -d '{"device_id":"sensor-1","nonce":1,"ts":1710000000,"payload":"22.5°C"}'
curl -s localhost:8080/head | jq .

15. Implementation Guidance


Stdlib-first. Keep interfaces stable. JSONL store first. Small PRs with tests. No breaking flags between phases. Graceful shutdown.

Others (maybe this is for the Agents.md file)

- Code shoud be Writen in go
- GO code should be in /pkg
- Main command should be in /cmd/skychain

Add an Agents.md file


**End of prompt.**
