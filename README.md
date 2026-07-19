# batasd

Code execution service in Go with pluggable sandbox backends.

## Status

This project is in early development. The current implementation includes:

- HTTP API built with chi.
- PostgreSQL-backed submission storage.
- Embedded database migrations.
- Embedded language catalog.
- Token-based API authentication.
- In-memory queue for local development.
- Direct local execution worker for development.
- Initial submission create and fetch endpoints.
- Base64 zip `additional_files` extraction.
- Completion callbacks with attempt logging.
- Startup recovery for unfinished queued/processing submissions.

Sandbox drivers are being built incrementally. The current implementation supports the `direct` and Docker development drivers, plus an initial Linux `isolate` driver for production-style execution.

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

Create a submission:

```bash
curl -X POST http://localhost:18080/v1/submissions \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer dev-token' \
  -d '{"language":"python-3.12","source":"print(\"hello\")","input":"","expected_output":"hello"}'
```

Fetch the result with the returned token:

```bash
curl -H 'Authorization: Bearer dev-token' http://localhost:18080/v1/submissions/sub_xxxxx
```

Create a submission with a callback:

```bash
scripts/callback-receiver.sh
```

```bash
curl -X POST http://localhost:18080/v1/submissions \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer dev-token' \
  -d '{"language":"python-3.12","source":"print(\"hello\")","expected_output":"hello","callback":{"url":"http://127.0.0.1:9000/callback"}}'
```

The callback receiver gets a `POST` with the same JSON shape returned by `GET /v1/submissions/{token}` after execution finishes. Delivery is attempted once in the MVP and recorded in `callback_attempts`.

Fetch callback attempts for a submission:

```bash
curl -H 'Authorization: Bearer dev-token' \
  http://localhost:18080/v1/submissions/sub_xxxxx/callbacks
```

Submit a source file with language detection:

```bash
scripts/submit.sh ./main.py
scripts/submit.sh --url http://localhost:18082 --wait ./main.js
scripts/submit.sh --additional-files-zip ./fixtures.zip ./main.py
scripts/submit.sh --memory-kb 2097152 --max-processes 256 ./main.js
scripts/submit.sh --callback-url http://127.0.0.1:9000/callback --wait ./main.py
```

The helper defaults to `http://localhost:18080/v1/submissions` and `Authorization: Bearer dev-token`. Override those with `--url`, `BATASD_SUBMISSIONS_URL`, `--token`, or `BATASD_TOKEN`. Add callbacks with `--callback-url` or `BATASD_CALLBACK_URL`. Set per-submission limits with flags such as `--cpu-time-ms`, `--wall-time-ms`, `--memory-kb`, `--max-processes`, and `--max-output-kb`.

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

## Configuration

Copy `.env.example` if you want local overrides:

```bash
cp .env.example .env
```

Important defaults:

- `HTTP_ADDR=:18080`
- `DATABASE_URL=postgres://sandbox:sandbox@localhost:5432/sandbox?sslmode=disable`
- `AUTHN_TOKENS=dev-token`
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

The language catalog can set per-language default limits. `node-22` currently uses a larger memory and process profile than the global defaults because V8 reserves substantial virtual memory at startup under isolate. Submission-provided limits may lower or raise the per-request limits, but they cannot exceed `MAX_LIMIT_*` configuration values.

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
