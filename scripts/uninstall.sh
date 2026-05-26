#!/bin/bash

set -u

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

TARGET=""
ASSUME_YES="false"
KEEP_CONFIG="false"

usage() {
  cat <<EOF
Usage:
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

  [ -n "$TARGET" ] || { usage; exit 1; }
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

main() {
  require_root
  parse_args "$@"

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
  esac

  systemctl daemon-reload >/dev/null 2>&1 || true
  remove_config_if_requested
  success "Uninstall completed."
}

main "$@"
