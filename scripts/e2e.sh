#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage:
  scripts/e2e.sh [options]

Options:
  --url URL             API base URL or submissions URL.
                        Default: BATASD_SUBMISSIONS_URL or http://localhost:18080/v1/submissions
  --token TOKEN         Bearer token. Default: BATASD_TOKEN or dev-token
  --lang LANGS          Run only selected language cases. Accepts comma or
                        whitespace separated slugs, for example:
                        --lang c
                        --lang c, java, php
                        --lang=c,javascript,node
  --interval SECONDS    Poll interval passed to submit.sh. Default: 0.2
  -h, --help            Show this help.

The API must already be running. These checks are intended for Docker or isolate
mode because they validate real sandbox statuses and limits.
USAGE
}

die() {
  printf 'e2e.sh: %s\n' "$*" >&2
  exit 1
}

need_command() {
  command -v "$1" >/dev/null 2>&1 || die "$1 is required"
}

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/.." && pwd)"
submit_script="$script_dir/submit.sh"
languages_dir="$repo_root/testdata/e2e/programs/languages"
scenarios_dir="$repo_root/testdata/e2e/programs/scenarios"

submissions_url="${BATASD_SUBMISSIONS_URL:-http://localhost:18080/v1/submissions}"
auth_token="${BATASD_TOKEN:-dev-token}"
interval="0.2"
selected_languages=()
matched_languages=()

add_language_filter() {
  local raw="$1"
  local part
  raw="${raw//,/ }"
  for part in $raw; do
    [[ -n "$part" ]] || continue
    selected_languages+=("$part")
  done
}

language_filter_active() {
  [[ "${#selected_languages[@]}" -gt 0 ]]
}

language_selected() {
  local slug="$1"
  local selected
  if ! language_filter_active; then
    return 0
  fi
  for selected in "${selected_languages[@]}"; do
    if [[ "$selected" == "$slug" ]]; then
      return 0
    fi
  done
  return 1
}

remember_matched_language() {
  local slug="$1"
  local matched
  for matched in "${matched_languages[@]}"; do
    if [[ "$matched" == "$slug" ]]; then
      return
    fi
  done
  matched_languages+=("$slug")
}

language_was_matched() {
  local slug="$1"
  local matched
  for matched in "${matched_languages[@]}"; do
    if [[ "$matched" == "$slug" ]]; then
      return 0
    fi
  done
  return 1
}

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
    --lang)
      shift
      [[ $# -gt 0 ]] || die "--lang requires a value"
      while [[ $# -gt 0 && "$1" != --* ]]; do
        add_language_filter "$1"
        shift
      done
      ;;
    --lang=*)
      add_language_filter "${1#--lang=}"
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
    *)
      die "unknown option: $1"
      ;;
  esac
done

need_command curl
need_command python3
[[ -x "$submit_script" ]] || die "submit script is not executable: $submit_script"
[[ -d "$languages_dir" ]] || die "language fixtures not found: $languages_dir"
[[ -d "$scenarios_dir" ]] || die "scenario fixtures not found: $scenarios_dir"

work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT

additional_zip="$work_dir/additional-files.zip"
python3 - "$additional_zip" <<'PY'
import sys
import zipfile

with zipfile.ZipFile(sys.argv[1], "w", compression=zipfile.ZIP_DEFLATED) as archive:
    archive.writestr("fixtures/message.txt", "hello from additional files\n")
PY

validate_response() {
  local label="$1"
  local output="$2"

  BATASD_E2E_OUTPUT="$output" python3 - "$label" <<'PY'
import json
import os
import sys

label = sys.argv[1]
raw = os.environ["BATASD_E2E_OUTPUT"]
decoder = json.JSONDecoder()
objects = []
index = 0

while index < len(raw):
    while index < len(raw) and raw[index].isspace():
        index += 1
    if index >= len(raw):
        break
    try:
        obj, index = decoder.raw_decode(raw, index)
    except json.JSONDecodeError as exc:
        print(f"{label}: failed to parse submit.sh JSON output: {exc}", file=sys.stderr)
        print(raw, file=sys.stderr)
        raise SystemExit(1)
    objects.append(obj)

if not objects:
    print(f"{label}: submit.sh returned no JSON", file=sys.stderr)
    raise SystemExit(1)

response = objects[-1]
status = response.get("status") or {}
limits = response.get("limits") or {}
problems = []

def env(name):
    return os.environ.get(name, "")

def env_bool(name):
    value = env(name)
    if value == "":
        return None
    return value == "true"

def add(problem):
    problems.append(problem)

expected_status = env("BATASD_E2E_EXPECT_STATUS")
if expected_status and status.get("code") != expected_status:
    add(f"status.code={status.get('code')!r}, want {expected_status!r}")

expected_language = env("BATASD_E2E_EXPECT_LANGUAGE")
if expected_language and response.get("language") != expected_language:
    add(f"language={response.get('language')!r}, want {expected_language!r}")

expected_version = env("BATASD_E2E_EXPECT_VERSION")
if expected_version and response.get("language_version") != expected_version:
    add(f"language_version={response.get('language_version')!r}, want {expected_version!r}")

if env_bool("BATASD_E2E_EXPECT_STDOUT_SET"):
    expected_stdout = env("BATASD_E2E_EXPECT_STDOUT") + "\n"
    if response.get("stdout") != expected_stdout:
        add(f"stdout={response.get('stdout')!r}, want {expected_stdout!r}")

stdout = response.get("stdout") or ""
stderr = response.get("stderr") or ""
compile_output = response.get("compile_output") or ""
message = response.get("message") or ""

if env_bool("BATASD_E2E_REQUIRE_STDOUT") and stdout == "":
    add("stdout is empty")

forbidden_stdout = env("BATASD_E2E_STDOUT_NOT_CONTAINS")
if forbidden_stdout and forbidden_stdout in stdout:
    add(f"stdout contains forbidden text {forbidden_stdout!r}")

stderr_contains = env("BATASD_E2E_STDERR_CONTAINS")
if stderr_contains and stderr_contains not in stderr:
    add(f"stderr={stderr!r}, want to contain {stderr_contains!r}")

message_contains = env("BATASD_E2E_MESSAGE_CONTAINS")
if message_contains and message_contains not in message:
    add(f"message={message!r}, want to contain {message_contains!r}")

compile_output_contains = env("BATASD_E2E_COMPILE_OUTPUT_CONTAINS")
if compile_output_contains and compile_output_contains not in compile_output:
    add(f"compile_output={compile_output!r}, want to contain {compile_output_contains!r}")

if env_bool("BATASD_E2E_REQUIRE_COMPILE_OUTPUT") and compile_output == "":
    add("compile_output is empty")

if env_bool("BATASD_E2E_EXPECT_MEMORY_EXHAUSTION"):
    status_code = status.get("code")
    if status_code == "memory_limit_exceeded":
        pass
    elif status_code == "runtime_error" and "MemoryError" in stderr:
        pass
    else:
        add(
            "memory exhaustion result was "
            f"status={status_code!r}, stderr={stderr!r}; "
            "want memory_limit_exceeded or Python MemoryError runtime_error"
        )

stdout_truncated = env_bool("BATASD_E2E_EXPECT_STDOUT_TRUNCATED")
if stdout_truncated is not None and response.get("stdout_truncated") is not stdout_truncated:
    add(f"stdout_truncated={response.get('stdout_truncated')!r}, want {stdout_truncated!r}")

expected_runs = env("BATASD_E2E_EXPECT_RUNS")
if expected_runs and limits.get("runs") != int(expected_runs):
    add(f"limits.runs={limits.get('runs')!r}, want {expected_runs}")

expected_exit_code = env("BATASD_E2E_EXPECT_EXIT_CODE")
if expected_exit_code and response.get("exit_code") != int(expected_exit_code):
    add(f"exit_code={response.get('exit_code')!r}, want {expected_exit_code}")

if expected_status == "accepted":
    for field in ("stderr", "compile_output", "message"):
        if response.get(field) not in (None, ""):
            add(f"{field}={response.get(field)!r}, want empty")
    for field in ("stdout_truncated", "stderr_truncated", "compile_output_truncated"):
        if response.get(field) is not False:
            add(f"{field}={response.get(field)!r}, want false")

if problems:
    print(f"{label}: validation failed", file=sys.stderr)
    for problem in problems:
        print(f"  - {problem}", file=sys.stderr)
    print(raw, file=sys.stderr)
    raise SystemExit(1)
PY
}

run_case() {
  local label="$1"
  local source_path="$2"
  shift 2

  local expect_status=""
  local expect_language=""
  local expect_version=""
  local expect_stdout=""
  local expect_stdout_set=false
  local require_stdout=false
  local stdout_not_contains=""
  local stderr_contains=""
  local message_contains=""
  local compile_output_contains=""
  local require_compile_output=false
  local expect_memory_exhaustion=false
  local expect_stdout_truncated=""
  local expect_runs=""
  local expect_exit_code=""
  local submit_language=""
  local submit_args=()

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --status)
        [[ $# -ge 2 ]] || die "--status requires a value"
        expect_status="$2"
        shift 2
        ;;
      --language)
        [[ $# -ge 2 ]] || die "--language requires a value"
        expect_language="$2"
        shift 2
        ;;
      --version)
        [[ $# -ge 2 ]] || die "--version requires a value"
        expect_version="$2"
        shift 2
        ;;
      --stdout)
        [[ $# -ge 2 ]] || die "--stdout requires a value"
        expect_stdout="$2"
        expect_stdout_set=true
        shift 2
        ;;
      --require-stdout)
        require_stdout=true
        shift
        ;;
      --stdout-not-contains)
        [[ $# -ge 2 ]] || die "--stdout-not-contains requires a value"
        stdout_not_contains="$2"
        shift 2
        ;;
      --stderr-contains)
        [[ $# -ge 2 ]] || die "--stderr-contains requires a value"
        stderr_contains="$2"
        shift 2
        ;;
      --message-contains)
        [[ $# -ge 2 ]] || die "--message-contains requires a value"
        message_contains="$2"
        shift 2
        ;;
      --compile-output-contains)
        [[ $# -ge 2 ]] || die "--compile-output-contains requires a value"
        compile_output_contains="$2"
        shift 2
        ;;
      --require-compile-output)
        require_compile_output=true
        shift
        ;;
      --memory-exhaustion)
        expect_memory_exhaustion=true
        shift
        ;;
      --stdout-truncated)
        [[ $# -ge 2 ]] || die "--stdout-truncated requires a value"
        expect_stdout_truncated="$2"
        shift 2
        ;;
      --runs-result)
        [[ $# -ge 2 ]] || die "--runs-result requires a value"
        expect_runs="$2"
        shift 2
        ;;
      --exit-code)
        [[ $# -ge 2 ]] || die "--exit-code requires a value"
        expect_exit_code="$2"
        shift 2
        ;;
      --submit-language)
        [[ $# -ge 2 ]] || die "--submit-language requires a value"
        submit_language="$2"
        shift 2
        ;;
      --input | --expected-output | --additional-files-zip | --cpu-time-ms | --cpu-extra-time-ms | --wall-time-ms | --memory-kb | --stack-kb | --max-processes | --max-output-kb | --max-file-kb | --runs | --network)
        [[ $# -ge 2 ]] || die "$1 requires a value"
        submit_args+=("$1" "$2")
        shift 2
        ;;
      *)
        die "unknown case option for $label: $1"
        ;;
    esac
  done

  if [[ -n "$submit_language" ]]; then
    submit_args=("--language" "$submit_language" "${submit_args[@]}")
  fi

  printf 'e2e %-28s ' "$label"
  local output
  if ! output="$("$submit_script" \
    --url "$submissions_url" \
    --token "$auth_token" \
    --wait \
    --interval "$interval" \
    "${submit_args[@]}" \
    "$source_path" 2>&1)"; then
    printf 'FAIL\n'
    printf '%s\n' "$output" >&2
    return 1
  fi

  if ! BATASD_E2E_EXPECT_STATUS="$expect_status" \
    BATASD_E2E_EXPECT_LANGUAGE="$expect_language" \
    BATASD_E2E_EXPECT_VERSION="$expect_version" \
    BATASD_E2E_EXPECT_STDOUT_SET="$expect_stdout_set" \
    BATASD_E2E_EXPECT_STDOUT="$expect_stdout" \
    BATASD_E2E_REQUIRE_STDOUT="$require_stdout" \
    BATASD_E2E_STDOUT_NOT_CONTAINS="$stdout_not_contains" \
    BATASD_E2E_STDERR_CONTAINS="$stderr_contains" \
    BATASD_E2E_MESSAGE_CONTAINS="$message_contains" \
    BATASD_E2E_COMPILE_OUTPUT_CONTAINS="$compile_output_contains" \
    BATASD_E2E_REQUIRE_COMPILE_OUTPUT="$require_compile_output" \
    BATASD_E2E_EXPECT_MEMORY_EXHAUSTION="$expect_memory_exhaustion" \
    BATASD_E2E_EXPECT_STDOUT_TRUNCATED="$expect_stdout_truncated" \
    BATASD_E2E_EXPECT_RUNS="$expect_runs" \
    BATASD_E2E_EXPECT_EXIT_CODE="$expect_exit_code" \
    validate_response "$label" "$output"; then
    printf 'FAIL\n'
    return 1
  fi
  printf 'ok\n'
}

run_language_case() {
  local slug="$1"
  shift

  if ! language_selected "$slug"; then
    return
  fi

  remember_matched_language "$slug"
  if ! run_case "$@"; then
    failures=$((failures + 1))
  fi
}

failures=0

run_language_case "python" "language python" "$languages_dir/hello.py" \
  --status accepted --language python --version 3.12 \
  --input Akeda --expected-output "hello from python: Akeda" --stdout "hello from python: Akeda"
run_language_case "javascript" "language javascript" "$languages_dir/hello.js" \
  --status accepted --language javascript --version 22 \
  --input Akeda --expected-output "hello from javascript: Akeda" --stdout "hello from javascript: Akeda"
run_language_case "node" "language node alias" "$languages_dir/hello-node.js" \
  --status accepted --language node --version 22 --submit-language node \
  --input Akeda --expected-output "hello from node alias: Akeda" --stdout "hello from node alias: Akeda"
run_language_case "c" "language c" "$languages_dir/hello.c" \
  --status accepted --language c --version gcc \
  --input Akeda --expected-output "hello from c: Akeda" --stdout "hello from c: Akeda"
run_language_case "cpp" "language cpp" "$languages_dir/hello.cpp" \
  --status accepted --language cpp --version gcc \
  --input Akeda --expected-output "hello from cpp: Akeda" --stdout "hello from cpp: Akeda"
run_language_case "go" "language go" "$languages_dir/hello.go" \
  --status accepted --language go --version 1.24 \
  --input Akeda --expected-output "hello from go: Akeda" --stdout "hello from go: Akeda"
run_language_case "rust" "language rust" "$languages_dir/hello.rs" \
  --status accepted --language rust --version 1.88 \
  --input Akeda --expected-output "hello from rust: Akeda" --stdout "hello from rust: Akeda"
run_language_case "java" "language java" "$languages_dir/Main.java" \
  --status accepted --language java --version 21 \
  --input Akeda --expected-output "hello from java: Akeda" --stdout "hello from java: Akeda"
run_language_case "php" "language php" "$languages_dir/hello.php" \
  --status accepted --language php --version 8.3 \
  --input Akeda --expected-output "hello from php: Akeda" --stdout "hello from php: Akeda"

if language_filter_active; then
  unmatched_languages=()
  for selected_language in "${selected_languages[@]}"; do
    if ! language_was_matched "$selected_language"; then
      unmatched_languages+=("$selected_language")
    fi
  done
  if [[ "${#unmatched_languages[@]}" -gt 0 ]]; then
    die "no e2e language case for: ${unmatched_languages[*]}"
  fi
else
  run_case "wrong answer" "$scenarios_dir/wrong-answer.py" \
    --status wrong_answer --language python --version 3.12 \
    --expected-output "expected output" --stdout "actual output" \
    --message-contains "output did not match" || failures=$((failures + 1))
  run_case "runtime error" "$scenarios_dir/runtime-error.py" \
    --status runtime_error --language python --version 3.12 \
    --stderr-contains "runtime failure path" --exit-code 7 \
    --message-contains "non-zero status" || failures=$((failures + 1))
  run_case "compile error" "$scenarios_dir/compile-error.c" \
    --status compilation_error --language c --version gcc \
    --require-compile-output --compile-output-contains "error" \
    --message-contains "compilation failed" || failures=$((failures + 1))
  run_case "wall time limit" "$scenarios_dir/slow.py" \
    --status time_limit_exceeded --language python --version 3.12 \
    --wall-time-ms 500 --message-contains "time limit exceeded" || failures=$((failures + 1))
  run_case "memory limit" "$scenarios_dir/memory.py" \
    --language python --version 3.12 \
    --memory-kb 64000 --memory-exhaustion || failures=$((failures + 1))
  run_case "output limit" "$scenarios_dir/output-flood.py" \
    --status output_limit_exceeded --language python --version 3.12 \
    --max-output-kb 1 --stdout-truncated true --require-stdout \
    --stdout-not-contains "BATASD_OUTPUT_SENTINEL" \
    --message-contains "output limit exceeded" || failures=$((failures + 1))
  run_case "repeated runs" "$scenarios_dir/runs.py" \
    --status accepted --language python --version 3.12 \
    --runs 3 --runs-result 3 --expected-output "hello runs" --stdout "hello runs" || failures=$((failures + 1))
  run_case "additional files" "$scenarios_dir/additional-files.py" \
    --status accepted --language python --version 3.12 \
    --additional-files-zip "$additional_zip" \
    --expected-output "hello from additional files" --stdout "hello from additional files" || failures=$((failures + 1))
fi

if [[ "$failures" -gt 0 ]]; then
  die "$failures e2e case(s) failed"
fi

printf 'e2e all cases ok\n'
