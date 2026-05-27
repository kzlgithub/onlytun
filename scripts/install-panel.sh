#!/bin/bash

set -u

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

PANEL_PORT="8080"
PANEL_PASSWORD=""
ACTION=""

PANEL_BIN="/usr/local/bin/onlytun-panel"
CONFIG_DIR="/etc/onlytun"
SERVICE_NAME="onlytun-panel"
SERVICE_PATH="/etc/systemd/system/${SERVICE_NAME}.service"
RELEASE_BASE_URL="${ONLYTUN_RELEASE_BASE_URL:-https://github.com/kzlgithub/onlytun/releases/latest/download}"

usage() {
  cat <<EOF
Usage:
  bash install-panel.sh
  bash install-panel.sh --port 8080 --password YOUR_PASSWORD
  bash install-panel.sh --install --port 8080 --password YOUR_PASSWORD
  bash install-panel.sh --uninstall
  bash install-panel.sh --update
  bash install-panel.sh --check-version
EOF
}

info() { printf "${YELLOW}[INFO]${NC} %s\n" "$1"; }
success() { printf "${GREEN}[OK]${NC} %s\n" "$1"; }
warn() { printf "${YELLOW}[WARN]${NC} %s\n" "$1"; }
fail() {
  printf "${RED}[ERROR]${NC} %s\n" "$1" >&2
  exit 1
}

require_root() {
  if [ "${EUID:-$(id -u)}" -ne 0 ]; then
    fail "Please run this script as root."
  fi
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "Missing required command: $1"
}

parse_args() {
  while [ $# -gt 0 ]; do
    case "$1" in
      --port)
        PANEL_PORT="${2:-}"
        shift 2
        ;;
      --password)
        PANEL_PASSWORD="${2:-}"
        shift 2
        ;;
      --install)
        ACTION="install"
        shift
        ;;
      --uninstall)
        ACTION="uninstall"
        shift
        ;;
      --update)
        ACTION="update"
        shift
        ;;
      --check-version|--version-check)
        ACTION="check-version"
        shift
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        fail "Unknown argument: $1"
        ;;
    esac
  done
}

prompt_action_if_missing() {
  if [ -n "$ACTION" ]; then
    return
  fi
  if [ -n "$PANEL_PASSWORD" ]; then
    ACTION="install"
    return
  fi
  [ -t 0 ] || ACTION="install"
  if [ -n "$ACTION" ]; then
    return
  fi

  while true; do
    printf "请选择操作：\n"
    printf "  1. 安装\n"
    printf "  2. 卸载\n"
    printf "  3. 更新\n"
    printf "  4. 查看&更新面板版本\n"
    printf "请输入序号 [1-4]: "
    read -r choice
    case "$choice" in
      1) ACTION="install"; break ;;
      2) ACTION="uninstall"; break ;;
      3) ACTION="update"; break ;;
      4) ACTION="check-version"; break ;;
      *) warn "请输入 1、2、3 或 4。" ;;
    esac
  done
}

validate_port() {
  printf '%s' "$PANEL_PORT" | grep -Eq '^[0-9]+$' || fail "--port must be a number."
  [ "$PANEL_PORT" -ge 1 ] && [ "$PANEL_PORT" -le 65535 ] || fail "--port must be between 1 and 65535."
}

detect_os() {
  [ -f /etc/os-release ] || fail "Cannot detect OS: /etc/os-release is missing."
  # shellcheck disable=SC1091
  . /etc/os-release

  local os_id="${ID:-}"
  local version_id="${VERSION_ID:-}"
  local major="${version_id%%.*}"

  case "$os_id" in
    ubuntu) [ "${major:-0}" -ge 18 ] || fail "Ubuntu 18.04+ is required." ;;
    debian) [ "${major:-0}" -ge 10 ] || fail "Debian 10+ is required." ;;
    centos) [ "${major:-0}" -ge 7 ] || fail "CentOS 7+ is required." ;;
    rocky) [ "${major:-0}" -ge 8 ] || fail "Rocky Linux 8+ is required." ;;
    *) fail "Unsupported OS: ${os_id:-unknown}" ;;
  esac

  success "OS check passed: ${PRETTY_NAME:-$os_id}"
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) fail "Unsupported CPU architecture: $(uname -m). Only amd64 and arm64 are supported." ;;
  esac

  success "CPU architecture check passed: ${ARCH}"
}

prepare_dirs() {
  mkdir -p "$CONFIG_DIR" /usr/local/bin || fail "Failed to create required directories."
  success "Directories are ready."
}

download_panel_to() {
  local dest="$1"
  local url="${RELEASE_BASE_URL}/onlytun-panel-linux-${ARCH}"
  info "Downloading panel binary: ${url}"
  curl --http1.1 --retry 5 --retry-delay 2 -fL# "$url" -o "$dest" || fail "Failed to download panel binary. Check network or GitHub Release."
}

download_panel() {
  download_panel_to "$PANEL_BIN"
  chmod +x "$PANEL_BIN" || fail "Failed to chmod panel binary."
  success "Panel binary installed at ${PANEL_BIN}"
}

prompt_password() {
  if [ -n "$PANEL_PASSWORD" ]; then
    return
  fi

  while true; do
    printf "Panel password: "
    stty -echo
    read -r first
    stty echo
    printf "\nConfirm panel password: "
    stty -echo
    read -r second
    stty echo
    printf "\n"

    [ -n "$first" ] || warn "Password cannot be empty."
    [ -n "$first" ] || continue

    if [ "$first" != "$second" ]; then
      warn "Passwords do not match. Please retry."
      continue
    fi

    PANEL_PASSWORD="$first"
    break
  done
}

escape_systemd_value() {
  printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

write_service() {
  local escaped_password
  escaped_password="$(escape_systemd_value "$PANEL_PASSWORD")"

  cat >"$SERVICE_PATH" <<EOF
[Unit]
Description=OnlyTun Panel
After=network.target

[Service]
Type=simple
Environment="ONLYTUN_PASSWORD=${escaped_password}"
Environment="ONLYTUN_PORT=${PANEL_PORT}"
ExecStart=${PANEL_BIN}
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
  success "systemd service written to ${SERVICE_PATH}"
}

enable_service() {
  systemctl daemon-reload || fail "systemctl daemon-reload failed."
  systemctl enable "$SERVICE_NAME" >/dev/null 2>&1 || fail "Failed to enable ${SERVICE_NAME}."
  systemctl restart "$SERVICE_NAME" || fail "Failed to start ${SERVICE_NAME}."
  success "${SERVICE_NAME} service started."
}

valid_ip() {
  printf '%s' "$1" | grep -Eq '^([0-9]{1,3}\.){3}[0-9]{1,3}$|^[0-9a-fA-F:]+$'
}

fetch_public_ip() {
  PUBLIC_IP=""
  local services="
https://api.ipify.org
https://ifconfig.me/ip
https://icanhazip.com
https://ident.me
http://checkip.amazonaws.com
http://ifconfig.me/ip
"

  for ip_service in $services; do
    PUBLIC_IP="$(curl -fsS --connect-timeout 3 --max-time 6 "$ip_service" 2>/dev/null | tr -d '[:space:]' || true)"
    if [ -n "$PUBLIC_IP" ] && valid_ip "$PUBLIC_IP"; then
      success "Public IP: ${PUBLIC_IP}"
      return 0
    fi
  done

  PUBLIC_IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
  if [ -n "$PUBLIC_IP" ]; then
    warn "Could not query public IP services. Showing local server IP instead: ${PUBLIC_IP}"
    return 0
  fi

  PUBLIC_IP="SERVER_IP"
  warn "Could not detect IP automatically. Replace SERVER_IP with your server public IP."
  return 0
}

check_service() {
  info "Waiting for service startup..."
  sleep 3
  if systemctl is-active --quiet "$SERVICE_NAME"; then
    fetch_public_ip
    success "OnlyTun Panel installed successfully."
    printf "%bPanel URL:%b http://%s:%s\n" "$GREEN" "$NC" "$PUBLIC_IP" "$PANEL_PORT"
  else
    warn "Service is not active. Status output:"
    systemctl status "$SERVICE_NAME" --no-pager || true
    fail "OnlyTun Panel was installed but failed to start."
  fi
}

uninstall_panel() {
  info "Uninstalling OnlyTun Panel..."
  systemctl stop "$SERVICE_NAME" >/dev/null 2>&1 || true
  systemctl disable "$SERVICE_NAME" >/dev/null 2>&1 || true
  pkill -x "$SERVICE_NAME" >/dev/null 2>&1 || true
  rm -f "$SERVICE_PATH" "/lib/systemd/system/${SERVICE_NAME}.service"
  rm -f "$PANEL_BIN"
  rm -rf "$CONFIG_DIR"
  systemctl daemon-reload >/dev/null 2>&1 || true
  systemctl reset-failed "$SERVICE_NAME.service" >/dev/null 2>&1 || true
  success "OnlyTun Panel removed. Service, binary, and ${CONFIG_DIR} have been deleted."
}

update_panel() {
  info "Updating OnlyTun Panel..."
  detect_os
  detect_arch
  prepare_dirs

  local tmp_file
  tmp_file="$(mktemp /tmp/onlytun-panel.XXXXXX)" || fail "Failed to create temporary file."
  download_panel_to "$tmp_file"
  chmod +x "$tmp_file" || fail "Failed to chmod downloaded binary."

  if [ -f "$PANEL_BIN" ]; then
    cp "$PANEL_BIN" "${PANEL_BIN}.bak.$(date +%Y%m%d%H%M%S)" || fail "Failed to backup current panel binary."
  fi
  cp "$tmp_file" "${PANEL_BIN}.new" || fail "Failed to stage updated panel binary."
  chmod +x "${PANEL_BIN}.new" || fail "Failed to chmod staged panel binary."
  mv -f "${PANEL_BIN}.new" "$PANEL_BIN" || fail "Failed to install updated panel binary."
  chmod +x "$PANEL_BIN" || fail "Failed to chmod updated panel binary."
  rm -f "$tmp_file"

  systemctl daemon-reload || fail "systemctl daemon-reload failed."
  systemctl restart "$SERVICE_NAME" || fail "Failed to restart ${SERVICE_NAME}."
  sleep 3
  if systemctl is-active --quiet "$SERVICE_NAME"; then
    success "OnlyTun Panel updated successfully. Database preserved at ${CONFIG_DIR}/panel.db."
  else
    systemctl status "$SERVICE_NAME" --no-pager || true
    fail "OnlyTun Panel update finished but service is not active."
  fi
}

current_panel_version() {
  if [ -x "$PANEL_BIN" ]; then
    "$PANEL_BIN" --version 2>/dev/null || printf "unknown"
  else
    printf "not installed"
  fi
}

latest_release_version() {
  curl -fsSL --connect-timeout 5 --max-time 12 \
    "https://api.github.com/repos/kzlgithub/onlytun/releases/latest" 2>/dev/null |
    grep -oE '"tag_name"[[:space:]]*:[[:space:]]*"[^"]+"' |
    sed 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/' |
    head -1
}

check_panel_version() {
  local current latest answer
  current="$(current_panel_version)"
  latest="$(latest_release_version || true)"
  [ -n "$latest" ] || latest="unknown"

  printf "%bCurrent Panel version:%b %s\n" "$GREEN" "$NC" "$current"
  printf "%bLatest Release version:%b %s\n" "$GREEN" "$NC" "$latest"

  if [ "$current" = "$latest" ]; then
    success "Panel is already up to date."
    return 0
  fi

  if [ -t 0 ]; then
    printf "Update Panel now? [y/N]: "
    read -r answer
    case "$answer" in
      y|Y) update_panel ;;
      *) warn "Update skipped." ;;
    esac
  else
    warn "Run 'bash install-panel.sh --update' to update Panel."
  fi
}

main() {
  require_root
  require_command curl
  require_command systemctl
  require_command grep
  require_command sed
  require_command awk

  parse_args "$@"
  prompt_action_if_missing
  case "$ACTION" in
    uninstall)
      uninstall_panel
      exit 0
      ;;
    update)
      update_panel
      exit 0
      ;;
    check-version)
      check_panel_version
      exit 0
      ;;
  esac

  validate_port
  require_command stty
  detect_os
  detect_arch
  prepare_dirs
  download_panel
  prompt_password
  write_service
  enable_service
  check_service
}

main "$@"
