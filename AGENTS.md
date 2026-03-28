# AGENTS.md

## Purpose

This repository contains ELK-Local, a CLI-first local development tool for PHP applications with an optional Vue web UI. Agents working in this repository should preserve that product shape.

## Repository Rules

- Keep the CLI usable on its own. Do not make the web UI mandatory for core workflows.
- Prefer Go for orchestration, manifest handling, Docker integration, and the local API.
- Keep the web UI in `webui/` and treat it as a consumer of the same core capabilities.
- Preserve framework-agnostic behavior even when adding WordPress-focused shortcuts.
- Do not assume Docker Desktop specifically; target Docker Engine plus Compose v2.

## Implementation Guidance

- Add new CLI commands under `internal/cli`.
- Keep command handlers thin; move reusable logic into internal packages.
- Favor explicit manifests and generated files over magic defaults.
- When introducing a new architectural pattern, add or update an ADR in `docs/adr`.
- If you change workflows, update `README.md` and `CONTRIBUTING.md` in the same change.

## Safety Constraints

- Avoid destructive operations against user environments without explicit confirmation paths.
- Treat backup and restore features as safety-critical.
- Make container naming, network naming, and filesystem paths deterministic.
- Prefer additive migrations for config formats.

## UI Guidance

- The web UI should remain optional and local-only.
- Keep the UI pragmatic and operational rather than marketing-heavy.
- Prefer direct visibility into environment state, service health, versions, and actions.
