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
  --language-version VERSION  Override the catalog default language version.
  --input VALUE               Stdin string for the submission.
  --input-file PATH           Read stdin string from file.
  --expected-output VALUE     Expected stdout string.
  --expected-output-file PATH Read expected stdout string from file.
  --additional-files-zip PATH Attach a zip archive as additional_files.
  --cpu-time-ms VALUE         Set limits.cpu_time_ms.
  --cpu-extra-time-ms VALUE   Set limits.cpu_extra_time_ms.
  --wall-time-ms VALUE        Set limits.wall_time_ms.
  --memory-kb VALUE           Set limits.memory_kb.
  --stack-kb VALUE            Set limits.stack_kb.
  --max-processes VALUE       Set limits.max_processes.
  --max-output-kb VALUE       Set limits.max_output_kb.
  --max-file-kb VALUE         Set limits.max_file_kb.
  --runs VALUE                Set limits.runs.
  --network true|false        Set limits.network.
  --callback-url URL          Callback URL to POST the completed result to.
  --wait                      Poll until the submission leaves queued/processing.
  --interval SECONDS          Poll interval for --wait. Default: 1
  -h, --help                  Show this help.

Environment:
  BATASD_SUBMISSIONS_URL      Overrides the submissions endpoint.
  BATASD_TOKEN                Overrides the bearer token.
  BATASD_LANGUAGE_VERSION     Adds language_version to the submission payload.
  BATASD_CALLBACK_URL         Adds a callback URL to the submission payload.
  BATASD_POLL_INTERVAL        Overrides the --wait poll interval.

Examples:
  scripts/submit.sh ./main.py
  scripts/submit.sh --url http://localhost:18082 ./main.js
  scripts/submit.sh --additional-files-zip ./fixtures.zip ./main.py
  scripts/submit.sh --memory-kb 2097152 --max-processes 256 ./main.js
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

require_integer() {
  [[ "$2" =~ ^[0-9]+$ ]] || die "$1 must be a non-negative integer"
}

require_bool() {
  case "$2" in
    true | false)
      ;;
    *)
      die "$1 must be true or false"
      ;;
  esac
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
    c)
      printf 'c\n'
      ;;
    cc | cpp | cxx)
      printf 'cpp\n'
      ;;
    go)
      printf 'go\n'
      ;;
    java)
      printf 'java\n'
      ;;
    php)
      printf 'php\n'
      ;;
    py)
      printf 'python\n'
      ;;
    rs)
      printf 'rust\n'
      ;;
    js | mjs | cjs)
      printf 'node\n'
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
language_version="${BATASD_LANGUAGE_VERSION:-}"
input=""
input_file=""
expected_output=""
expected_output_file=""
expected_output_set=false
additional_files_zip=""
limit_cpu_time_ms=""
limit_cpu_extra_time_ms=""
limit_wall_time_ms=""
limit_memory_kb=""
limit_stack_kb=""
limit_max_processes=""
limit_max_output_kb=""
limit_max_file_kb=""
limit_runs=""
limit_network=""
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
    --language-version)
      [[ $# -ge 2 ]] || die "--language-version requires a value"
      language_version="$2"
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
    --additional-files-zip)
      [[ $# -ge 2 ]] || die "--additional-files-zip requires a value"
      [[ -f "$2" ]] || die "additional files zip not found: $2"
      [[ -r "$2" ]] || die "additional files zip is not readable: $2"
      additional_files_zip="$2"
      shift 2
      ;;
    --cpu-time-ms)
      [[ $# -ge 2 ]] || die "--cpu-time-ms requires a value"
      require_integer "--cpu-time-ms" "$2"
      limit_cpu_time_ms="$2"
      shift 2
      ;;
    --cpu-extra-time-ms)
      [[ $# -ge 2 ]] || die "--cpu-extra-time-ms requires a value"
      require_integer "--cpu-extra-time-ms" "$2"
      limit_cpu_extra_time_ms="$2"
      shift 2
      ;;
    --wall-time-ms)
      [[ $# -ge 2 ]] || die "--wall-time-ms requires a value"
      require_integer "--wall-time-ms" "$2"
      limit_wall_time_ms="$2"
      shift 2
      ;;
    --memory-kb)
      [[ $# -ge 2 ]] || die "--memory-kb requires a value"
      require_integer "--memory-kb" "$2"
      limit_memory_kb="$2"
      shift 2
      ;;
    --stack-kb)
      [[ $# -ge 2 ]] || die "--stack-kb requires a value"
      require_integer "--stack-kb" "$2"
      limit_stack_kb="$2"
      shift 2
      ;;
    --max-processes)
      [[ $# -ge 2 ]] || die "--max-processes requires a value"
      require_integer "--max-processes" "$2"
      limit_max_processes="$2"
      shift 2
      ;;
    --max-output-kb)
      [[ $# -ge 2 ]] || die "--max-output-kb requires a value"
      require_integer "--max-output-kb" "$2"
      limit_max_output_kb="$2"
      shift 2
      ;;
    --max-file-kb)
      [[ $# -ge 2 ]] || die "--max-file-kb requires a value"
      require_integer "--max-file-kb" "$2"
      limit_max_file_kb="$2"
      shift 2
      ;;
    --runs)
      [[ $# -ge 2 ]] || die "--runs requires a value"
      require_integer "--runs" "$2"
      limit_runs="$2"
      shift 2
      ;;
    --network)
      [[ $# -ge 2 ]] || die "--network requires a value"
      require_bool "--network" "$2"
      limit_network="$2"
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

BATASD_SUBMIT_INPUT="$input" \
  BATASD_SUBMIT_LANGUAGE_VERSION="$language_version" \
  BATASD_SUBMIT_EXPECTED_OUTPUT="$expected_output" \
  BATASD_SUBMIT_CALLBACK_URL="$callback_url" \
  BATASD_SUBMIT_LIMIT_CPU_TIME_MS="$limit_cpu_time_ms" \
  BATASD_SUBMIT_LIMIT_CPU_EXTRA_TIME_MS="$limit_cpu_extra_time_ms" \
  BATASD_SUBMIT_LIMIT_WALL_TIME_MS="$limit_wall_time_ms" \
  BATASD_SUBMIT_LIMIT_MEMORY_KB="$limit_memory_kb" \
  BATASD_SUBMIT_LIMIT_STACK_KB="$limit_stack_kb" \
  BATASD_SUBMIT_LIMIT_MAX_PROCESSES="$limit_max_processes" \
  BATASD_SUBMIT_LIMIT_MAX_OUTPUT_KB="$limit_max_output_kb" \
  BATASD_SUBMIT_LIMIT_MAX_FILE_KB="$limit_max_file_kb" \
  BATASD_SUBMIT_LIMIT_RUNS="$limit_runs" \
  BATASD_SUBMIT_LIMIT_NETWORK="$limit_network" \
  python3 - \
  "$program_path" \
  "$language" \
  "$input_file" \
  "$expected_output_set" \
  "$expected_output_file" \
  "$additional_files_zip" >"$payload_path" <<'PY'
import base64
import json
import os
import pathlib
import sys

program_path = pathlib.Path(sys.argv[1])
language = sys.argv[2]
input_file = sys.argv[3]
expected_output_set = sys.argv[4] == "true"
expected_output_file = sys.argv[5]
additional_files_zip = sys.argv[6]

stdin = os.environ.get("BATASD_SUBMIT_INPUT", "")
if input_file:
    stdin = pathlib.Path(input_file).read_text()

payload = {
    "language": language,
    "source": program_path.read_text(),
    "input": stdin,
}

language_version = os.environ.get("BATASD_SUBMIT_LANGUAGE_VERSION", "")
if language_version:
    payload["language_version"] = language_version

if expected_output_set:
    expected_output = os.environ.get("BATASD_SUBMIT_EXPECTED_OUTPUT", "")
    if expected_output_file:
        expected_output = pathlib.Path(expected_output_file).read_text()
    payload["expected_output"] = expected_output

if additional_files_zip:
    payload["additional_files"] = {
        "encoding": "zip_base64",
        "content": base64.b64encode(pathlib.Path(additional_files_zip).read_bytes()).decode("ascii"),
    }

limits = {}
for env_key, json_key in (
    ("BATASD_SUBMIT_LIMIT_CPU_TIME_MS", "cpu_time_ms"),
    ("BATASD_SUBMIT_LIMIT_CPU_EXTRA_TIME_MS", "cpu_extra_time_ms"),
    ("BATASD_SUBMIT_LIMIT_WALL_TIME_MS", "wall_time_ms"),
    ("BATASD_SUBMIT_LIMIT_MEMORY_KB", "memory_kb"),
    ("BATASD_SUBMIT_LIMIT_STACK_KB", "stack_kb"),
    ("BATASD_SUBMIT_LIMIT_MAX_PROCESSES", "max_processes"),
    ("BATASD_SUBMIT_LIMIT_MAX_OUTPUT_KB", "max_output_kb"),
    ("BATASD_SUBMIT_LIMIT_MAX_FILE_KB", "max_file_kb"),
    ("BATASD_SUBMIT_LIMIT_RUNS", "runs"),
):
    value = os.environ.get(env_key, "")
    if value:
        limits[json_key] = int(value)

network = os.environ.get("BATASD_SUBMIT_LIMIT_NETWORK", "")
if network:
    limits["network"] = network == "true"

if limits:
    payload["limits"] = limits

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
