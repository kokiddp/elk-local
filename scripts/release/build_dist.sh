#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
out_dir="${OUT_DIR:-$repo_root/dist/releases}"
version="${VERSION:-}"
commit="${COMMIT:-}"
build_date="${DATE:-}"
targets="${TARGETS:-linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64}"

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    printf 'missing required command: %s\n' "$1" >&2
    exit 1
  fi
}

resolve_version() {
  if [[ -n "$version" ]]; then
    return
  fi

  if [[ -n "${GITHUB_REF_NAME:-}" ]]; then
    version="$GITHUB_REF_NAME"
    return
  fi

  version="$(git -C "$repo_root" describe --tags --always --dirty 2>/dev/null || true)"
  if [[ -z "$version" ]]; then
    version="0.0.0-dev"
  fi
}

resolve_commit() {
  if [[ -n "$commit" ]]; then
    return
  fi

  commit="$(git -C "$repo_root" rev-parse --short HEAD 2>/dev/null || true)"
  if [[ -z "$commit" ]]; then
    commit="unknown"
  fi
}

resolve_build_date() {
  if [[ -n "$build_date" ]]; then
    return
  fi

  build_date="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
}

build_webui() {
  printf '==> Installing web UI dependencies\n'
  (
    cd "$repo_root/webui"
    npm ci
    npm run build
  )
}

ldflags() {
  printf '%s' "-s -w -X elk-local/internal/version.Version=$version -X elk-local/internal/version.Commit=$commit -X elk-local/internal/version.Date=$build_date"
}

archive_extension() {
  local os="$1"
  if [[ "$os" == "windows" ]]; then
    printf 'zip'
    return
  fi

  printf 'tar.gz'
}

archive_basename() {
  local os="$1"
  local arch="$2"
  printf 'elk-local_%s_%s_%s' "$version" "$os" "$arch"
}

package_target() {
  local os="$1"
  local arch="$2"
  local base_name
  local stage_dir
  local binary_name="elk-local"
  local archive_name

  base_name="$(archive_basename "$os" "$arch")"
  stage_dir="$out_dir/$base_name"
  archive_name="$base_name.$(archive_extension "$os")"

  if [[ "$os" == "windows" ]]; then
    binary_name="elk-local.exe"
  fi

  printf '==> Building %s/%s\n' "$os" "$arch"
  rm -rf "$stage_dir"
  mkdir -p "$stage_dir/webui"

  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
    go build -trimpath -ldflags "$(ldflags)" -o "$stage_dir/$binary_name" ./cmd/elk-local

  cp -R "$repo_root/webui/dist" "$stage_dir/webui/dist"
  cp "$repo_root/README.md" "$stage_dir/README.md"
  cp "$repo_root/LICENSE.md" "$stage_dir/LICENSE.md"
  cp "$repo_root/install.sh" "$stage_dir/install.sh"
  chmod +x "$stage_dir/install.sh"

  case "$os" in
    windows)
      rm -f "$out_dir/$archive_name"
      (
        cd "$out_dir"
        zip -rq "$archive_name" "$base_name"
      )
      ;;
    *)
      tar -C "$out_dir" -czf "$out_dir/$archive_name" "$base_name"
      ;;
  esac

  printf '%s\n' "$archive_name" >>"$out_dir/.artifacts"
}

write_checksums() {
  local checksum_file="$out_dir/sha256sum.txt"
  rm -f "$checksum_file"

  (
    cd "$out_dir"
    while IFS= read -r artifact; do
      [[ -n "$artifact" ]] || continue
      sha256sum "$artifact"
    done <"$out_dir/.artifacts"
  ) >"$checksum_file"

  printf 'sha256sum.txt\n' >>"$out_dir/.artifacts"
}

main() {
  require_command git
  require_command go
  require_command npm
  require_command tar
  require_command zip
  require_command sha256sum

  resolve_version
  resolve_commit
  resolve_build_date

  rm -rf "$out_dir"
  mkdir -p "$out_dir"
  : >"$out_dir/.artifacts"

  cd "$repo_root"
  build_webui

  for target in $targets; do
    IFS='/' read -r os arch <<<"$target"
    package_target "$os" "$arch"
  done

  write_checksums
  rm -f "$out_dir/.artifacts"

  printf 'Built release artifacts in %s\n' "$out_dir"
}

main "$@"