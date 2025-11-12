# SkyChain
A lightweight proof-of-authority (PoA) blockchain designed for IoT sensor data.

SkyChain provides an append-only, verifiable ledger where IoT device readings are recorded into signed, immutable blocks on a fixed schedule. It demonstrates a scalable and modular design for distributed, fault-tolerant data collection across IoT networks.

---

## Overview
SkyChain maintains a time-sequenced blockchain for recording IoT events. Each block:

- references the previous block by hash (ensuring immutability);
- stores batches of signed IoT events;
- is timestamped and signed by authorised validators;
- can be verified independently by any node or gateway.

Gateways act as light clients: they collect device data, verify signatures, and submit events to validators for inclusion in the next block.

---

## Core Concepts

| Component | Role |
|------------|------|
| **Devices** | Generate signed readings (`device_id`, `nonce`, `timestamp`, `payload`). |
| **Gateways** | Buffer and verify readings, forward to validators, resync after outages. |
| **Validators** | Produce and sign blocks at fixed intervals, maintain the chain’s integrity. |
| **ChainStore** | Pluggable backend for block storage (JSON file for MVP, scalable later). |

Consensus is deterministic and schedule-based: one validator proposes a block every 10 seconds, others verify and countersign. No mining or tokens — just verifiable trust in data.

---

## MVP Features

- Single validator (PoA-Lite)
- JSON file storage (`data/chain.json`)
- HTTP API:  
  - `POST /event` — submit IoT reading  
  - `GET /chain` — view full blockchain  
  - `GET /head` — view latest block metadata  
  - `GET /health` — node status  
- Configurable block interval (default 10 s)
- Simple demo client (`skyctl`) for posting test events

---

## Stretch Goals

- ED25519 device signatures + whitelist  
- Multi-validator PoA consensus (quorum)  
- Merkle roots + inclusion proofs  
- Validator/device registries and key rotation  
- Web dashboard showing block height & recent events  
- External timestamp anchoring (optional)

---

## Architecture Overview

Device → Gateway (light node) → Validator Cluster (PoA) → Archive / Analytics

Validators store the canonical chain; gateways keep recent headers and proofs; devices remain stateless.

---

## Running SkyChain

### Start validator
```bash
go run ./cmd/skychain --role=validator --interval=10s --data=data/chain.json
```

### Send test event
```bash
curl -X POST localhost:8080/event   -H 'Content-Type: application/json'   -d '{"device_id":"sensor-1","nonce":1,"ts":1710000000,"payload":"22.5°C"}'
```

### Inspect chain
```bash
curl localhost:8080/chain | jq .
```

---

## Development Plan

All build steps are documented in `/plans/`:
- `MVP.md` — core single-validator ledger  
- `STRETCH1.md` — signatures and device registry  
- `STRETCH2.md` — multi-validator PoA, dashboard, metrics

Each phase is self-contained and can be implemented independently.

---

## License
MIT © 2025 David Maddison
