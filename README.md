# SkyChain
A lightweight proof-of-authority (PoA) blockchain tailored for trustworthy IoT telemetry.

SkyChain records IoT readings in an append-only ledger produced by a small, curated validator set. The project focuses on reliable data provenance, easy verification by gateways, and modular components that can evolve without reshaping the entire stack.

![Title](Images/TitleImage.png)

---

## What SkyChain Delivers
- **Deterministic PoA ledger** — Validators take scheduled turns producing blocks, removing the need for mining or tokens.
- **Signed IoT events** — Devices submit authenticated readings that can be traced back to their origin.
- **Gateway friendliness** — Intermediate nodes buffer, verify, and forward events while remaining lightweight clients.
- **Pluggable storage** — Chain data is persisted behind a storage interface so different backends can be swapped in as requirements grow.

These pillars stay constant even as specific protocols, storage engines, or deployment choices evolve.

---

## System Building Blocks
| Component | Purpose |
|-----------|---------|
| Devices | Emit signed telemetry payloads tagged with device metadata.|
| Gateways | Collect device messages, perform lightweight checks, and relay them to validators.|
| Validators | Author and sign blocks on a fixed cadence to extend the canonical chain.|
| ChainStore | Abstract persistence layer that supports the validator workflow.|

---

## Development Themes
SkyChain is iterated through focused milestones captured under [`/plans`](plans). Each plan documents a cohesive slice of functionality (e.g., single-validator MVP, signature hardening, multi-validator expansion). The README stays high-level; refer to those living documents for step-by-step tasks.

Key themes guiding future work:
- Gradual hardening of device identity and key management.
- Scaling from a single validator to a committee-based PoA cluster.
- Enhancing observability and external integrations without compromising simplicity.

---

## Repository Layout
```
├── cmd/skychain      # Entry point for running validator or gateway roles
├── pkg/              # Go packages implementing ledger, networking, and storage logic
├── plans/            # Milestone documents outlining implementation phases
├── Images/           # Project diagrams and artwork
└── README.md         # High-level project overview (this file)
```

---

## MVP Validator
The initial MVP targets a single-validator setup that accepts IoT events over HTTP, seals them into blocks every 10 seconds, and persists the ledger as JSON on disk.

### Build
```bash
go build ./cmd/skychain
```

### Run
```bash
./skychain \
  --addr ":8080" \
  --data data/chain.json \
  --interval 10s \
  --validator skychain-validator \
  --secret skychain-local-secret \
  --devices config/devices.json
```

The node listens on the configured address and writes the blockchain to the provided `--data` path. On shutdown (Ctrl+C) it flushes pending events and persists the latest chain snapshot.

### REST API
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/event` | `POST` | Submit an IoT event `{device_id, nonce, ts, payload, sig}`. `sig` is a base64 ED25519 signature over the canonical event payload. `ts` must be RFC3339; it defaults to the arrival time when omitted. |
| `/chain` | `GET` | Retrieve the full chain as JSON. |
| `/head`  | `GET` | Inspect metadata for the most recent block. |
| `/health`| `GET` | Node status (validator id, block count, pending queue length). |

Example event submission:
```bash
curl -X POST http://localhost:8080/event \
  -H 'Content-Type: application/json' \
  -d '{
    "device_id": "sensor-1",
    "nonce": 42,
    "ts": "2024-01-02T15:04:05Z",
    "payload": {"temp": 21.7},
    "sig": "BASE64_SIGNATURE"
  }'
```

Requests from devices that do not appear in the configured registry or that present invalid signatures receive `403 Forbidden` responses. Each device nonce must be strictly increasing; replayed nonces result in `409 Conflict` responses.

---

## Device Registry and Tooling

SkyChain ships with a managed ED25519 public-key registry stored in `devices.json`. The validator daemon loads the file on startup and continuously polls it for changes, enabling hot updates without restarting the node.

Use the `devicectl` helper to manage registry entries and emit an audit trail:

```bash
go build ./cmd/devicectl
./devicectl --devices config/devices.json --audit config/devices.audit.log add sensor-3 Abase64Key==
./devicectl --devices config/devices.json update sensor-1 NewBase64Key==
./devicectl --devices config/devices.json revoke sensor-2
./devicectl --devices config/devices.json list
```

Each command validates that keys decode to 32-byte ED25519 public keys, preserves deterministic JSON ordering, and appends a JSON line entry to the audit log describing the change (timestamp, action, device, old/new keys).

Query the latest head:
```bash
curl http://localhost:8080/head | jq
```

---

## Testing
```bash
go test ./...
```

---

## Getting Started
Implementation details evolve, but the general workflow remains:
1. **Review the relevant plan** in [`/plans`](plans) for the milestone you are targeting.
2. **Build or run Go binaries** from the [`cmd/skychain`](cmd/skychain) entry point with configuration flags suited to your role (validator, gateway, etc.).
3. **Exercise APIs or clients** to submit sample events and inspect resulting blocks.

Refer to the plan documents and in-code documentation for concrete commands and configuration options as they change over time.
