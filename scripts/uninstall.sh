#!/bin/bash

set -u

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

TARGET=""
ACTION=""
ASSUME_YES="false"
KEEP_CONFIG="false"

usage() {
  cat <<EOF
Usage:
  bash uninstall.sh
  bash uninstall.sh --install
  bash uninstall.sh --uninstall agent [--yes] [--keep-config]
  bash uninstall.sh agent [--yes] [--keep-config]
  bash uninstall.sh panel [--yes] [--keep-config]
  bash uninstall.sh all   [--yes] [--keep-config]
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
    fail "Please run this script as root."
  fi
}

parse_args() {
  while [ $# -gt 0 ]; do
    case "$1" in
      agent|panel|all)
        TARGET="$1"
        shift
        ;;
      --install)
        ACTION="install"
        shift
        ;;
      --uninstall)
        ACTION="uninstall"
        shift
        ;;
      --yes|-y)
        ASSUME_YES="true"
        shift
        ;;
      --keep-config)
        KEEP_CONFIG="true"
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

  if [ -z "$ACTION" ] && [ -n "$TARGET" ]; then
    ACTION="uninstall"
  fi
}

prompt_action_if_missing() {
  if [ -n "$ACTION" ]; then
    return
  fi
  [ -t 0 ] || { usage; exit 1; }

  while true; do
    printf "请选择操作：1. 安装  2. 卸载 [1/2]: "
    read -r choice
    case "$choice" in
      1) ACTION="install"; break ;;
      2) ACTION="uninstall"; break ;;
      *) warn "请输入 1 或 2。" ;;
    esac
  done
}

prompt_install_target_if_missing() {
  if [ -n "$TARGET" ] && [ "$TARGET" != "all" ]; then
    return
  fi
  [ -t 0 ] || fail "Install target is required in non-interactive mode."

  while true; do
    printf "请选择安装类型：1. 面板  2. 隧道机Agent [1/2]: "
    read -r choice
    case "$choice" in
      1) TARGET="panel"; break ;;
      2) TARGET="agent"; break ;;
      *) warn "请输入 1 或 2。" ;;
    esac
  done
}

prompt_uninstall_target_if_missing() {
  if [ -n "$TARGET" ]; then
    return
  fi
  [ -t 0 ] || fail "Uninstall target is required in non-interactive mode."

  while true; do
    printf "请选择卸载类型：1. Agent  2. 面板  3. 全部 [1/2/3]: "
    read -r choice
    case "$choice" in
      1) TARGET="agent"; break ;;
      2) TARGET="panel"; break ;;
      3) TARGET="all"; break ;;
      *) warn "请输入 1、2 或 3。" ;;
    esac
  done
}

stop_and_disable_service() {
  local service_name="$1"
  systemctl stop "$service_name" >/dev/null 2>&1 || true
  systemctl disable "$service_name" >/dev/null 2>&1 || true
  systemctl reset-failed "${service_name}.service" >/dev/null 2>&1 || true
  rm -f "/etc/systemd/system/${service_name}.service" "/lib/systemd/system/${service_name}.service"
  pkill -x "$service_name" >/dev/null 2>&1 || true
  success "${service_name} service removed."
}

remove_agent() {
  stop_and_disable_service "onlytun-agent"
  rm -f /usr/local/bin/onlytun-agent
  success "Agent binary removed."
}

remove_panel() {
  stop_and_disable_service "onlytun-panel"
  rm -f /usr/local/bin/onlytun-panel
  success "Panel binary removed."
}

remove_config_if_requested() {
  if [ "$KEEP_CONFIG" = "true" ]; then
    warn "Keeping /etc/onlytun because --keep-config was provided."
    return
  fi

  if [ ! -d /etc/onlytun ]; then
    warn "/etc/onlytun does not exist. Skipping config cleanup."
    return
  fi

  if [ "$ASSUME_YES" != "true" ]; then
    printf "Delete /etc/onlytun and all cached config/database files? [y/N]: "
    read -r answer
    if [ "$answer" != "y" ] && [ "$answer" != "Y" ]; then
      warn "Keeping /etc/onlytun."
      return
    fi
  fi

  rm -rf /etc/onlytun || fail "Failed to remove /etc/onlytun."
  success "/etc/onlytun removed."
}

run_install_flow() {
  command -v curl >/dev/null 2>&1 || fail "Missing required command: curl"
  prompt_install_target_if_missing
  case "$TARGET" in
    panel)
      bash <(curl -fsSL https://raw.githubusercontent.com/kzlgithub/onlytun/main/scripts/install-panel.sh) --install
      ;;
    agent)
      bash <(curl -fsSL https://raw.githubusercontent.com/kzlgithub/onlytun/main/scripts/install.sh) --install
      ;;
    *)
      fail "Install target only supports agent or panel."
      ;;
  esac
}

run_uninstall_flow() {
  prompt_uninstall_target_if_missing
  case "$TARGET" in
    agent)
      remove_agent
      ;;
    panel)
      remove_panel
      ;;
    all)
      remove_agent
      remove_panel
      ;;
    *)
      fail "Uninstall target only supports agent, panel, or all."
      ;;
  esac

  systemctl daemon-reload >/dev/null 2>&1 || true
  remove_config_if_requested
  success "Uninstall completed."
}

main() {
  require_root
  parse_args "$@"
  prompt_action_if_missing

  if [ "$ACTION" = "install" ]; then
    run_install_flow
  else
    run_uninstall_flow
  fi
}

main "$@"
