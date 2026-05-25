#!/bin/bash

set -u

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

ROLE=""
PANEL_URL=""
INSTALL_TOKEN=""
MACHINE_NAME=""

AGENT_BIN="/usr/local/bin/onlytun-agent"
CONFIG_DIR="/etc/onlytun"
CONFIG_PATH="${CONFIG_DIR}/cache.json"
SERVICE_PATH="/etc/systemd/system/onlytun-agent.service"
DEFAULT_TUNNEL_ADDR="0.0.0.0:19999"
RELEASE_BASE_URL="${ONLYTUN_RELEASE_BASE_URL:-https://github.com/kzlgithub/onlytun/releases/latest/download}"

usage() {
  cat <<EOF
Usage:
  bash install.sh --token INSTALL_TOKEN --role ingress|egress --panel http://host:port

Optional non-interactive flags:
  --name MACHINE_NAME
EOF
}

info() {
  printf "${YELLOW}[INFO]${NC} %s\n" "$1"
}

success() {
  printf "${GREEN}[OK]${NC} %s\n" "$1"
}

warn() {
  printf "${YELLOW}[WARN]${NC} %s\n" "$1"
}

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
      --role)
        ROLE="${2:-}"
        shift 2
        ;;
      --panel)
        PANEL_URL="${2:-}"
        shift 2
        ;;
      --token)
        INSTALL_TOKEN="${2:-}"
        shift 2
        ;;
      --name)
        MACHINE_NAME="${2:-}"
        shift 2
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

  [ -n "$INSTALL_TOKEN" ] || fail "--token is required."
}

prompt_if_missing() {
  if [ -z "$ROLE" ]; then
    [ -t 0 ] || fail "--role is required in non-interactive mode."
    while true; do
      printf "Select machine role [ingress/egress]: "
      read -r ROLE
      case "$ROLE" in
        ingress|egress) break ;;
        *) warn "Please enter ingress or egress." ;;
      esac
    done
  fi

  case "$ROLE" in
    ingress|egress) ;;
    *) fail "--role only supports ingress or egress." ;;
  esac

  if [ -z "$PANEL_URL" ]; then
    [ -t 0 ] || fail "--panel is required in non-interactive mode."
    while [ -z "$PANEL_URL" ]; do
      printf "Panel URL, for example http://1.2.3.4:8080: "
      read -r PANEL_URL
    done
  fi
  PANEL_URL="${PANEL_URL%/}"

}

detect_os() {
  [ -f /etc/os-release ] || fail "Cannot detect OS: /etc/os-release is missing."
  # shellcheck disable=SC1091
  . /etc/os-release

  local os_id="${ID:-}"
  local version_id="${VERSION_ID:-}"
  local major="${version_id%%.*}"

  case "$os_id" in
    ubuntu)
      [ "${major:-0}" -ge 18 ] || fail "Ubuntu 18.04+ is required."
      ;;
    debian)
      [ "${major:-0}" -ge 10 ] || fail "Debian 10+ is required."
      ;;
    centos)
      [ "${major:-0}" -ge 7 ] || fail "CentOS 7+ is required."
      ;;
    rocky)
      [ "${major:-0}" -ge 8 ] || fail "Rocky Linux 8+ is required."
      ;;
    *)
      fail "Unsupported OS: ${os_id:-unknown}"
      ;;
  esac

  success "OS check passed: ${PRETTY_NAME:-$os_id}"
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64)
      ARCH="amd64"
      ;;
    aarch64|arm64)
      ARCH="arm64"
      ;;
    *)
      fail "Unsupported CPU architecture: $(uname -m). Only amd64 and arm64 are supported."
      ;;
  esac

  success "CPU architecture check passed: ${ARCH}"
}

prepare_dirs() {
  mkdir -p "$CONFIG_DIR" /usr/local/bin || fail "Failed to create required directories."
  success "Directories are ready."
}

download_agent() {
  local url="${RELEASE_BASE_URL}/onlytun-agent-linux-${ARCH}"
  info "Downloading Agent binary: ${url}"
  curl --retry 3 --retry-delay 2 -fL# "$url" -o "$AGENT_BIN" || fail "Failed to download Agent binary."
  chmod +x "$AGENT_BIN" || fail "Failed to chmod Agent binary."
  success "Agent binary installed at ${AGENT_BIN}"
}

fetch_public_ip() {
  PUBLIC_IP="$(curl -fsS https://api.ipify.org)" || fail "Failed to detect public IP."
  [ -n "$PUBLIC_IP" ] || fail "Public IP is empty."
  [ -n "$MACHINE_NAME" ] || MACHINE_NAME="$PUBLIC_IP"
  success "Public IP: ${PUBLIC_IP}"
}

json_escape() {
  printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

register_machine() {
  local payload
  payload=$(cat <<EOF
{"name":"$(json_escape "$MACHINE_NAME")","role":"$(json_escape "$ROLE")","token":"$(json_escape "$INSTALL_TOKEN")","ip":"$(json_escape "$PUBLIC_IP")","os":"$(json_escape "$(uname -s)")"}
EOF
)

  info "Registering machine with panel..."
  local response
  response="$(curl -fsS -X POST "${PANEL_URL}/api/agent/register" \
    -H "Authorization: Bearer ${INSTALL_TOKEN}" \
    -H "Content-Type: application/json" \
    -d "$payload")" || fail "Machine registration failed. Check panel URL, token, and network."

  MACHINE_ID="$(printf '%s' "$response" | grep -oE '"machine_id"[[:space:]]*:[[:space:]]*"[^"]+"' | sed 's/.*"machine_id"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')"
  PSK="$(printf '%s' "$response" | grep -oE '"psk"[[:space:]]*:[[:space:]]*"[^"]+"' | sed 's/.*"psk"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')"

  [ -n "${MACHINE_ID:-}" ] || fail "Registration response does not contain machine_id."
  [ -n "${PSK:-}" ] || fail "Registration response does not contain psk."
  success "Machine registered. Machine ID: ${MACHINE_ID}"
}

write_config() {
  cat >"$CONFIG_PATH" <<EOF
{
  "machine_id": "${MACHINE_ID}",
  "role": "${ROLE}",
  "psk": "${PSK}",
  "panel_url": "${PANEL_URL}",
  "token": "${INSTALL_TOKEN}",
  "tunnel_listen_addr": "${DEFAULT_TUNNEL_ADDR}",
  "rules": []
}
EOF
  success "Config written to ${CONFIG_PATH}"
}

write_service() {
  cat >"$SERVICE_PATH" <<EOF
[Unit]
Description=OnlyTun Agent
After=network.target

[Service]
Type=simple
ExecStart=${AGENT_BIN} --config ${CONFIG_PATH}
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
  success "systemd service written to ${SERVICE_PATH}"
}

enable_service() {
  systemctl daemon-reload || fail "systemctl daemon-reload failed."
  systemctl enable onlytun-agent >/dev/null 2>&1 || fail "Failed to enable onlytun-agent."
  systemctl restart onlytun-agent || fail "Failed to start onlytun-agent."
  success "onlytun-agent service started."
}

check_service() {
  info "Checking service status..."
  sleep 3
  if systemctl is-active --quiet onlytun-agent; then
    success "OnlyTun Agent installed successfully."
    printf "%bPanel:%b %s\n" "$GREEN" "$NC" "$PANEL_URL"
    printf "%bConfig:%b %s\n" "$GREEN" "$NC" "$CONFIG_PATH"
  else
    warn "onlytun-agent is not active. Service status:"
    systemctl status onlytun-agent --no-pager || true
    fail "OnlyTun Agent was installed but failed to start."
  fi
}

main() {
  require_root
  require_command curl
  require_command systemctl
  require_command grep
  require_command sed

  parse_args "$@"
  prompt_if_missing
  detect_os
  detect_arch
  prepare_dirs
  download_agent
  fetch_public_ip
  register_machine
  write_config
  write_service
  enable_service
  check_service
}

main "$@"
