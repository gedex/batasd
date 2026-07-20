#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage:
  scripts/smoke-languages.sh [options]

Options:
  --url URL             API base URL or submissions URL.
                        Default: BATASD_SUBMISSIONS_URL or http://localhost:18080/v1/submissions
  --token TOKEN         Bearer token. Default: BATASD_TOKEN or dev-token
  --interval SECONDS    Poll interval passed to submit.sh. Default: 0.2
  -h, --help            Show this help.
USAGE
}

die() {
  printf 'smoke-languages.sh: %s\n' "$*" >&2
  exit 1
}

need_command() {
  command -v "$1" >/dev/null 2>&1 || die "$1 is required"
}

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
submit_script="$script_dir/submit.sh"

submissions_url="${BATASD_SUBMISSIONS_URL:-http://localhost:18080/v1/submissions}"
auth_token="${BATASD_TOKEN:-dev-token}"
interval="0.2"

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

work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT

cat >"$work_dir/main.py" <<'PY'
import sys

name = sys.stdin.read().strip()
print(f"hello from python: {name}")
PY

cat >"$work_dir/main.js" <<'JS'
const fs = require("node:fs");

const name = fs.readFileSync(0, "utf8").trim();
console.log(`hello from node: ${name}`);
JS

cat >"$work_dir/main.c" <<'C'
#include <stdio.h>

int main(void) {
    char name[128] = {0};
    if (fgets(name, sizeof(name), stdin) == NULL) {
        return 1;
    }
    for (char *p = name; *p != '\0'; p++) {
        if (*p == '\n' || *p == '\r') {
            *p = '\0';
            break;
        }
    }
    printf("hello from c: %s\n", name);
    return 0;
}
C

cat >"$work_dir/main.cpp" <<'CPP'
#include <iostream>
#include <string>

int main() {
    std::string name;
    if (!std::getline(std::cin, name)) {
        return 1;
    }
    std::cout << "hello from cpp: " << name << '\n';
    return 0;
}
CPP

cat >"$work_dir/main.go" <<'GO'
package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

func main() {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		panic(err)
	}
	fmt.Printf("hello from go: %s\n", strings.TrimSpace(string(data)))
}
GO

cat >"$work_dir/main.rs" <<'RS'
use std::io::{self, Read};

fn main() {
    let mut input = String::new();
    io::stdin().read_to_string(&mut input).unwrap();
    println!("hello from rust: {}", input.trim());
}
RS

cat >"$work_dir/Main.java" <<'JAVA'
public class Main {
    public static void main(String[] args) throws Exception {
        byte[] data = System.in.readAllBytes();
        String name = new String(data).trim();
        System.out.println("hello from java: " + name);
    }
}
JAVA

cat >"$work_dir/main.php" <<'PHP'
<?php

$name = trim(stream_get_contents(STDIN));
echo "hello from php: {$name}\n";
PHP

validate_response() {
  local label="$1"
  local language="$2"
  local version="$3"
  local expected="$4"
  local output="$5"

  BATASD_SMOKE_OUTPUT="$output" python3 - "$label" "$language" "$version" "$expected" <<'PY'
import json
import os
import sys

label, language, version, expected = sys.argv[1:5]
raw = os.environ["BATASD_SMOKE_OUTPUT"]
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
problems = []

if status.get("code") != "accepted":
    problems.append(f"status.code={status.get('code')!r}, want 'accepted'")
if response.get("language") != language:
    problems.append(f"language={response.get('language')!r}, want {language!r}")
if response.get("language_version") != version:
    problems.append(f"language_version={response.get('language_version')!r}, want {version!r}")
if response.get("stdout") != expected + "\n":
    problems.append(f"stdout={response.get('stdout')!r}, want {(expected + chr(10))!r}")
for field in ("stderr", "compile_output", "message"):
    if response.get(field) not in (None, ""):
        problems.append(f"{field}={response.get(field)!r}, want empty")
for field in ("stdout_truncated", "stderr_truncated", "compile_output_truncated"):
    if response.get(field) is not False:
        problems.append(f"{field}={response.get(field)!r}, want false")

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
  local language="$2"
  local version="$3"
  local expected="$4"
  local source_path="$5"
  local output

  printf 'smoke %-10s ' "$label"
  if ! output="$("$submit_script" \
    --url "$submissions_url" \
    --token "$auth_token" \
    --wait \
    --interval "$interval" \
    --input "Akeda" \
    --expected-output "$expected" \
    "$source_path" 2>&1)"; then
    printf 'FAIL\n'
    printf '%s\n' "$output" >&2
    return 1
  fi
  if ! validate_response "$label" "$language" "$version" "$expected" "$output"; then
    printf 'FAIL\n'
    return 1
  fi
  printf 'ok\n'
}

failures=0
run_case "python" "python" "3.12" "hello from python: Akeda" "$work_dir/main.py" || failures=$((failures + 1))
run_case "node" "node" "22" "hello from node: Akeda" "$work_dir/main.js" || failures=$((failures + 1))
run_case "c" "c" "gcc" "hello from c: Akeda" "$work_dir/main.c" || failures=$((failures + 1))
run_case "cpp" "cpp" "gcc" "hello from cpp: Akeda" "$work_dir/main.cpp" || failures=$((failures + 1))
run_case "go" "go" "1.24" "hello from go: Akeda" "$work_dir/main.go" || failures=$((failures + 1))
run_case "rust" "rust" "1.88" "hello from rust: Akeda" "$work_dir/main.rs" || failures=$((failures + 1))
run_case "java" "java" "21" "hello from java: Akeda" "$work_dir/Main.java" || failures=$((failures + 1))
run_case "php" "php" "8.3" "hello from php: Akeda" "$work_dir/main.php" || failures=$((failures + 1))

if [[ "$failures" -gt 0 ]]; then
  die "$failures smoke case(s) failed"
fi

printf 'smoke all languages ok\n'
