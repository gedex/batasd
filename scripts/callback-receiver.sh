#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage:
  scripts/callback-receiver.sh [options]

Options:
  --host HOST     Host to bind. Default: BATASD_CALLBACK_HOST or 127.0.0.1
  --port PORT     Port to bind. Default: BATASD_CALLBACK_PORT or 9000
  -h, --help      Show this help.

Environment:
  BATASD_CALLBACK_HOST   Overrides the bind host.
  BATASD_CALLBACK_PORT   Overrides the bind port.

Examples:
  scripts/callback-receiver.sh
  scripts/callback-receiver.sh --port 9001
USAGE
}

die() {
  printf 'callback-receiver.sh: %s\n' "$*" >&2
  exit 1
}

need_command() {
  command -v "$1" >/dev/null 2>&1 || die "$1 is required"
}

host="${BATASD_CALLBACK_HOST:-127.0.0.1}"
port="${BATASD_CALLBACK_PORT:-9000}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --host)
      [[ $# -ge 2 ]] || die "--host requires a value"
      host="$2"
      shift 2
      ;;
    --port)
      [[ $# -ge 2 ]] || die "--port requires a value"
      port="$2"
      shift 2
      ;;
    -h | --help)
      usage
      exit 0
      ;;
    *)
      die "unknown option: $1"
      ;;
  esac
done

[[ "$port" =~ ^[0-9]+$ ]] || die "--port must be an integer"
need_command python3

BATASD_CALLBACK_HOST="$host" BATASD_CALLBACK_PORT="$port" python3 - <<'PY'
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
import sys


HOST = os.environ["BATASD_CALLBACK_HOST"]
PORT = int(os.environ["BATASD_CALLBACK_PORT"])


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/healthz":
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(b'{"ok":true}\n')
            return

        self.send_response(404)
        self.end_headers()

    def do_POST(self):
        length = int(self.headers.get("Content-Length") or "0")
        body = self.rfile.read(length)

        print("\n--- callback received ---")
        print(f"{self.command} {self.path}")
        for header in ("Content-Type", "User-Agent"):
            value = self.headers.get(header)
            if value:
                print(f"{header}: {value}")

        if body:
            text = body.decode("utf-8", "replace")
            try:
                payload = json.loads(text)
            except json.JSONDecodeError:
                print(text)
            else:
                print(json.dumps(payload, indent=2, sort_keys=True))
        else:
            print("(empty body)")

        sys.stdout.flush()
        self.send_response(204)
        self.end_headers()

    def log_message(self, format, *args):
        return


server = ThreadingHTTPServer((HOST, PORT), Handler)
print(f"Listening for callbacks on http://{HOST}:{PORT}/callback")
print("Press Ctrl+C to stop.")
sys.stdout.flush()

try:
    server.serve_forever()
except KeyboardInterrupt:
    print("\nStopping callback receiver.")
finally:
    server.server_close()
PY
