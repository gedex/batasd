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

Submit a source file with language detection:

```bash
scripts/submit.sh ./main.py
scripts/submit.sh --url http://localhost:18082 --wait ./main.js
```

The helper defaults to `http://localhost:18080/v1/submissions` and `Authorization: Bearer dev-token`. Override those with `--url`, `BATASD_SUBMISSIONS_URL`, `--token`, or `BATASD_TOKEN`.

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

The language catalog can set per-language default limits. `node-22` currently uses a larger memory and process profile than the global defaults because V8 reserves substantial virtual memory at startup under isolate.

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
