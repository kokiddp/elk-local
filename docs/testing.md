# Testing

ELK-Local currently uses three verification layers:

- Go unit and package-level integration tests under `internal/**`.
- Web UI typecheck and production build verification through Vite.
- Shell smoke tests that exercise the compiled CLI and loopback daemon end to end.

## Fast Commands

```bash
make cli-test
make ui-build
make smoke-daemon
make test-all
```

Equivalent direct commands:

```bash
go test ./...
cd webui && npm run build
./scripts/smoke/daemon_smoke.sh
```

## Current Go Coverage Areas

- Environment creation, update, manifest validation, and generated artifact rendering.
- Backup archive create, import, restore, and database client script selection.
- API environment listing, creation, lifecycle actions, backup inventory, and daemon shutdown.
- Daemon state persistence, loopback status and shutdown helpers, stale-state cleanup, and CLI output paths.

## Smoke Coverage

The daemon smoke test validates the real compiled binary rather than package internals.

It currently verifies:

- background daemon startup
- loopback daemon status reachability
- CLI status reporting against persisted daemon state
- graceful daemon stop
- loss of reachability after stop

Override the port or binary path when needed:

```bash
ELK_SMOKE_PORT=42733 ./scripts/smoke/daemon_smoke.sh
ELK_SMOKE_BINARY=./bin/elk-local ./scripts/smoke/daemon_smoke.sh
```

## Known Gaps

- No browser automation for the Vue dashboard yet.
- No container-backed integration suite that boots full PHP stacks in CI yet.
- No performance or soak testing for long-lived daemon sessions yet.

Those gaps are tracked in [TODO.md](../TODO.md).