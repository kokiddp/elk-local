# Contributing

## Working Principles

- Keep ELK-Local CLI-first. The web UI is optional and should never become the only way to manage environments.
- Prefer small, composable changes over broad speculative abstractions.
- Optimize for local developer ergonomics, reproducibility, and predictable upgrades.
- Treat WordPress as a first-class target without hard-coding WordPress assumptions into the whole platform.

## Development Setup

### CLI

```bash
go mod tidy
go test ./...
./scripts/smoke/daemon_smoke.sh
go run ./cmd/elk-local version
go run ./cmd/elk-local daemon status
```

### Web UI

```bash
cd webui
npm install
npm run build
```

### Full Local Validation

```bash
make test-all
```

This currently covers Go tests, the Vue production build, and the daemon smoke test.

### Release Packaging Dry Run

```bash
make dist
```

This mirrors the GitHub `Release` workflow by rebuilding `webui/dist`, cross-compiling the CLI for the supported desktop targets, and writing ready-to-distribute archives plus checksums under `dist/releases/`. The release payload now also includes `install.sh`, which installs into `${HOME}/.elk-local` and writes the installed defaults file used by the CLI, including the configured environment roots, backups root, daemon auto-start preference, and default web UI port.

When a change needs installed-path validation, do not stop at local source runs. After making edits, run the installed loop in order: `./install.sh --uninstall`, `make dist`, and then install the freshly generated payload again. This is the required path for checking installer behavior, packaged web UI assets, and daemon/runtime behavior as actually shipped.

### Local API Workflow

- Use `elk-local daemon start` for the normal always-on local control plane.
- Use `elk-local daemon stop` and `elk-local daemon status` for lifecycle checks.
- Use `elk-local serve` only for foreground debugging or development sessions that should stay attached to one terminal.

## Project Conventions

- Put CLI command wiring in `internal/cli`.
- Keep domain logic out of command handlers as the project grows.
- Treat Docker Compose files as generated artifacts, not hand-edited source.
- Prefer explicit environment manifests over hidden mutable state.
- Keep preset-driven application installation deterministic. If a preset installs an app, record the installed app name and version in the manifest.
- Avoid letting tests reach the network or invoke real Composer bootstraps unless the test is explicitly an integration test.
- Keep application config syncing explicit and predictable. If ELK-Local updates `.env` or `wp-config.php`, limit the change to environment-owned connection settings.
- Treat backup and restore flows as safety-critical. Destructive restore behavior must keep an explicit confirmation path.
- Keep managed backup artifacts under each environment's `backups/` directory unless the user explicitly exports elsewhere.
- Keep daemon state loopback-only and store its file-backed runtime metadata under `.elk-local/daemon/`.
- Keep dashboard create defaults aligned with the CLI. A blank create project root should resolve to the installed `environmentsDir/<name>` when installed config is present, while still honoring explicit overrides.
- Keep dashboard progress and notice copy operational and terse. Long-running create requests should show an obvious in-flight state, and lifecycle success notices should summarize the outcome rather than dumping raw command output.
- Keep dashboard backup controls local and explicit. Managed archives may be downloaded, opened in the host file explorer, restored, or deleted from the UI, but destructive actions must stay clearly named and require deliberate user input.
- Keep the Environments tab operationally clear. Show state immediately in the list, open one detail pane at a time, and gate permanent delete behind a prior destroy so active containers are not removed by surprise.
- Keep host-port assignment preflighted. Auto-assigned HTTP, database, Adminer, Mailpit, and Xdebug ports should skip blocked host ports before the manifest is written, while explicit overrides must fail clearly when the requested port is unavailable.
- Keep Xdebug onboarding zero-touch for VS Code users. When Xdebug is enabled, generate `.vscode/launch.json` in the app root with the managed listener configs, make those listeners follow the stack's actual Xdebug client port, include the `/var/www/html` to `${workspaceFolder}` path mapping, and remove only the ELK-managed entries when Xdebug is disabled so unrelated user launch configs survive.
- Keep the dashboard editor handoff practical. The `Open in VS Code` action should open the project root directly, and WSL installs should prefer the WSL VS Code command when it exists.
- Keep Wordmove-friendly local tooling available for WordPress environments. WordPress PHP images should include WP-CLI, and the installed PATH wrappers for `wp` plus MySQL client commands should proxy into the matching ELK containers when invoked from a managed project root while still falling back to host binaries elsewhere.
- Keep generated WordPress loopback behavior self-consistent. If ELK-Local advertises `127.0.0.1:<port>`, the Apache container and `wp-config.php` should agree on that same port, local file writes should not degrade to FTP, and any custom PHP entrypoint must preserve the base runtime command such as `apache2-foreground` or `php-fpm`.
- Document significant architectural choices in `docs/adr/`.
- Keep test workflow documentation current in `docs/testing.md`, and log project status in `docs/progress.md` plus `TODO.md` when priorities change.

## Pull Request Expectations

- Describe the user-visible change and the developer-facing implementation impact.
- Update docs when behavior, architecture, or workflows change. If CI or release automation changes, keep `README.md` and this file aligned with the new entrypoints.
- Include tests for non-trivial logic where practical.
- If you touched code that will be exercised through the installed product, finish by validating the installed workflow against a freshly regenerated artifact set rather than a stale local install.
- Call out any known gaps, follow-up work, or compatibility constraints.

## Early Priorities

1. Runtime switching for PHP, web server, and database.
2. Lifecycle commands beyond create.
3. Backup inventory and richer restore flows.
4. Loopback local API for the web UI.
5. Vue web UI integration against the local API.
