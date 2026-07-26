# Development Guide

This guide keeps the practical details out of the short README.

## Requirements

- Go 1.23 or newer.
- Docker and Docker Compose for local PostgreSQL and Docker-backed execution.
- Linux `isolate` for production-style sandbox execution.
- PostgreSQL in every environment.

## Local Setup

Start PostgreSQL:

```bash
docker compose up -d postgres
```

Build the local Docker runner image:

```bash
docker compose build runner
```

Run with direct execution:

```bash
env GOCACHE=/tmp/batasd-go-cache \
  go run ./cmd/batasd
```

Run with Docker execution:

```bash
env SANDBOX_DRIVER=docker \
  GOCACHE=/tmp/batasd-go-cache \
  go run ./cmd/batasd
```

Run with isolate on Linux:

```bash
env SANDBOX_DRIVER=isolate \
  GOCACHE=/tmp/batasd-go-cache \
  go run ./cmd/batasd
```

Default API address: `:18080`.

## API Basics

All protected endpoints use:

```http
Authorization: Bearer dev-token
```

Useful endpoints:

- `GET /healthz`
- `GET /v1/auth`
- `GET /v1/languages`
- `GET /v1/languages/{slug}`
- `GET /v1/statuses`
- `GET /v1/config`
- `GET /v1/workers`
- `POST /v1/submissions`
- `GET /v1/submissions`
- `GET /v1/submissions/{token}`
- `GET /v1/submissions/{token}/callbacks`

`GET /v1/languages` returns the full language catalog. Catalog entries with `enabled: false` are discoverable but cannot be submitted until a runner/isolate toolchain and commands are configured for that language version. The catalog covers all 83 Exercism tracks and keeps `node` as a backward-compatible alias for JavaScript execution.

Create a submission:

```bash
curl -X POST http://localhost:18080/v1/submissions \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer dev-token' \
  -d '{"language":"python","source":"print(\"hello\")","input":"","expected_output":"hello"}'
```

Create and briefly wait for completion:

```bash
curl -X POST 'http://localhost:18080/v1/submissions?wait=true' \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer dev-token' \
  -d '{"language":"python","source":"print(\"hello\")","expected_output":"hello"}'
```

`language_version` is optional. When omitted, batasd uses the catalog `default_version`.

`wait=true` is bounded by `SUBMISSION_WAIT_TIMEOUT_MS` and polls every `SUBMISSION_WAIT_POLL_INTERVAL_MS`. If the wait times out first, the API returns `202 Accepted` with the latest queued or processing state.

List recent submissions:

```bash
curl -H 'Authorization: Bearer dev-token' \
  'http://localhost:18080/v1/submissions?limit=20&status=accepted&language=python'
```

The list endpoint defaults to `limit=20` and clamps oversized limits to `100`. If the response includes `pagination.next_before`, pass that token as `before` for the next page.

Field filtering works on `GET /v1/submissions`, `GET /v1/submissions/{token}`, and `POST /v1/submissions?wait=true`:

```bash
curl -H 'Authorization: Bearer dev-token' \
  'http://localhost:18080/v1/submissions/sub_xxxxx?fields=token,status,stdout,time_ms'
```

## Submit Helper

`scripts/submit.sh` reads a source file, detects language by extension, sends it to the API, and optionally polls until terminal status.

```bash
scripts/submit.sh ./main.py
scripts/submit.sh --wait --expected-output "hello" ./main.py
scripts/submit.sh --url http://localhost:18082 --wait ./main.js
scripts/submit.sh --language-version 3.12 --wait ./main.py
scripts/submit.sh --runs 3 --wait ./main.py
scripts/submit.sh --memory-kb 2097152 --max-processes 256 ./main.js
```

Defaults:

- URL: `http://localhost:18080/v1/submissions`
- token: `dev-token`

Override with `--url`, `BATASD_SUBMISSIONS_URL`, `--token`, or `BATASD_TOKEN`.

The helper detects common extensions for the catalog, including `.py`, `.js`, `.c`, `.cpp`, `.go`, `.rs`, `.java`, `.php`, `.rb`, `.ts`, `.cs`, `.fs`, `.kt`, `.lua`, `.r`, `.sql`, `.swift`, `.zig`, and many more. Ambiguous extensions, such as assembly or Pascal variants, may still need `--language`.

JavaScript files detect as the Exercism `javascript` slug. The older `node` slug remains available as an enabled compatibility alias.

Java submissions use one source file named `Main.java` with class `Main`.

Catalog-shaped fixtures under `testdata/e2e/programs/languages/<slug>/<source_file>` cover every catalog entry. Fixtures for disabled languages are placeholders for future toolchain work; submitting one directly returns `422 validation_failed` until that language version is marked `enabled`.

## Additional Files

Submissions may include a base64-encoded zip archive:

```json
"additional_files": {
  "encoding": "zip_base64",
  "content": "<base64-encoded zip archive>"
}
```

Archive paths are extracted into the execution directory. Absolute paths, `..`, backslashes, symlinks, duplicate paths, and source-file overwrites are rejected.

`scripts/submit.sh` can attach a zip:

```bash
scripts/submit.sh --additional-files-zip ./fixtures.zip ./main.py
```

## Callbacks

Start a local receiver:

```bash
scripts/callback-receiver.sh
```

Submit with a callback:

```bash
scripts/submit.sh \
  --callback-url http://127.0.0.1:9000/callback \
  --wait ./main.py
```

The receiver gets a `POST` with the same shape returned by `GET /v1/submissions/{token}`. Delivery is attempted once in the MVP and recorded in `callback_attempts`.

Fetch callback attempts:

```bash
curl -H 'Authorization: Bearer dev-token' \
  http://localhost:18080/v1/submissions/sub_xxxxx/callbacks
```

## Testing

Run Go tests:

```bash
env GOCACHE=/tmp/batasd-go-cache \
  go test ./...
```

Let Go use its default module cache for normal local development. If you set `GOMODCACHE` to a disposable directory such as `/tmp/batasd-go-mod` and see missing source files under that path, delete that custom module cache and let Go download modules again.

Run the full e2e fixture suite against a running Docker or isolate API:

```bash
scripts/e2e.sh --url http://localhost:18080
```

Run only selected language cases:

```bash
scripts/e2e.sh --url http://localhost:18080 --lang c
scripts/e2e.sh --url http://localhost:18080 --lang c, java, php
scripts/e2e.sh --url http://localhost:18080 --lang=javascript,node
```

When `--lang` is set, scenario cases such as wrong answer, compile error, and limits are skipped.

The e2e suite uses `scripts/submit.sh` and fixtures in `testdata/e2e/programs`. It covers accepted submissions for every enabled Exercism runner language, the `node` compatibility alias, wrong answer, runtime error, compile error, wall-time limit, memory exhaustion, output limit/truncation, repeated runs, and `additional_files`.

Note: Python memory exhaustion can surface differently by sandbox. Docker usually reports `memory_limit_exceeded`; isolate may let Python raise `MemoryError` and return `runtime_error`. The e2e suite accepts both as the same memory-exhaustion behavior.

The OpenAPI 3.1 contract lives at `api/openapi.yaml` and is updated manually. `go test ./...` catches common contract drift: every chi method/path must be documented in OpenAPI, every OpenAPI method/path must exist in chi, public DTO JSON fields must match OpenAPI component properties, and status codes must match the OpenAPI enum. Detailed validation semantics such as descriptions, examples, constraints, and status-specific response variants still need review unless the project moves to OpenAPI code generation.

## Configuration

Copy `.env.example` if you want local overrides:

```bash
cp .env.example .env
```

Important defaults:

- `HTTP_ADDR=:18080`
- `HTTP_MAX_BODY_BYTES=26214400`
- `DATABASE_URL=postgres://sandbox:sandbox@localhost:5432/sandbox?sslmode=disable`
- `AUTHN_TOKENS=dev-token`
- `SUBMISSION_WAIT_TIMEOUT_MS=10000`
- `SUBMISSION_WAIT_POLL_INTERVAL_MS=100`
- `SANDBOX_DRIVER=direct`
- `SANDBOX_DOCKER_IMAGE=batasd-runner:local`
- `SANDBOX_ISOLATE_BINARY=isolate`
- `SANDBOX_ISOLATE_BOX_ID_START=0`
- `SANDBOX_ISOLATE_BOX_ID_COUNT=16`
- `SANDBOX_ISOLATE_CGROUP=true`
- `CALLBACK_TIMEOUT_MS=5000`
- `MAX_LIMIT_CPU_TIME_MS=15000`
- `MAX_LIMIT_WALL_TIME_MS=30000`
- `MAX_LIMIT_MEMORY_KB=2097152`
- `MAX_LIMIT_MAX_PROCESSES=256`
- `MAX_LIMIT_MAX_FILE_KB=65536`
- `MAX_LIMIT_RUNS=20`

Language versions may set default limits in `internal/language/catalog/languages.json`. Submission-provided limits can lower or raise per-request limits, but they cannot exceed `MAX_LIMIT_*` values.

## Runtime Behavior

- `stdout`, `stderr`, and `compile_output` are capped by `max_output_kb`.
- Truncation sets `stdout_truncated`, `stderr_truncated`, or `compile_output_truncated`.
- `limits.runs > 1` compiles once, runs repeatedly, and stops on the first failed run.
- Successful repeated runs return the latest output, average time fields, and maximum `memory_kb`.
- Startup recovery resets unfinished `queued` or `processing` submissions back to `queued` and enqueues them.

## Sandbox Notes

- `direct` is development-only and best effort.
- Docker is the portable local/staging driver.
- `isolate` is the production-style Linux driver.
- Raw Docker alone is easy to misconfigure for hostile code; keep sandbox-specific logic in `internal/sandbox/*`.
- The Docker runner image is built from `docker/runner/Dockerfile`.
- The isolate driver exposes a constrained environment. It intentionally sets `PATH`, Go cache env, Rust env, and Python bytecode env.
