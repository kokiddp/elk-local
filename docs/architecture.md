# Architecture Overview

## Product Shape

ELK-Local has two interfaces over one core capability set:

- A CLI for day-to-day developer operations and automation.
- An optional web UI for visual environment management.

The CLI is the primary interface. The web UI is a local control surface, not a hosted product.

## Core Concepts

### Environment

An environment is a named local stack for one project. It owns:

- A manifest describing PHP, web server, database, ports, volumes, and project type.
- Generated Docker Compose configuration.
- Runtime data such as persistent volumes and backups.

### Preset

A preset captures common stack defaults for targets like WordPress, Laravel, Symfony, or a fully custom PHP app.

When a preset maps to a known application, create also bootstraps that application into the project root and records the installed app metadata in the manifest.

### Local API

The same Go application that powers the CLI will also expose a loopback-only HTTP API for the optional web UI.

The first implementation of that API now exists and supports:

- listing managed environments
- creating environments from presets
- lifecycle actions for start, stop, and destroy
- serving the built Vue dashboard from the same loopback process when `webui/dist` is present

Backup and restore workflows currently live in the CLI only. API and dashboard support for those operations is follow-up work.

## Initial Components

### Go CLI and Service Layer

- Command execution
- Manifest management
- Docker and Compose orchestration
- Preset-aware application installation during create
- Lifecycle operations for start, stop, status, and destroy
- Backup and restore workflows
- Local API serving

### Vue Web UI

- Environment list and status screens
- Create environment flow
- Start, stop, and destroy actions
- Direct links to app, Adminer, and Mailpit endpoints
- Runtime switch controls for PHP, web server, and database as follow-up work
- Backup inventory and restore actions as follow-up work

## Storage Strategy

Planned initial storage model:

- Human-readable YAML manifests for environments
- Generated Compose files derived from manifests
- Local metadata directory for cache and runtime state
- File-based backup artifacts plus database dumps

The first implementation of this model now exists as a generated environment directory containing:

- `environment.yaml` as the source-of-truth manifest
- `compose.yaml` as the generated runtime definition
- `nginx/default.conf` when the selected web server is Nginx
- `backups/` as the managed backup artifact directory

For presets that install an application, the project root is also treated as managed bootstrap output during create. The manifest records the installed application name and requested or detected version so the CLI and dashboard can surface it consistently.

The current backup archive format is a portable `.tar.gz` file containing:

- `metadata.json`
- `manifest.yaml`
- `database.sql`
- `project/` when the user explicitly includes project files

Current restore behavior replaces the target database contents and optionally merges project files back into the project root. Extra project files that are not present in the archive are left in place.

## Non-Goals For The First Milestone

- Cloud sync
- Multi-user orchestration
- Remote environment hosting
- Windows service integration beyond standard local execution
