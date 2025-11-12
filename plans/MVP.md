# MVP Specification — SkyChain

**Goal:** Implement the minimal functioning blockchain capable of recording IoT events in a single validator setup.

## Features
1. **Core Ledger Engine**
   - Append-only chain of blocks, each referencing previous block by hash.
   - Single validator signs each block on a fixed interval (default 10s).

2. **IoT Event Transactions**
   - Accept simple events via HTTP: `{device_id, nonce, ts, payload}`.
   - Store events in an in-memory queue until next block tick.

3. **Block Persistence**
   - JSON file storage (`data/chain.json` or `data/chain.jsonl`).
   - Validate block continuity on startup.

4. **API Endpoints**
   - `POST /event` — submit IoT event
   - `GET /chain` — view full chain
   - `GET /head` — current block metadata
   - `GET /health` — node status

5. **Demo Workflow**
   - Run validator.
   - Submit events.
   - Verify new blocks are appended.

## Deliverables
- Executable Go binary (`skychain`)
- Basic unit tests for block linkage and JSON reload
- README curl examples
