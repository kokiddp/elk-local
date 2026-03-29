#!/usr/bin/env bash

set -euo pipefail

install_root="${HOME}/.elk-local"
bin_dir="$install_root/bin"
webui_dist_dir="$install_root/webui/dist"
share_dir="$install_root/share"
config_path="$install_root/config.yaml"
service_name="elk-local.service"
service_dir="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user"
service_path="$service_dir/$service_name"
service_wrapper_path="$bin_dir/elk-local-daemon-service"
tool_proxy_wrapper_path="$bin_dir/elk-local-tool-proxy"
default_environments_dir="${HOME}/elk-local/environments"
default_backups_dir="${HOME}/elk-local/backups"
default_webui_port="4173"
path_block_start="# >>> elk-local >>>"
path_block_end="# <<< elk-local <<<"

usage() {
  cat <<'EOF'
Usage:
  ./install.sh
  ./install.sh --uninstall

The installer copies ELK-Local into ${HOME}/.elk-local, updates your shell PATH,
and can register a user-level daemon service for startup on login.
EOF
}

prompt_with_default() {
  local prompt_text="$1"
  local default_value="$2"
  local answer

  read -r -p "$prompt_text [$default_value]: " answer
  if [[ -z "$answer" ]]; then
    printf '%s\n' "$default_value"
    return
  fi

  printf '%s\n' "$answer"
}

prompt_yes_no() {
  local prompt_text="$1"
  local default_value="$2"
  local answer

  while true; do
    read -r -p "$prompt_text" answer
    answer="${answer:-$default_value}"
    case "${answer,,}" in
      y|yes)
        return 0
        ;;
      n|no)
        return 1
        ;;
    esac
    printf 'Please answer yes or no.\n' >&2
  done
}

detect_shell_rc() {
  case "$(basename "${SHELL:-bash}")" in
    bash)
      printf '%s/.bashrc\n' "$HOME"
      ;;
    zsh)
      printf '%s/.zshrc\n' "$HOME"
      ;;
    *)
      printf '%s/.profile\n' "$HOME"
      ;;
  esac
}

remove_marked_block() {
  local target_file="$1"

  if [[ ! -f "$target_file" ]]; then
    return
  fi

  local tmp_file
  tmp_file="$(mktemp)"
  awk -v start="$path_block_start" -v end="$path_block_end" '
    $0 == start { skip = 1; next }
    $0 == end { skip = 0; next }
    skip != 1 { print }
  ' "$target_file" >"$tmp_file"
  mv "$tmp_file" "$target_file"
}

ensure_path_block() {
  local shell_rc="$1"

  mkdir -p "$(dirname "$shell_rc")"
  touch "$shell_rc"
  remove_marked_block "$shell_rc"

  cat >>"$shell_rc" <<EOF

$path_block_start
export PATH="$bin_dir:\$PATH"
$path_block_end
EOF
}

read_config_value() {
  local key="$1"
  if [[ ! -f "$config_path" ]]; then
    return
  fi

  sed -n "s/^${key}:[[:space:]]*//p" "$config_path" | head -n 1 | sed 's/^"//; s/"$//'
}

config_value_or_default() {
  local key="$1"
  local fallback="$2"
  local value

  value="$(read_config_value "$key" || true)"
  if [[ -n "$value" ]]; then
    printf '%s\n' "$value"
    return
  fi

  printf '%s\n' "$fallback"
}

preflight_checks() {
  if ! command -v docker >/dev/null 2>&1; then
    printf 'Warning: docker is not on PATH yet. ELK-Local requires Docker Engine with Compose v2.\n' >&2
    return
  fi

  if ! docker compose version >/dev/null 2>&1; then
    printf 'Warning: docker compose v2 is not available yet. ELK-Local requires Docker Engine with Compose v2.\n' >&2
  fi
}

stop_existing_daemon() {
  if [[ -x "$bin_dir/elk-local" ]]; then
    "$bin_dir/elk-local" daemon stop --project-root "$HOME" >/dev/null 2>&1 || true
  fi
}

write_service_wrapper() {
  cat >"$service_wrapper_path" <<EOF
#!/usr/bin/env bash

set -euo pipefail

elk_bin="${bin_dir}/elk-local"
project_root="${HOME}"
webui_dist="${install_root}/webui/dist"
config_path="${install_root}/config.yaml"
webui_port="${default_webui_port}"

if [[ -f "\$config_path" ]]; then
  configured_port="$(sed -n 's/^webuiPort:[[:space:]]*//p' "\$config_path" | head -n 1 | sed 's/^"//; s/"$//')"
  if [[ "\$configured_port" =~ ^[0-9]+$ ]] && (( configured_port >= 1 && configured_port <= 65535 )); then
    webui_port="\$configured_port"
  fi
fi

cleanup() {
  "\$elk_bin" daemon stop --project-root "\$project_root" >/dev/null 2>&1 || true
}

trap cleanup EXIT INT TERM

"\$elk_bin" daemon start --project-root "\$project_root" --host 127.0.0.1 --port "\$webui_port" --webui-dist "\$webui_dist" >/dev/null

while "\$elk_bin" daemon status --project-root "\$project_root" >/dev/null 2>&1; do
  sleep 5
done

exit 1
EOF
  chmod +x "$service_wrapper_path"
}

write_tool_proxy_wrappers() {
  cat >"$tool_proxy_wrapper_path" <<EOF
#!/usr/bin/env bash

set -euo pipefail

tool_name="\$(basename "\$0")"
exec "${bin_dir}/elk-local" proxy "\$tool_name" "\$@"
EOF
  chmod +x "$tool_proxy_wrapper_path"

  for tool_name in wp mysql mariadb mysqldump mariadb-dump; do
    ln -sf "$tool_proxy_wrapper_path" "$bin_dir/$tool_name"
  done
}

write_systemd_service() {
  mkdir -p "$service_dir"
  cat >"$service_path" <<EOF
[Unit]
Description=ELK-Local background daemon
After=default.target

[Service]
Type=simple
ExecStart=${service_wrapper_path}
Restart=on-failure
RestartSec=5
WorkingDirectory=${HOME}

[Install]
WantedBy=default.target
EOF
}

configure_autostart() {
  local enable_autostart="$1"

  rm -f "$service_path" "$service_wrapper_path"

  if [[ "$enable_autostart" != "yes" ]]; then
    if command -v systemctl >/dev/null 2>&1; then
      systemctl --user disable --now "$service_name" >/dev/null 2>&1 || true
      systemctl --user daemon-reload >/dev/null 2>&1 || true
    fi
    return
  fi

  if ! command -v systemctl >/dev/null 2>&1; then
    printf 'Warning: systemctl is not available, so daemon auto-start was skipped.\n' >&2
    return
  fi

  write_service_wrapper
  write_systemd_service

  if ! systemctl --user daemon-reload >/dev/null 2>&1; then
    printf 'Warning: unable to reload the user systemd daemon. The service file was written to %s.\n' "$service_path" >&2
    return
  fi

  if ! systemctl --user enable --now "$service_name" >/dev/null 2>&1; then
    printf 'Warning: unable to enable/start the user service automatically. You can run:\n' >&2
    printf '  systemctl --user enable --now %s\n' "$service_name" >&2
  fi
}

write_config() {
  local environments_dir="$1"
  local backups_dir="$2"
  local webui_port="$3"
  local shell_rc="$4"
  local daemon_auto_start="$5"

  mkdir -p "$install_root"
  cat >"$config_path" <<EOF
environmentsDir: "$environments_dir"
backupsDir: "$backups_dir"
webuiPort: ${webui_port}
shellRC: "$shell_rc"
daemonAutoStart: ${daemon_auto_start}
EOF
}

copy_payload() {
  local source_dir="$1"
  local source_binary="$source_dir/elk-local"
  local source_webui_dist="$source_dir/webui/dist"

  if [[ ! -x "$source_binary" ]]; then
    printf 'Expected an elk-local binary next to install.sh at %s\n' "$source_binary" >&2
    exit 1
  fi

  if [[ ! -d "$source_webui_dist" ]]; then
    printf 'Expected built web UI assets next to install.sh at %s\n' "$source_webui_dist" >&2
    exit 1
  fi

  mkdir -p "$bin_dir" "$install_root/webui" "$share_dir"

  cp "$source_binary" "$bin_dir/elk-local"
  chmod +x "$bin_dir/elk-local"
  write_tool_proxy_wrappers

  rm -rf "$webui_dist_dir"
  cp -R "$source_webui_dist" "$webui_dist_dir"

  cp "$source_dir/README.md" "$share_dir/README.md"
  cp "$source_dir/LICENSE.md" "$share_dir/LICENSE.md"
  cp "$source_dir/install.sh" "$install_root/install.sh"
  chmod +x "$install_root/install.sh"
}

confirm_upgrade_if_needed() {
  if [[ ! -d "$install_root" ]]; then
    return
  fi

  if [[ -z "$(find "$install_root" -mindepth 1 -maxdepth 1 -print -quit 2>/dev/null)" ]]; then
    return
  fi

  if ! prompt_yes_no "Existing install data was found in ${install_root}. Upgrade/overwrite the program files? [y/N]: " "n"; then
    printf 'Install cancelled.\n' >&2
    exit 1
  fi
}

run_install() {
  local script_dir
  local environments_dir
  local backups_dir
  local webui_port
  local shell_rc
  local auto_start="no"
  local auto_start_default="n"
  local auto_start_prompt='Start the ELK-Local daemon automatically on login? [y/N]: '
  local prompt_webui_port

  script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

  confirm_upgrade_if_needed
  preflight_checks
  stop_existing_daemon

  environments_dir="$(prompt_with_default 'Default folder for environment work trees' "$(config_value_or_default environmentsDir "$default_environments_dir")")"
  backups_dir="$(prompt_with_default 'Default folder for managed backups' "$(config_value_or_default backupsDir "$default_backups_dir")")"
  prompt_webui_port="$(config_value_or_default webuiPort "$default_webui_port")"
  webui_port="$(prompt_with_default 'Web UI and daemon port' "$prompt_webui_port")"
  shell_rc="$(prompt_with_default 'Shell profile file for PATH updates' "$(config_value_or_default shellRC "$(detect_shell_rc)")")"

  if [[ "$(read_config_value daemonAutoStart || true)" == "true" ]]; then
    auto_start_default="y"
    auto_start_prompt='Start the ELK-Local daemon automatically on login? [Y/n]: '
  fi

  if prompt_yes_no "$auto_start_prompt" "$auto_start_default"; then
    auto_start="yes"
  fi

  environments_dir="$(realpath -m "$environments_dir")"
  backups_dir="$(realpath -m "$backups_dir")"
  shell_rc="$(realpath -m "$shell_rc")"

  if ! [[ "$webui_port" =~ ^[0-9]+$ ]] || (( webui_port < 1 || webui_port > 65535 )); then
    printf 'Invalid web UI port: %s\n' "$webui_port" >&2
    exit 1
  fi

  mkdir -p "$environments_dir" "$backups_dir"
  copy_payload "$script_dir"
  ensure_path_block "$shell_rc"
  write_config "$environments_dir" "$backups_dir" "$webui_port" "$shell_rc" "$([[ "$auto_start" == "yes" ]] && printf 'true' || printf 'false')"
  configure_autostart "$auto_start"

  printf '\nInstallation complete.\n'
  printf '  Binary: %s\n' "$bin_dir/elk-local"
  printf '  Registry root: %s/.elk-local\n' "$HOME"
  printf '  Default environments: %s\n' "$environments_dir"
  printf '  Default backups: %s\n' "$backups_dir"
  printf '  Web UI port: %s\n' "$webui_port"
  printf '  PATH updated in: %s\n' "$shell_rc"
  printf '\nOpen a new shell or run:\n'
  printf '  export PATH="%s:$PATH"\n' "$bin_dir"
}

run_uninstall() {
  local configured_shell_rc
  local shell_rc

  if ! prompt_yes_no "Uninstall ELK-Local from ${install_root}? Program files will be removed, but environments and backups will be preserved. [y/N]: " "n"; then
    printf 'Uninstall cancelled.\n' >&2
    exit 1
  fi

  configured_shell_rc="$(read_config_value shellRC || true)"

  stop_existing_daemon
  configure_autostart "no"

  for shell_rc in "$configured_shell_rc" "$HOME/.bashrc" "$HOME/.zshrc" "$HOME/.profile"; do
    [[ -n "$shell_rc" ]] || continue
    remove_marked_block "$shell_rc"
  done

  rm -rf "$bin_dir" "$install_root/webui" "$install_root/daemon" "$share_dir"
  rm -f "$install_root/install.sh"

  printf 'ELK-Local was uninstalled. Managed environments and backups were left in place.\n'
}

main() {
  if [[ $# -gt 1 ]]; then
    usage >&2
    exit 1
  fi

  case "${1:-}" in
    "")
      run_install
      ;;
    --uninstall)
      run_uninstall
      ;;
    -h|--help)
      usage
      ;;
    *)
      usage >&2
      exit 1
      ;;
  esac
}

main "$@"