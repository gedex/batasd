#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage:
  scripts/submit.sh [options] <program-file>

Options:
  --url URL                   API base URL or submissions URL.
                              Default: http://localhost:18080/v1/submissions
  --token TOKEN               Bearer token. Default: BATASD_TOKEN or dev-token
  --language SLUG             Override language detection.
  --input VALUE               Stdin string for the submission.
  --input-file PATH           Read stdin string from file.
  --expected-output VALUE     Expected stdout string.
  --expected-output-file PATH Read expected stdout string from file.
  --callback-url URL          Callback URL to POST the completed result to.
  --wait                      Poll until the submission leaves queued/processing.
  --interval SECONDS          Poll interval for --wait. Default: 1
  -h, --help                  Show this help.

Environment:
  BATASD_SUBMISSIONS_URL      Overrides the submissions endpoint.
  BATASD_TOKEN                Overrides the bearer token.
  BATASD_CALLBACK_URL         Adds a callback URL to the submission payload.
  BATASD_POLL_INTERVAL        Overrides the --wait poll interval.

Examples:
  scripts/submit.sh ./main.py
  scripts/submit.sh --url http://localhost:18082 ./main.js
  scripts/submit.sh --callback-url http://127.0.0.1:9000/callback ./main.py
  scripts/submit.sh --wait --expected-output "hello" ./main.py
USAGE
}

die() {
  printf 'submit.sh: %s\n' "$*" >&2
  exit 1
}

need_command() {
  command -v "$1" >/dev/null 2>&1 || die "$1 is required"
}

normalize_submissions_url() {
  case "$1" in
    */v1/submissions)
      printf '%s\n' "$1"
      ;;
    */v1/submissions/)
      printf '%s\n' "${1%/}"
      ;;
    *)
      printf '%s/v1/submissions\n' "${1%/}"
      ;;
  esac
}

detect_language() {
  local path="$1"
  local ext
  ext="$(printf '%s' "${path##*.}" | tr '[:upper:]' '[:lower:]')"

  case "$ext" in
    py)
      printf 'python-3.12\n'
      ;;
    js | mjs | cjs)
      printf 'node-22\n'
      ;;
    *)
      die "cannot detect language from .$ext; pass --language"
      ;;
  esac
}

extract_token() {
  python3 -c 'import json, sys
try:
    print(json.load(sys.stdin).get("token", ""))
except json.JSONDecodeError:
    print("")'
}

extract_status_code() {
  python3 -c 'import json, sys
try:
    data = json.load(sys.stdin)
except json.JSONDecodeError:
    print("")
    raise SystemExit

status = data.get("status") or {}
print(status.get("code", ""))'
}

request() {
  local method="$1"
  local url="$2"
  local body_path="${3:-}"
  local response_path="$4"
  local http_status

  if [[ "$method" == "POST" ]]; then
    http_status="$(
      curl -sS -o "$response_path" -w '%{http_code}' \
        -X POST "$url" \
        -H 'Content-Type: application/json' \
        -H "Authorization: Bearer $auth_token" \
        --data-binary "@$body_path"
    )"
  else
    http_status="$(
      curl -sS -o "$response_path" -w '%{http_code}' \
        -H "Authorization: Bearer $auth_token" \
        "$url"
    )"
  fi

  if [[ ! "$http_status" =~ ^2 ]]; then
    cat "$response_path" >&2
    printf '\nsubmit.sh: request failed with HTTP %s\n' "$http_status" >&2
    return 1
  fi
}

submissions_url="${BATASD_SUBMISSIONS_URL:-http://localhost:18080/v1/submissions}"
auth_token="${BATASD_TOKEN:-dev-token}"
language=""
input=""
input_file=""
expected_output=""
expected_output_file=""
expected_output_set=false
callback_url="${BATASD_CALLBACK_URL:-}"
wait=false
interval="${BATASD_POLL_INTERVAL:-1}"
program_path=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --url)
      [[ $# -ge 2 ]] || die "--url requires a value"
      submissions_url="$2"
      shift 2
      ;;
    --token)
      [[ $# -ge 2 ]] || die "--token requires a value"
      auth_token="$2"
      shift 2
      ;;
    --language)
      [[ $# -ge 2 ]] || die "--language requires a value"
      language="$2"
      shift 2
      ;;
    --input)
      [[ $# -ge 2 ]] || die "--input requires a value"
      input="$2"
      input_file=""
      shift 2
      ;;
    --input-file)
      [[ $# -ge 2 ]] || die "--input-file requires a value"
      [[ -f "$2" ]] || die "input file not found: $2"
      [[ -r "$2" ]] || die "input file is not readable: $2"
      input_file="$2"
      input=""
      shift 2
      ;;
    --expected-output)
      [[ $# -ge 2 ]] || die "--expected-output requires a value"
      expected_output="$2"
      expected_output_file=""
      expected_output_set=true
      shift 2
      ;;
    --expected-output-file)
      [[ $# -ge 2 ]] || die "--expected-output-file requires a value"
      [[ -f "$2" ]] || die "expected output file not found: $2"
      [[ -r "$2" ]] || die "expected output file is not readable: $2"
      expected_output_file="$2"
      expected_output=""
      expected_output_set=true
      shift 2
      ;;
    --callback-url)
      [[ $# -ge 2 ]] || die "--callback-url requires a value"
      callback_url="$2"
      shift 2
      ;;
    --wait)
      wait=true
      shift
      ;;
    --interval)
      [[ $# -ge 2 ]] || die "--interval requires a value"
      interval="$2"
      shift 2
      ;;
    -h | --help)
      usage
      exit 0
      ;;
    --)
      shift
      break
      ;;
    -*)
      die "unknown option: $1"
      ;;
    *)
      if [[ -n "$program_path" ]]; then
        die "only one program file can be submitted"
      fi
      program_path="$1"
      shift
      ;;
  esac
done

if [[ $# -gt 0 ]]; then
  [[ $# -eq 1 ]] || die "only one program file can be submitted"
  if [[ -n "$program_path" ]]; then
    die "only one program file can be submitted"
  fi
  program_path="$1"
fi

[[ -n "$program_path" ]] || die "program file is required"
[[ -f "$program_path" ]] || die "program file not found: $program_path"
[[ -r "$program_path" ]] || die "program file is not readable: $program_path"

need_command curl
need_command python3

submissions_url="$(normalize_submissions_url "$submissions_url")"
if [[ -z "$language" ]]; then
  language="$(detect_language "$program_path")"
fi

payload_path="$(mktemp)"
response_path="$(mktemp)"
trap 'rm -f "$payload_path" "$response_path"' EXIT

BATASD_SUBMIT_INPUT="$input" BATASD_SUBMIT_EXPECTED_OUTPUT="$expected_output" BATASD_SUBMIT_CALLBACK_URL="$callback_url" python3 - \
  "$program_path" \
  "$language" \
  "$input_file" \
  "$expected_output_set" \
  "$expected_output_file" >"$payload_path" <<'PY'
import json
import os
import pathlib
import sys

program_path = pathlib.Path(sys.argv[1])
language = sys.argv[2]
input_file = sys.argv[3]
expected_output_set = sys.argv[4] == "true"
expected_output_file = sys.argv[5]

stdin = os.environ.get("BATASD_SUBMIT_INPUT", "")
if input_file:
    stdin = pathlib.Path(input_file).read_text()

payload = {
    "language": language,
    "source": program_path.read_text(),
    "input": stdin,
}

if expected_output_set:
    expected_output = os.environ.get("BATASD_SUBMIT_EXPECTED_OUTPUT", "")
    if expected_output_file:
        expected_output = pathlib.Path(expected_output_file).read_text()
    payload["expected_output"] = expected_output

callback_url = os.environ.get("BATASD_SUBMIT_CALLBACK_URL", "")
if callback_url:
    payload["callback"] = {"url": callback_url}

json.dump(payload, sys.stdout)
print()
PY

request POST "$submissions_url" "$payload_path" "$response_path"
cat "$response_path"
printf '\n'

if [[ "$wait" != true ]]; then
  exit 0
fi

token="$(cat "$response_path" | extract_token)"
[[ -n "$token" ]] || die "response did not contain a token"

result_url="${submissions_url%/}/$token"
while true; do
  sleep "$interval"
  request GET "$result_url" "" "$response_path"
  status_code="$(cat "$response_path" | extract_status_code)"
  case "$status_code" in
    queued | processing)
      continue
      ;;
    *)
      cat "$response_path"
      printf '\n'
      break
      ;;
  esac
done
