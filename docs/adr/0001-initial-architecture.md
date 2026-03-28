# ADR 0001: Initial Architecture And Technology Choices

## Status

Accepted

## Context

ELK-Local needs to manage local PHP development environments with a strong CLI experience and an optional web UI. It must support multiple PHP versions, interchangeable web servers, and database engines while remaining easy to install and portable across developer machines.

## Decision

We will use:

- Go for the main application runtime.
- Cobra for the CLI surface.
- Docker Compose v2 for initial environment orchestration.
- Vue 3 with TypeScript and Vite for the optional web UI.

The Go application will own both CLI execution and a local HTTP API used by the web UI.

## Rationale

### Go

- Produces a simple distributable binary.
- Works well for orchestration-heavy tooling.
- Avoids requiring users to install a scripting runtime just to use the CLI.

### Docker Compose v2

- Matches how developers already think about local service stacks.
- Reduces the amount of orchestration code needed in the first milestone.
- Keeps future migration paths open if a more specialized runtime layer is needed later.

### Vue

- Good fit for an optional local UI with modest complexity.
- Faster to keep lean than heavier frontend frameworks.

## Consequences

### Positive

- Clear CLI-first architecture.
- Portable runtime.
- Web UI can evolve without replacing the CLI.

### Negative

- Docker Compose generation and version management must be designed carefully.
- We need a clear contract between CLI/service code and the web UI.

## Follow-Up Decisions

- Environment manifest schema
- Preset model
- Backup artifact format
- API contract between Go service and Vue web UI
