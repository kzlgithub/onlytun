#!/bin/bash

set -u

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

PANEL_PORT="8080"
PANEL_PASSWORD=""
PANEL_BIN="/usr/local/bin/onlytun-panel"
CONFIG_DIR="/etc/onlytun"
SERVICE_PATH="/etc/systemd/system/onlytun-panel.service"

usage() {
  cat <<EOF
Usage: bash scripts/install-panel.sh [--port 8080] [--password YOUR_PASSWORD]
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
      --port)
        PANEL_PORT="${2:-}"
        shift 2
        ;;
      --password)
        PANEL_PASSWORD="${2:-}"
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

  printf '%s' "$PANEL_PORT" | grep -Eq '^[0-9]+$' || fail "--port 必须为数字"
  [ "$PANEL_PORT" -ge 1 ] && [ "$PANEL_PORT" -le 65535 ] || fail "--port 必须在 1-65535 之间"
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

download_panel() {
  local url="https://github.com/onlytun/onlytun/releases/latest/download/onlytun-panel-linux-${ARCH}"
  info "开始下载面板二进制: ${url}"
  curl -fL# "$url" -o "$PANEL_BIN" || fail "下载面板二进制失败，请检查网络或 Release 是否存在"
  chmod +x "$PANEL_BIN" || fail "设置面板执行权限失败"
  success "面板二进制已安装到 ${PANEL_BIN}"
}

prepare_dirs() {
  mkdir -p "$CONFIG_DIR" /usr/local/bin || fail "创建目录失败"
  success "目录已准备完成"
}

prompt_password() {
  if [ -n "$PANEL_PASSWORD" ]; then
    return
  fi

  while true; do
    printf "请输入面板管理密码: "
    stty -echo
    read -r first
    stty echo
    printf "\n请再次输入面板管理密码: "
    stty -echo
    read -r second
    stty echo
    printf "\n"

    [ -n "$first" ] || warn "密码不能为空"
    [ -n "$first" ] || continue

    if [ "$first" != "$second" ]; then
      warn "两次输入的密码不一致，请重新输入"
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
  success "systemd 服务文件已写入 ${SERVICE_PATH}"
}

enable_service() {
  systemctl daemon-reload || fail "systemd daemon-reload 执行失败"
  systemctl enable onlytun-panel >/dev/null 2>&1 || fail "启用 onlytun-panel 服务失败"
  systemctl start onlytun-panel || fail "启动 onlytun-panel 服务失败"
  success "onlytun-panel 服务已启用并启动"
}

fetch_public_ip() {
  PUBLIC_IP="$(curl -fsS https://api.ipify.org)" || fail "获取本机公网 IP 失败"
  [ -n "$PUBLIC_IP" ] || fail "获取到的公网 IP 为空"
}

check_service() {
  info "等待服务启动..."
  sleep 3
  if systemctl is-active --quiet onlytun-panel; then
    fetch_public_ip
    success "OnlyTun 面板安装成功，服务运行正常"
    printf "%b访问地址:%b http://%s:%s\n" "$GREEN" "$NC" "$PUBLIC_IP" "$PANEL_PORT"
  else
    warn "服务未处于 active 状态，以下是状态输出："
    systemctl status onlytun-panel --no-pager || true
    fail "OnlyTun 面板安装完成但服务启动失败"
  fi
}

main() {
  require_root
  require_command curl
  require_command systemctl
  require_command grep
  require_command sed
  require_command stty

  parse_args "$@"
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
