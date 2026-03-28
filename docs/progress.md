# Project Progress

## Completed Recently

- Added preset-aware application installation for WordPress, Laravel, and Symfony.
- Added generated manifest, Compose, Adminer, Mailpit, and Xdebug environment artifacts.
- Added backup, export, import, and restore flows plus API and dashboard support.
- Refactored the Vue dashboard into tabbed components backed by the same loopback API.
- Added a daemonized local control plane so the CLI and web UI can rely on an always-on loopback service.
- Expanded automated verification with daemon package tests, CLI command tests, API tests, environment tests, and a compiled-binary smoke script.

## Verified Workflows

- `create`, `switch`, `start`, `stop`, `status`, and `destroy`
- `backup`, `export`, `import`, and `restore`
- `daemon start`, `daemon status`, `daemon stop`, and `daemon restart`
- Vue production build against the current API contract
- loopback daemon smoke lifecycle through the compiled CLI binary

## Current State

- CLI-first workflow is intact.
- Web UI remains optional and local-only.
- The daemon is now the default long-running control-plane workflow.
- Test and smoke commands are documented in [docs/testing.md](./testing.md).

## Next Milestones

1. Auto-start the daemon from selected CLI flows when the control plane is missing.
2. Add richer environment inspection and health data.
3. Add backup pruning and archive inspection details.
4. Add browser-level dashboard tests and fuller container-backed integration coverage.