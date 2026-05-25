#!/bin/bash

set -u

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

usage() {
  cat <<EOF
Usage:
  bash scripts/uninstall.sh agent
  bash scripts/uninstall.sh panel
EOF
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

stop_and_disable_service() {
  local service_name="$1"
  if systemctl list-unit-files | grep -q "^${service_name}\.service"; then
    systemctl stop "${service_name}" >/dev/null 2>&1 || warn "停止 ${service_name} 服务失败，继续执行"
    systemctl disable "${service_name}" >/dev/null 2>&1 || warn "禁用 ${service_name} 服务失败，继续执行"
    rm -f "/etc/systemd/system/${service_name}.service"
    systemctl daemon-reload >/dev/null 2>&1 || warn "systemd daemon-reload 执行失败，继续执行"
    success "${service_name} 服务已停止并禁用"
  else
    warn "未找到 ${service_name} 服务，跳过 systemd 清理"
  fi
}

remove_file_if_exists() {
  local target="$1"
  if [ -e "$target" ]; then
    rm -f "$target" || fail "删除文件失败: $target"
    success "已删除 ${target}"
  else
    warn "文件不存在，跳过: $target"
  fi
}

maybe_remove_config() {
  if [ ! -d /etc/onlytun ]; then
    warn "/etc/onlytun 配置目录不存在，跳过"
    return
  fi

  printf "是否删除 /etc/onlytun/ 配置目录？输入 y 删除，其它任意键保留 [y/N]: "
  read -r answer
  if [ "$answer" = "y" ] || [ "$answer" = "Y" ]; then
    rm -rf /etc/onlytun || fail "删除 /etc/onlytun 配置目录失败"
    success "配置目录 /etc/onlytun 已删除"
  else
    warn "已保留配置目录 /etc/onlytun"
  fi
}

main() {
  require_root

  [ $# -eq 1 ] || { usage; exit 1; }

  case "$1" in
    agent)
      stop_and_disable_service "onlytun-agent"
      remove_file_if_exists "/usr/local/bin/onlytun-agent"
      ;;
    panel)
      stop_and_disable_service "onlytun-panel"
      remove_file_if_exists "/usr/local/bin/onlytun-panel"
      ;;
    *)
      usage
      exit 1
      ;;
  esac

  maybe_remove_config
  success "卸载完成"
}

main "$@"
