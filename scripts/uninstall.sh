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
  bash uninstall.sh --install agent|panel
  bash uninstall.sh --uninstall agent|panel|all [--yes] [--keep-config]
  bash uninstall.sh --update agent|panel|all
  bash uninstall.sh --check-version agent|panel|all
  bash uninstall.sh agent [--yes] [--keep-config]
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
      --update)
        ACTION="update"
        shift
        ;;
      --check-version|--version-check)
        ACTION="check-version"
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
      4) ACTION="check-version"; TARGET="agent"; break ;;
      *) warn "请输入 1、2、3 或 4。" ;;
    esac
  done
}

prompt_target_if_missing() {
  local purpose="$1"
  if [ -n "$TARGET" ]; then
    return
  fi
  [ -t 0 ] || fail "${purpose} target is required in non-interactive mode."

  while true; do
    printf "请选择目标：\n"
    printf "  1. Agent\n"
    printf "  2. 面板\n"
    printf "  3. 全部\n"
    printf "请输入序号 [1-3]: "
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

run_remote_script() {
  local script_name="$1"
  local action_flag="$2"
  command -v curl >/dev/null 2>&1 || fail "Missing required command: curl"
  bash <(curl -fsSL "https://raw.githubusercontent.com/kzlgithub/onlytun/main/scripts/${script_name}") "$action_flag"
}

run_install_flow() {
  prompt_target_if_missing "Install"
  case "$TARGET" in
    panel) run_remote_script "install-panel.sh" "--install" ;;
    agent) run_remote_script "install.sh" "--install" ;;
    *) fail "Install target only supports agent or panel." ;;
  esac
}

run_update_flow() {
  prompt_target_if_missing "Update"
  case "$TARGET" in
    agent)
      if [ -f /root/install.sh ]; then
        bash /root/install.sh --update
      else
        run_remote_script "install.sh" "--update"
      fi
      ;;
    panel)
      if [ -f /root/install-panel.sh ]; then
        bash /root/install-panel.sh --update
      else
        run_remote_script "install-panel.sh" "--update"
      fi
      ;;
    all)
      TARGET="agent"; run_update_flow
      TARGET="panel"; run_update_flow
      ;;
    *) fail "Update target only supports agent, panel, or all." ;;
  esac
}

run_check_version_flow() {
  prompt_target_if_missing "Version check"
  case "$TARGET" in
    agent)
      if [ -f /root/install.sh ]; then
        bash /root/install.sh --check-version
      else
        run_remote_script "install.sh" "--check-version"
      fi
      ;;
    panel)
      if [ -f /root/install-panel.sh ]; then
        bash /root/install-panel.sh --check-version
      else
        run_remote_script "install-panel.sh" "--check-version"
      fi
      ;;
    all)
      TARGET="agent"; run_check_version_flow
      TARGET="panel"; run_check_version_flow
      ;;
    *) fail "Version check target only supports agent, panel, or all." ;;
  esac
}

run_uninstall_flow() {
  prompt_target_if_missing "Uninstall"
  case "$TARGET" in
    agent) remove_agent ;;
    panel) remove_panel ;;
    all)
      remove_agent
      remove_panel
      ;;
    *) fail "Uninstall target only supports agent, panel, or all." ;;
  esac

  systemctl daemon-reload >/dev/null 2>&1 || true
  remove_config_if_requested
  success "Uninstall completed."
}

main() {
  require_root
  parse_args "$@"
  prompt_action_if_missing

  case "$ACTION" in
    install) run_install_flow ;;
    uninstall) run_uninstall_flow ;;
    update) run_update_flow ;;
    check-version) run_check_version_flow ;;
    *) fail "Unknown action: ${ACTION}" ;;
  esac
}

main "$@"
