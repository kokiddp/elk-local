# ADR 0002: Daemonized Local Control Plane

## Status

Accepted

## Context

ELK-Local already exposes a loopback HTTP API for the optional web UI, but the initial workflow required a foreground `serve` process. That meant the dashboard and API disappeared as soon as the serving terminal closed, and there was no stable lifecycle for a long-running local control plane.

The product shape remains CLI-first, but the CLI and web UI both benefit from a daemon that stays reachable across terminals and browser sessions.

## Decision

We will add a background daemon lifecycle for the local API.

- `elk-local daemon start|stop|status|restart` manages the background process.
- `elk-local serve` remains as the foreground mode for debugging and direct development.
- The API server exposes loopback-only daemon status and graceful shutdown endpoints used by the daemon manager.
- The daemon persists runtime metadata and logs under `.elk-local/daemon/` inside the selected project root.

## Rationale

- Keeps the CLI primary while making the web UI consistently available.
- Avoids introducing a second service implementation separate from the existing API server.
- Uses simple file-backed state and loopback HTTP control rather than platform-specific service managers.
- Preserves repository portability across Docker Engine environments without requiring Docker Desktop or OS-native service installation.

## Consequences

### Positive

- The control plane can stay available across terminals.
- CLI users gain explicit background lifecycle commands.
- The same API server implementation serves both foreground and daemon workflows.

### Negative

- We now own daemon state cleanup and stale-process handling.
- Native OS service integration is still out of scope.
- Loopback shutdown endpoints must remain constrained to local use and documented clearly.

## Follow-Up Decisions

- Whether CLI environment actions should optionally route through the daemon in the future.
- Whether daemon logs should gain rotation or structured formatting.