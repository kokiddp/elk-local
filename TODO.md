# TODO

## Near Term

- [ ] Auto-start the daemon for selected CLI workflows that depend on the loopback control plane.
- [ ] Add rebuild and clone commands on top of the lifecycle layer.
- [ ] Add backup inventory pruning and richer archive inspection.
- [ ] Add dashboard support for runtime switching workflows.

## Testing

- [ ] Add browser automation for the Vue dashboard.
- [ ] Add container-backed integration tests that boot full PHP stacks.
- [ ] Add CI wiring for `make test-all`.

## Operations

- [ ] Add daemon log rotation or structured logging.
- [ ] Decide whether more CLI operations should route through the daemon by default.