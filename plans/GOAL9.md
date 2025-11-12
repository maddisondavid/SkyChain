# Goal 9 — Metrics Collection and Exposure

**Goal:** Provide observability into chain health through structured metrics.

## Features
1. Instrument validator runtime with Prometheus counters, gauges, and summaries.
2. Track block production time, pending events, and signature success rates.
3. Serve `/metrics` endpoint compatible with Prometheus scrapers.

## Deliverables
- Metrics registry integrated with consensus and queue components
- HTTP handler exposing metrics in Prometheus text format
- Tests verifying metric registration and sample emission
