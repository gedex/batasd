# batasd

Code execution service in Go with pluggable sandbox backends.

## Status

This project is in early development. The current implementation includes:

- HTTP API built with chi.
- PostgreSQL-backed submission storage.
- Embedded database migrations.
- Embedded language catalog.
- OpenAPI 3.1 contract with route drift checks.
- Token-based API authentication.
- In-memory queue for local development.
- Direct local execution worker for development.
- Initial submission create and fetch endpoints.
- Base64 zip `additional_files` extraction.
- Completion callbacks with attempt logging.
- Startup recovery for unfinished queued/processing submissions.

Sandbox drivers are being built incrementally. The current implementation supports the `direct` and Docker development drivers, plus an initial Linux `isolate` driver for production-style execution.
The `direct` driver is development-only; it uses best-effort time/output limits and terminates the submitted process group on cancellation.

## Requirements

- Go 1.23 or newer.
- Docker and Docker Compose.

## Local Development

Start PostgreSQL:

```bash
docker compose up -d postgres
```

Build the local Docker runner image when using `SANDBOX_DRIVER=docker`:

```bash
docker compose build runner
```

Run the API:

```bash
env GOCACHE=/private/tmp/codeexec-go-cache GOMODCACHE=/private/tmp/codeexec-go-mod go run ./cmd/batasd
```

The API listens on `:18080` by default.

List languages:

```bash
curl -H 'Authorization: Bearer dev-token' http://localhost:18080/v1/languages
```

The default catalog includes `python`, `node`, `c`, `cpp`, `go`, `rust`, `java`, and `php`. Each language has one or more versions and a `default_version`.

Create a submission:

```bash
curl -X POST http://localhost:18080/v1/submissions \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer dev-token' \
  -d '{"language":"python","source":"print(\"hello\")","input":"","expected_output":"hello"}'
```

Create a submission and wait briefly for completion:

```bash
curl -X POST 'http://localhost:18080/v1/submissions?wait=true' \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer dev-token' \
  -d '{"language":"python","source":"print(\"hello\")","input":"","expected_output":"hello"}'
```

`language_version` is optional. When omitted, batasd uses the catalog `default_version` for that language.

`wait=true` is bounded by `SUBMISSION_WAIT_TIMEOUT_MS` and polls every `SUBMISSION_WAIT_POLL_INTERVAL_MS`. If the wait times out before the submission reaches a terminal status, the API returns `202 Accepted` with the latest queued or processing state.

Fetch the result with the returned token:

```bash
curl -H 'Authorization: Bearer dev-token' http://localhost:18080/v1/submissions/sub_xxxxx
```

Fetch only selected submission fields:

```bash
curl -H 'Authorization: Bearer dev-token' \
  'http://localhost:18080/v1/submissions/sub_xxxxx?fields=token,status,stdout,time_ms'
```

`fields` also works on `GET /v1/submissions` and `POST /v1/submissions?wait=true`. On list responses, `fields` filters each submission item and keeps `pagination`.

List recent submissions:

```bash
curl -H 'Authorization: Bearer dev-token' \
  'http://localhost:18080/v1/submissions?limit=20&status=accepted&language=python'
```

If the response includes `pagination.next_before`, pass that token as `before` to fetch the next page.
The list endpoint defaults to `limit=20` and clamps oversized limits to `100`.

Create a submission with a callback:

```bash
scripts/callback-receiver.sh
```

```bash
curl -X POST http://localhost:18080/v1/submissions \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer dev-token' \
  -d '{"language":"python","source":"print(\"hello\")","expected_output":"hello","callback":{"url":"http://127.0.0.1:9000/callback"}}'
```

The callback receiver gets a `POST` with the same JSON shape returned by `GET /v1/submissions/{token}` after execution finishes. Delivery is attempted once in the MVP and recorded in `callback_attempts`.

Fetch callback attempts for a submission:

```bash
curl -H 'Authorization: Bearer dev-token' \
  http://localhost:18080/v1/submissions/sub_xxxxx/callbacks
```

Inspect queue and worker activity:

```bash
curl -H 'Authorization: Bearer dev-token' http://localhost:18080/v1/workers
```

Submit a source file with language detection:

```bash
scripts/submit.sh ./main.py
scripts/submit.sh --url http://localhost:18082 --wait ./main.js
scripts/submit.sh --wait ./main.c
scripts/submit.sh --wait ./main.cpp
scripts/submit.sh --wait ./main.go
scripts/submit.sh --wait ./main.rs
scripts/submit.sh --wait ./Main.java
scripts/submit.sh --wait ./main.php
scripts/submit.sh --additional-files-zip ./fixtures.zip ./main.py
scripts/submit.sh --memory-kb 2097152 --max-processes 256 ./main.js
scripts/submit.sh --runs 3 --wait ./main.py
scripts/submit.sh --callback-url http://127.0.0.1:9000/callback --wait ./main.py
scripts/submit.sh --language-version 3.12 --wait ./main.py
```

The helper defaults to `http://localhost:18080/v1/submissions` and `Authorization: Bearer dev-token`. Override those with `--url`, `BATASD_SUBMISSIONS_URL`, `--token`, or `BATASD_TOKEN`. Override catalog defaults with `--language-version` or `BATASD_LANGUAGE_VERSION`. Add callbacks with `--callback-url` or `BATASD_CALLBACK_URL`. Set per-submission limits with flags such as `--cpu-time-ms`, `--wall-time-ms`, `--memory-kb`, `--max-processes`, and `--max-output-kb`.

Java submissions use a single source file named `Main.java` and should define class `Main`.

Attach supporting files with:

```json
"additional_files": {
  "encoding": "zip_base64",
  "content": "<base64-encoded zip archive>"
}
```

Archive paths are extracted into the execution directory. Absolute paths, `..`, backslashes, symlinks, duplicate paths, and source-file overwrites are rejected.

Run tests:

```bash
env GOCACHE=/private/tmp/codeexec-go-cache GOMODCACHE=/private/tmp/codeexec-go-mod go test ./...
```

The OpenAPI 3.1 contract lives at `api/openapi.yaml`. The test suite checks that documented method/path pairs match the chi router.

Smoke-test all catalog languages against a running API:

```bash
scripts/smoke-languages.sh --url http://localhost:18080
```

The smoke script submits `python`, `node`, `c`, `cpp`, `go`, `rust`, `java`, and `php`, then checks that each response is accepted, uses the expected resolved `language_version`, has the expected stdout, and has no stderr or compile output.

Run the end-to-end fixture suite against a running API:

```bash
scripts/e2e.sh --url http://localhost:18080
```

The e2e suite uses `scripts/submit.sh` and repo-owned fixtures under `testdata/e2e/programs`. It covers accepted submissions for every catalog language plus wrong answer, runtime error, compile error, wall-time limit, memory limit, output limit/truncation, repeated runs, and `additional_files`. It is intended for Docker or isolate mode because it validates real sandbox statuses and limits.

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

The language catalog can set per-language-version default limits. `node`, `go`, `rust`, and `java` currently use larger memory or process profiles than the global defaults because their runtimes or compilers are heavier under sandbox limits. Submission-provided limits may lower or raise the per-request limits, but they cannot exceed `MAX_LIMIT_*` configuration values.

When stdout or stderr exceeds `max_output_kb`, batasd keeps only the capped output, sets `stdout_truncated`, `stderr_truncated`, or `compile_output_truncated`, and stops the running command early.

When `limits.runs` is greater than 1, batasd compiles once, executes the run command repeatedly, and fails the submission on the first failed run. Successful repeated runs return the latest stdout/stderr, average `time_ms` and `wall_time_ms`, and maximum `memory_kb`.

On startup, batasd resets unfinished `queued` or `processing` submissions back to `queued` and enqueues them for workers. This keeps the in-memory queue usable locally while preserving restart recovery through PostgreSQL.

For Docker-backed local execution:

```bash
env SANDBOX_DRIVER=docker GOCACHE=/private/tmp/codeexec-go-cache GOMODCACHE=/private/tmp/codeexec-go-mod go run ./cmd/batasd
```

For isolate-backed Linux execution:

```bash
env SANDBOX_DRIVER=isolate GOCACHE=/tmp/batasd-go-cache GOMODCACHE=/tmp/batasd-go-mod go run ./cmd/batasd
```

## License

MIT
