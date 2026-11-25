# AMR Fleet Telemetry Gateway

High-throughput Autonomous Mobile Robot (AMR) telemetry ingestion, health monitoring, and remote command dispatch engine.

## Problem
In commercial robot operations (autonomous floor cleaners, shelf-scanning mobile robots, warehouse transport AMRs), cloud gateways must reliably ingest concurrent high-frequency robot telemetry (pose, battery state of charge, hardware sensor faults, operational status) and dispatch mission-critical control commands with atomic sequence tracking and lease acknowledgments.

## Architecture & Capabilities
- **Concurrent Telemetry Ingestion**: Thread-safe ingestion pipeline buffering incoming robot state pings.
- **Heartbeat & Liveness Tracker**: Automated heartbeat expiration detecting disconnected or stalled AMRs.
- **Safety & Anomaly Evaluator**: Immediate automated safety triggers on critical low battery, emergency stops, or severe sensor divergence.
- **Command Dispatch Mesh**: Sequenced, idempotent command queuing (e.g. `RETURN_TO_DOCK`, `PAUSE_MISSION`, `RESUME_MISSION`, `ESTOP`) with ACK validation.

## Running Tests
```bash
go test -v -race ./...
```
