GO ?= go
NPM ?= npm

.PHONY: help
help:
	@printf "Targets:\n"
	@printf "  make cli-build    Build the CLI binary\n"
	@printf "  make cli-run      Run the CLI locally\n"
	@printf "  make cli-test     Run Go tests\n"
	@printf "  make dist         Build release archives under dist/releases\n"
	@printf "  make smoke-daemon Run the daemon smoke test\n"
	@printf "  make test-all     Run Go tests, web UI build, and daemon smoke test\n"
	@printf "  make ui-install   Install web UI dependencies\n"
	@printf "  make ui-dev       Start the Vue development server\n"
	@printf "  make ui-build     Build the Vue application\n"

.PHONY: cli-build
cli-build:
	$(GO) build -o bin/elk-local ./cmd/elk-local

.PHONY: cli-run
cli-run:
	$(GO) run ./cmd/elk-local

.PHONY: cli-test
cli-test:
	$(GO) test ./...

.PHONY: dist
dist:
	./scripts/release/build_dist.sh

.PHONY: smoke-daemon
smoke-daemon:
	./scripts/smoke/daemon_smoke.sh

.PHONY: test-all
test-all: cli-test ui-build smoke-daemon

.PHONY: ui-install
ui-install:
	cd webui && $(NPM) install

.PHONY: ui-dev
ui-dev:
	cd webui && $(NPM) run dev

.PHONY: ui-build
ui-build:
	cd webui && $(NPM) run build
