#!/usr/bin/env sh
set -eu

CONFIG_PATH="${CONFIG_PATH:-/etc/onlytun/cache.json}"
ROLE="ingress"
MACHINE_ID=""
PSK=""
PANEL_URL=""
TOKEN=""
TUNNEL_PORT="19999"

usage() {
  cat <<'EOF'
Usage: install.sh [options]

Options:
  --config PATH         Config output path (default: /etc/onlytun/cache.json)
  --role ROLE           Agent role: ingress or egress (default: ingress)
  --machine-id ID       Machine ID
  --psk HEX             Hex-encoded PSK
  --panel-url URL       Panel URL
  --token TOKEN         Panel token
  --tunnel-port PORT    Egress tunnel listen port (default: 19999)
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --config)
      CONFIG_PATH="$2"
      shift 2
      ;;
    --role)
      ROLE="$2"
      shift 2
      ;;
    --machine-id)
      MACHINE_ID="$2"
      shift 2
      ;;
    --psk)
      PSK="$2"
      shift 2
      ;;
    --panel-url)
      PANEL_URL="$2"
      shift 2
      ;;
    --token)
      TOKEN="$2"
      shift 2
      ;;
    --tunnel-port)
      TUNNEL_PORT="$2"
      shift 2
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

CONFIG_DIR=$(dirname "$CONFIG_PATH")
mkdir -p "$CONFIG_DIR"

cat >"$CONFIG_PATH" <<EOF
{
  "machine_id": "$MACHINE_ID",
  "role": "$ROLE",
  "psk": "$PSK",
  "panel_url": "$PANEL_URL",
  "token": "$TOKEN",
  "tunnel_listen_addr": "0.0.0.0:$TUNNEL_PORT",
  "rules": []
}
EOF
