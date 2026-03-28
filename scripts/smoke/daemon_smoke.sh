#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
tmp_root="$(mktemp -d)"
port="${ELK_SMOKE_PORT:-41733}"
binary_path="${ELK_SMOKE_BINARY:-$repo_root/bin/elk-local}"

cleanup() {
  if [[ -x "$binary_path" ]]; then
    "$binary_path" daemon stop --project-root "$tmp_root" >/dev/null 2>&1 || true
  fi
  rm -rf "$tmp_root"
}

trap cleanup EXIT

cd "$repo_root"

go build -o "$binary_path" ./cmd/elk-local

"$binary_path" daemon start \
  --project-root "$tmp_root" \
  --host 127.0.0.1 \
  --port "$port" \
  --webui-dist "$repo_root/webui/dist" >/dev/null

curl -fsS "http://127.0.0.1:${port}/api/daemon/status" >/dev/null

status_output="$($binary_path daemon status --project-root "$tmp_root")"
case "$status_output" in
  *"ELK-Local daemon is running."*) ;;
  *)
    printf 'unexpected daemon status output:\n%s\n' "$status_output" >&2
    exit 1
    ;;
esac

"$binary_path" daemon stop --project-root "$tmp_root" >/dev/null

if curl -fsS "http://127.0.0.1:${port}/api/daemon/status" >/dev/null 2>&1; then
  printf 'daemon remained reachable after stop\n' >&2
  exit 1
fi

printf 'daemon smoke test passed on port %s\n' "$port"