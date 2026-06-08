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
ACTION=""
ACCESS_ADDR=""
IS_IX=false
TUNNEL_ADVERTISE_ADDR=""

AGENT_BIN="/usr/local/bin/onlytun-agent"
CONFIG_DIR="/etc/onlytun"
CONFIG_PATH="${CONFIG_DIR}/cache.json"
SERVICE_PATH="/etc/systemd/system/onlytun-agent.service"
SYSCTL_PATH="/etc/sysctl.d/99-onlytun-tcp.conf"
DEFAULT_TUNNEL_ADDR="0.0.0.0:19999"
RELEASE_BASE_URL="${ONLYTUN_RELEASE_BASE_URL:-https://github.com/kzlgithub/onlytun/releases/latest/download}"

usage() {
  cat <<EOF
Usage:
  bash install.sh
  bash install.sh --token INSTALL_TOKEN --role ingress|egress --panel http://host:port --access-addr NODE_ACCESS_ADDR
  bash install.sh --install --token INSTALL_TOKEN --role ingress|egress --panel http://host:port --access-addr NODE_ACCESS_ADDR
  bash install.sh --uninstall
  bash install.sh --update
  bash install.sh --check-version

Optional non-interactive flags:
  --name MACHINE_NAME
  --access-addr NODE_ACCESS_ADDR
  --ix
  --tunnel-advertise-addr HOST:PORT
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
      --access-addr)
        ACCESS_ADDR="${2:-}"
        shift 2
        ;;
      --ix)
        IS_IX=true
        shift
        ;;
      --tunnel-advertise-addr)
        TUNNEL_ADVERTISE_ADDR="${2:-}"
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
  if [ -n "$INSTALL_TOKEN" ] || [ -n "$ROLE" ] || [ -n "$PANEL_URL" ] || [ -n "$ACCESS_ADDR" ]; then
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
    printf "  4. 查看&更新 Agent 版本\n"
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

validate_install_args() {
  [ -n "$INSTALL_TOKEN" ] || fail "--token is required."
  [ -n "$ACCESS_ADDR" ] || fail "--access-addr is required."
  validate_access_addr "$ACCESS_ADDR"
  [ -z "$MACHINE_NAME" ] && MACHINE_NAME="$ACCESS_ADDR"
  if [ "$IS_IX" = true ]; then
    [ "$ROLE" = "egress" ] || fail "--ix can only be used with --role egress."
  fi
}

prompt_if_missing() {
  if [ -z "$INSTALL_TOKEN" ]; then
    [ -t 0 ] || fail "--token is required in non-interactive mode."
    while [ -z "$INSTALL_TOKEN" ]; do
      printf "Install token: "
      read -r INSTALL_TOKEN
    done
  fi

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

  if [ -z "$ACCESS_ADDR" ]; then
    [ -t 0 ] || fail "--access-addr is required in non-interactive mode."
    while [ -z "$ACCESS_ADDR" ]; do
      printf "Node access address (public IP/domain; IX uses domestic entry IP): "
      read -r ACCESS_ADDR
      ACCESS_ADDR="$(printf '%s' "$ACCESS_ADDR" | tr -d '[:space:]')"
      if [ -z "$ACCESS_ADDR" ]; then
        warn "Node access address cannot be empty."
      fi
    done
  fi

  if [ "$IS_IX" = true ] && [ "$ROLE" != "egress" ]; then
    fail "--ix can only be used with egress role."
  fi
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

apply_tcp_tuning() {
  cat >"$SYSCTL_PATH" <<EOF
net.core.somaxconn = 4096
net.core.netdev_max_backlog = 5000
net.ipv4.tcp_max_syn_backlog = 4096
net.ipv4.ip_local_port_range = 10000 65000
net.ipv4.tcp_fin_timeout = 15
net.ipv4.tcp_tw_reuse = 1
net.ipv4.tcp_syncookies = 1
EOF

  if sysctl -p "$SYSCTL_PATH" >/dev/null 2>&1; then
    success "TCP tuning applied via ${SYSCTL_PATH}"
  else
    warn "TCP tuning file was written, but sysctl failed to apply all settings immediately."
  fi
}

download_agent_to() {
  local dest="$1"
  local url="${RELEASE_BASE_URL}/onlytun-agent-linux-${ARCH}"
  info "Downloading Agent binary: ${url}"
  curl --http1.1 --retry 5 --retry-delay 2 -fL# "$url" -o "$dest" || fail "Failed to download Agent binary."
}

download_agent() {
  download_agent_to "$AGENT_BIN"
  chmod +x "$AGENT_BIN" || fail "Failed to chmod Agent binary."
  success "Agent binary installed at ${AGENT_BIN}"
}

validate_access_addr() {
  local value="$1"
  [ -n "$value" ] || fail "--access-addr is required."
  case "$value" in
    *ACCESS_ADDR*) fail "Please replace the access address placeholder with the real node access address." ;;
  esac
  printf '%s' "$value" | grep -Eq '^[^[:space:]/]+$' || fail "--access-addr must be an IP address or hostname without path."
  case "$value" in
    *://*) fail "--access-addr must not include http:// or https://." ;;
  esac
  success "Node access address: ${value}"
}

json_escape() {
  printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

register_machine() {
  local payload
  payload=$(cat <<EOF
{"name":"$(json_escape "$MACHINE_NAME")","role":"$(json_escape "$ROLE")","token":"$(json_escape "$INSTALL_TOKEN")","ip":"$(json_escape "$ACCESS_ADDR")","os":"$(json_escape "$(uname -s)")","is_ix":${IS_IX},"tunnel_advertise_addr":"$(json_escape "$TUNNEL_ADVERTISE_ADDR")"}
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
  "access_addr": "$(json_escape "$ACCESS_ADDR")",
  "tunnel_listen_addr": "${DEFAULT_TUNNEL_ADDR}",
  "tunnel_advertise_addr": "$(json_escape "$TUNNEL_ADVERTISE_ADDR")",
  "is_ix": ${IS_IX},
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
LimitNOFILE=1048576
LimitNPROC=65535

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

uninstall_agent() {
  info "Uninstalling OnlyTun Agent..."
  systemctl stop onlytun-agent >/dev/null 2>&1 || true
  systemctl disable onlytun-agent >/dev/null 2>&1 || true
  pkill -x onlytun-agent >/dev/null 2>&1 || true
  rm -f "$SERVICE_PATH" /lib/systemd/system/onlytun-agent.service
  rm -f "$SYSCTL_PATH"
  rm -f "$AGENT_BIN"
  rm -rf "$CONFIG_DIR"
  systemctl daemon-reload >/dev/null 2>&1 || true
  systemctl reset-failed onlytun-agent.service >/dev/null 2>&1 || true
  success "OnlyTun Agent removed. Service, binary, and ${CONFIG_DIR} have been deleted."
}

update_agent() {
  info "Updating OnlyTun Agent..."
  detect_os
  detect_arch
  prepare_dirs
  apply_tcp_tuning

  local tmp_file
  tmp_file="$(mktemp /tmp/onlytun-agent.XXXXXX)" || fail "Failed to create temporary file."
  download_agent_to "$tmp_file"
  chmod +x "$tmp_file" || fail "Failed to chmod downloaded binary."

  if [ -f "$AGENT_BIN" ]; then
    cp "$AGENT_BIN" "${AGENT_BIN}.bak.$(date +%Y%m%d%H%M%S)" || fail "Failed to backup current Agent binary."
  fi
  cp "$tmp_file" "${AGENT_BIN}.new" || fail "Failed to stage updated Agent binary."
  chmod +x "${AGENT_BIN}.new" || fail "Failed to chmod staged Agent binary."
  mv -f "${AGENT_BIN}.new" "$AGENT_BIN" || fail "Failed to install updated Agent binary."
  chmod +x "$AGENT_BIN" || fail "Failed to chmod updated Agent binary."
  rm -f "$tmp_file"

  if [ -f "$SERVICE_PATH" ]; then
    write_service
  fi
  systemctl daemon-reload || fail "systemctl daemon-reload failed."
  systemctl restart onlytun-agent || fail "Failed to restart onlytun-agent."
  sleep 3
  if systemctl is-active --quiet onlytun-agent; then
    success "OnlyTun Agent updated successfully. Config preserved at ${CONFIG_PATH}."
  else
    systemctl status onlytun-agent --no-pager || true
    fail "OnlyTun Agent update finished but service is not active."
  fi
}

current_agent_version() {
  if [ -x "$AGENT_BIN" ]; then
    "$AGENT_BIN" --version 2>/dev/null || printf "unknown"
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

check_agent_version() {
  local current latest answer
  current="$(current_agent_version)"
  latest="$(latest_release_version || true)"
  [ -n "$latest" ] || latest="unknown"

  printf "%bCurrent Agent version:%b %s\n" "$GREEN" "$NC" "$current"
  printf "%bLatest Release version:%b %s\n" "$GREEN" "$NC" "$latest"

  if [ "$current" = "$latest" ]; then
    success "Agent is already up to date."
    return 0
  fi

  if [ -t 0 ]; then
    printf "Update Agent now? [y/N]: "
    read -r answer
    case "$answer" in
      y|Y) update_agent ;;
      *) warn "Update skipped." ;;
    esac
  else
    warn "Run 'bash install.sh --update' to update Agent."
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
      uninstall_agent
      exit 0
      ;;
    update)
      update_agent
      exit 0
      ;;
    check-version)
      check_agent_version
      exit 0
      ;;
  esac

  prompt_if_missing
  validate_install_args
  detect_os
  detect_arch
  prepare_dirs
  apply_tcp_tuning
  download_agent
  register_machine
  write_config
  write_service
  enable_service
  check_service
}

main "$@"
