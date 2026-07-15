# batasd

Code execution service in Go with pluggable sandbox backends.

## Status

This project is in early development. The current implementation includes:

- HTTP API built with chi.
- PostgreSQL-backed submission storage.
- Embedded database migrations.
- Embedded language catalog.
- Token-based API authentication.
- In-memory queue stub for local development.
- Initial submission create and fetch endpoints.

Execution workers and sandbox drivers are being built incrementally. The local development sandbox targets are `direct` and Docker, while production is intended to use `isolate`.

## Requirements

- Go 1.23 or newer.
- Docker and Docker Compose.

## Local Development

Start PostgreSQL:

```bash
docker compose up -d postgres
```

Run the API:

```bash
env GOCACHE=/private/tmp/codeexec-go-cache GOMODCACHE=/private/tmp/codeexec-go-mod go run ./cmd/sandboxd
```

The API listens on `:18080` by default.

List languages:

```bash
curl -H 'X-Auth-Token: dev-token' http://localhost:18080/v1/languages
```

Create a submission:

```bash
curl -X POST http://localhost:18080/v1/submissions \
  -H 'Content-Type: application/json' \
  -H 'X-Auth-Token: dev-token' \
  -d '{"language":"python-3.12","source":"print(\"hello\")","input":"","expected_output":"hello"}'
```

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
- `AUTHN_HEADER=X-Auth-Token`
- `AUTHN_TOKENS=dev-token`
- `SANDBOX_DRIVER=direct`

## License

MIT
