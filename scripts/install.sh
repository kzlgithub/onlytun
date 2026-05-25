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

usage() {
  cat <<EOF
Usage: bash scripts/install.sh --role ingress|egress --panel http://host:port --token INSTALL_TOKEN [--name MACHINE_NAME]
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
    fail "请使用 root 权限运行此脚本。"
  fi
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "缺少依赖命令: $1"
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
        fail "未知参数: $1"
        ;;
    esac
  done

  [ -n "$ROLE" ] || fail "--role 为必填参数"
  [ -n "$PANEL_URL" ] || fail "--panel 为必填参数"
  [ -n "$INSTALL_TOKEN" ] || fail "--token 为必填参数"

  case "$ROLE" in
    ingress|egress) ;;
    *)
      fail "--role 仅支持 ingress 或 egress"
      ;;
  esac

  PANEL_URL="${PANEL_URL%/}"
  if [ -z "$MACHINE_NAME" ]; then
    MACHINE_NAME="$(hostname 2>/dev/null || true)"
  fi
  [ -n "$MACHINE_NAME" ] || MACHINE_NAME="onlytun-${ROLE}"
}

detect_os() {
  [ -f /etc/os-release ] || fail "无法识别当前操作系统：缺少 /etc/os-release"
  # shellcheck disable=SC1091
  . /etc/os-release

  local os_id="${ID:-}"
  local version_id="${VERSION_ID:-}"
  local major="${version_id%%.*}"

  case "$os_id" in
    ubuntu)
      [ "${major:-0}" -ge 18 ] || fail "仅支持 Ubuntu 18.04 及以上版本"
      ;;
    debian)
      [ "${major:-0}" -ge 10 ] || fail "仅支持 Debian 10 及以上版本"
      ;;
    centos)
      [ "${major:-0}" -ge 7 ] || fail "仅支持 CentOS 7 及以上版本"
      ;;
    rocky)
      [ "${major:-0}" -ge 8 ] || fail "仅支持 Rocky Linux 8 及以上版本"
      ;;
    *)
      fail "当前系统 ${os_id:-unknown} 暂不受支持"
      ;;
  esac

  success "操作系统检测通过: ${PRETTY_NAME:-$os_id}"
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
      fail "当前 CPU 架构 $(uname -m) 暂不受支持，仅支持 amd64 / arm64"
      ;;
  esac

  success "CPU 架构检测通过: ${ARCH}"
}

prepare_dirs() {
  mkdir -p "$CONFIG_DIR" /usr/local/bin || fail "创建目录失败"
  success "目录已准备完成"
}

download_agent() {
  local url="https://github.com/kzlgithub/onlytun/releases/latest/download/onlytun-agent-linux-${ARCH}"
  info "开始下载 Agent 二进制: ${url}"
  curl --retry 3 --retry-delay 2 -fL# "$url" -o "$AGENT_BIN" || fail "下载 Agent 二进制失败，请检查网络或 Release 是否存在"
  chmod +x "$AGENT_BIN" || fail "设置 Agent 执行权限失败"
  success "Agent 二进制已安装到 ${AGENT_BIN}"
}

fetch_public_ip() {
  PUBLIC_IP="$(curl -fsS https://api.ipify.org)" || fail "获取本机公网 IP 失败"
  [ -n "$PUBLIC_IP" ] || fail "获取到的公网 IP 为空"
  success "公网 IP: ${PUBLIC_IP}"
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

  info "向面板注册机器..."
  local response
  response="$(curl -fsS -X POST "${PANEL_URL}/api/agent/register" \
    -H "Authorization: Bearer ${INSTALL_TOKEN}" \
    -H "Content-Type: application/json" \
    -d "$payload")" || fail "注册机器失败，请检查面板地址、安装 token 或网络连接"

  MACHINE_ID="$(printf '%s' "$response" | grep -oE '"machine_id"[[:space:]]*:[[:space:]]*"[^"]+"' | sed 's/.*"machine_id"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')"
  PSK="$(printf '%s' "$response" | grep -oE '"psk"[[:space:]]*:[[:space:]]*"[^"]+"' | sed 's/.*"psk"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')"

  [ -n "${MACHINE_ID:-}" ] || fail "注册响应缺少 machine_id"
  [ -n "${PSK:-}" ] || fail "注册响应缺少 psk"
  success "机器注册成功，Machine ID: ${MACHINE_ID}"
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
  success "配置文件已写入 ${CONFIG_PATH}"
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
  success "systemd 服务文件已写入 ${SERVICE_PATH}"
}

enable_service() {
  systemctl daemon-reload || fail "systemd daemon-reload 执行失败"
  systemctl enable onlytun-agent >/dev/null 2>&1 || fail "启用 onlytun-agent 服务失败"
  systemctl start onlytun-agent || fail "启动 onlytun-agent 服务失败"
  success "onlytun-agent 服务已启用并启动"
}

check_service() {
  info "等待服务启动..."
  sleep 3
  if systemctl is-active --quiet onlytun-agent; then
    success "OnlyTun Agent 安装成功，服务运行正常"
    printf "%b面板地址:%b %s\n" "$GREEN" "$NC" "$PANEL_URL"
    printf "%b配置文件:%b %s\n" "$GREEN" "$NC" "$CONFIG_PATH"
  else
    warn "服务未处于 active 状态，以下是状态输出："
    systemctl status onlytun-agent --no-pager || true
    fail "OnlyTun Agent 安装完成但服务启动失败"
  fi
}

main() {
  require_root
  require_command curl
  require_command systemctl
  require_command grep
  require_command sed

  parse_args "$@"
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
