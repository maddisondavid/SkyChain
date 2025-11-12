# Goal 10 — Web Dashboard for Observability

**Goal:** Deliver a simple UI surface to inspect validator health and recent blocks.

## Features
1. Serve `/ui` route displaying head height, validator set, and recent block summaries.
2. Include live metrics snippets sourced from the `/metrics` endpoint.
3. Provide interactive form to submit test events against the public API.

## Deliverables
- Static asset bundle or templated page served by the binary
- API client in the UI for live data refresh
- Tests covering handler wiring and basic rendering
