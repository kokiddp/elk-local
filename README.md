# ELK-Local

ELK-Local is a local development tool for PHP applications with a CLI-first workflow and an optional web UI. It is designed to spin up disposable or durable development environments backed by Docker containers, starting with WordPress as a first-class use case without being locked to WordPress.

The project targets the same problem space as XAMPP, MAMP, and Local, but with a stronger emphasis on reproducible environments, fast PHP/runtime switching, and automation-friendly workflows.

## Goals

- Create and manage full local development environments on demand.
- Support WordPress out of the box while remaining framework-agnostic.
- Allow fast switching between PHP versions, web servers, and databases.
- Expose the same capabilities through a CLI and an optional local web UI.
- Make backups, exports, imports, and restores first-class operations.

## Initial Technology Choices

### Core Runtime

- Go 1.20+ for the main application.
- Cobra for the CLI command structure.
- Docker Compose v2 as the initial container orchestration layer.

Why Go:

- Simple single-binary distribution for Linux, macOS, and Windows.
- Good fit for shelling out to Docker, managing files, and exposing a local HTTP API.
- Small runtime footprint for a tool that should feel closer to a native utility than a web app.

### Optional Web UI

- Vue 3
- TypeScript
- Vite
- SCSS
- Bootstrap 5.3

Why Vue:

- Lower ceremony for a local control panel.
- Easy to keep optional and decoupled from the CLI.
- Good ergonomics for a small desktop-like dashboard.

### Environment Model

- Docker containers for web and database services at minimum.
- Optional Adminer, Mailpit, and Xdebug support per environment.
- Compose-generated environment definitions for each project.
- PHP, web server, database engine, and application database credentials encoded in a manifest.
- Local backups and import/export flows built around Docker volumes and database dump tooling.

## Planned Capabilities

- Create environments from presets like `wordpress`, `laravel`, `symfony`, or `custom`.
- Bootstrap the selected application when a preset implies one, including app version selection during create.
- Switch between PHP versions such as 7.4 through 8.4.
- Switch between web servers such as Apache and Nginx.
- Switch between MySQL and MariaDB variants.
- Optionally add Adminer, Mailpit, and Xdebug per environment.
- Start, stop, rebuild, clone, backup, export, import, and restore environments.
- Run a loopback-only local API daemon that the CLI and optional web UI can rely on.

## Repository Layout

```text
cmd/elk-local/         CLI entrypoint
internal/cli/          Cobra commands
internal/version/      Build metadata
docs/                  Architecture and decisions
webui/                 Optional Vue application
```

## Current Status

This repository now includes the initial environment manifest model, built-in presets, a `create` workflow that generates a managed Compose-based environment scaffold and installs the selected preset application, lifecycle commands for `start`, `stop`, `status`, and `destroy`, first-phase backup/export/import/restore commands, and a loopback local API with a usable Vue dashboard for exercising the core environment flows. Create and switch also sync application database settings into supported project config files.

Application installation currently works like this:

- `wordpress` downloads and installs WordPress into the project root.
- `laravel` runs `composer create-project laravel/laravel` in the project root.
- `symfony` runs `composer create-project symfony/skeleton` in the project root.
- `custom` does not install an application.

Version selection currently works like this:

- `--app-version` applies during `create` for presets that install an application.
- WordPress accepts `latest`, `nightly`, stable versions like `6.9.4`, and prereleases like `7.0-beta6` or `7.0-RC2`.
- Laravel and Symfony accept Composer version constraints such as `^12.0`, `11.*`, `^7.0`, or `6.4.*`.

Application config syncing currently works like this:

- WordPress updates `wp-config.php`.
- Laravel updates `.env`.
- Symfony updates `.env.local`.
- Generic PHP presets update `.env.local` unless `.env` already exists.

## Getting Started

### Prerequisites

- Go 1.20+
- Node.js 20+
- npm 10+
- Docker Engine with Docker Compose v2

### Installer

Release packages include a top-level `install.sh` for Linux user installs:

```bash
./install.sh
```

The installer copies ELK-Local into `${HOME}/.elk-local`, adds `${HOME}/.elk-local/bin` to your shell `PATH`, writes `${HOME}/.elk-local/config.yaml`, asks whether the loopback daemon should auto-start on login, and lets you customize the default environment work-tree root, the default backups root, and the loopback web UI port.

By default the installed layout is:

- Program files: `${HOME}/.elk-local`
- Environment work trees: `${HOME}/elk-local/environments/<name>`
- Managed backups: `${HOME}/elk-local/backups/<name>`
- Manifest registry: `${HOME}/.elk-local/environments/<name>`
- Web UI and daemon port: `4173`

To unregister and uninstall the program files while preserving environments and backups, run:

```bash
./install.sh --uninstall
```

### CLI

```bash
go mod tidy
go run ./cmd/elk-local version
go run ./cmd/elk-local presets
go run ./cmd/elk-local create my-wordpress-site --preset wordpress --app-version 6.9.4 --db-name wp_local --db-user wp --db-password secret --adminer --mailpit --xdebug
go run ./cmd/elk-local create my-wordpress-beta --preset wordpress --app-version 7.0-RC2
go run ./cmd/elk-local create my-laravel-site --preset laravel --app-version ^12.0
go run ./cmd/elk-local switch my-wordpress-site --php 7.4 --web-server nginx --db-password newer-secret --enable-xdebug
go run ./cmd/elk-local start my-wordpress-site
go run ./cmd/elk-local backup my-wordpress-site --include-project-files
go run ./cmd/elk-local export my-wordpress-site --output /tmp/my-wordpress-site.tar.gz --include-project-files
go run ./cmd/elk-local import my-wordpress-site /tmp/my-wordpress-site.tar.gz
go run ./cmd/elk-local restore my-wordpress-site my-wordpress-site-20260328-123456Z.tar.gz --project-files --force
go run ./cmd/elk-local daemon start --host 127.0.0.1 --port 4173
go run ./cmd/elk-local daemon status
go run ./cmd/elk-local status my-wordpress-site
go run ./cmd/elk-local stop my-wordpress-site
go run ./cmd/elk-local daemon stop
go run ./cmd/elk-local serve --host 127.0.0.1 --port 4173
```

When ELK-Local is installed with `install.sh`, commands that rely on the manifest registry or daemon state default to your home registry at `${HOME}/.elk-local`. `create NAME` also defaults the application/project root to `${HOME}/elk-local/environments/<name>` unless you pass `--project-root` explicitly. If you omit `--port` for `elk-local daemon start` or `elk-local serve`, the installed `webuiPort` setting is used.

The `create` command writes generated artifacts under `.elk-local/environments/<name>` by default:

- `environment.yaml`
- `compose.yaml`
- `nginx/default.conf` for Nginx-backed presets
- `adminer/` when Adminer is enabled
- `xdebug/` when Xdebug is enabled
- `backups/`

For presets that install an application, `create` also populates the project root itself. The create flow expects either an empty project directory or an already matching application tree.

Managed backup archives are written to `backups/` by `elk-local backup`. `elk-local export` writes the same archive format to a path you choose.

The first backup archive format includes:

- `metadata.json` with archive metadata.
- `manifest.yaml` with a copy of the environment manifest.
- `database.sql` with a database dump restored through the `db` service.
- `project/` when `--include-project-files` is used.

Current restore behavior is intentionally conservative:

- `restore` always requires `--force` because it replaces the target database contents.
- `restore --project-files` overwrites matching files from the archive but does not delete extra files already present in the project root.

The daemon stores its runtime metadata and logs under `.elk-local/daemon/` in the project root:

- `state.json`
- `daemon.log`

### Web UI

```bash
cd webui
npm install
npm run dev
```

For the local dashboard, start the background API daemon once:

```bash
go run ./cmd/elk-local daemon start --host 127.0.0.1 --port 4173
```

The Vite dev server proxies `/api` to the loopback Go daemon. If you build the web UI first, the daemon will also serve the built dashboard directly from `webui/dist`.

The dashboard create form follows the same project-root default as the CLI. On installed setups it creates the application under `${HOME}/elk-local/environments/<name>` unless you enter a custom project root override.

While a create request is running, the dashboard now keeps an explicit in-flight status panel visible in the Create tab instead of only changing the button label. Runtime action notices in the Environments tab also stay concise and no longer echo the full raw Compose output into the success banner.

Generated WordPress stacks now keep the local URL and container runtime aligned: `wp-config.php` pins `WP_HOME` and `WP_SITEURL` to the local `127.0.0.1:<port>` address, Apache listens on that same port inside the container, and filesystem updates use the direct method instead of falling back to FTP for the default local setup.

The Environments tab now uses a simple list-and-detail layout instead of a wall of cards. Selecting a stack opens a single detail pane with its state, runtime controls, links, and container inventory, and a destroyed environment can now be deleted from the dashboard once no containers are still reported.

ELK-Local now verifies host TCP ports before it writes them into a stack manifest. Auto-assigned HTTP, database, Adminer, Mailpit, and Xdebug ports skip blocked host ports instead of failing later at `docker compose up`, while explicit port overrides still fail fast if the requested port is unavailable.

When Xdebug is enabled for a stack, ELK-Local writes `.vscode/launch.json` into the app root with ready-to-run PHP debug listeners that follow the stack's actual Xdebug client port and include the `/var/www/html` to `${workspaceFolder}` path mapping needed for container breakpoints to bind in VS Code.

WordPress preset images now install WP-CLI inside the PHP container. Installed ELK-Local setups also place `wp`, `mysql`, `mariadb`, `mysqldump`, and `mariadb-dump` wrappers on your PATH; when you run them from inside an ELK-managed project root they proxy into the matching containers, which keeps Wordmove workflows usable from a terminal opened in the webroot.

The Environments detail pane now includes an `Open in VS Code` action. On WSL, ELK-Local prefers the WSL `code` command so the project opens in the remote WSL workspace when that integration is available.

Use `go run ./cmd/elk-local daemon status` to verify the control plane is up, `go run ./cmd/elk-local daemon stop` to shut it down, and `go run ./cmd/elk-local serve` only when you explicitly want a foreground/debug session.

## Testing

For the current verification matrix, run:

```bash
make cli-test
make ui-build
make smoke-daemon
make test-all
```

`make test-all` runs the Go test suite, validates the Vue production build, and executes a compiled-binary daemon smoke test that starts, checks, and stops the background control plane.

## Automation

GitHub Actions now covers two delivery paths:

- `Test On Master` runs on every push to `master` and executes the existing local validation matrix with `make test-all`.
- `Release` runs on `v*` tags or through manual dispatch, builds the Vue production assets, cross-compiles the CLI, stamps version metadata into the binary, and uploads distribution archives plus `sha256sum.txt`.

For a local release dry run, use:

```bash
make dist
```

That command writes versioned `.tar.gz` and `.zip` archives under `dist/releases/`. Each package includes the CLI binary, `webui/dist`, `install.sh`, and the top-level README and license files so the extracted directory is ready to install or run.

When you are validating installed behavior after source edits, use the full installed workflow instead of only running from source: uninstall the current copy with `./install.sh --uninstall`, regenerate artifacts with `make dist`, and then install again from the updated payload. This keeps installer behavior, packaged assets, and the installed daemon path in sync with the code you just changed.

## Roadmap

1. Add rebuild and clone commands on top of the lifecycle layer.
2. Add backup inventory, pruning, and richer restore inspection.
3. Add richer environment inspection and health details.
4. Expand the dashboard with switch and backup operations.

## Documentation

- [Architecture](./docs/architecture.md)
- [Initial ADR](./docs/adr/0001-initial-architecture.md)
- [Daemon Control Plane ADR](./docs/adr/0002-daemon-control-plane.md)
- [Manifest](./docs/manifest.md)
- [Backups](./docs/backups.md)
- [Testing](./docs/testing.md)
- [Progress](./docs/progress.md)
- [Contributing](./CONTRIBUTING.md)
- [TODO](./TODO.md)
- [Agent Guidance](./AGENTS.md)
- [License](./LICENSE.md)
