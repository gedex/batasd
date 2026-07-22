# batasd

Code execution service in Go with pluggable sandbox backends.

`batasd` accepts source code submissions, runs them under configurable limits, and returns structured execution results. It is built for private/internal systems that need a small online judge style execution core without tying the API to a specific sandbox runtime.

## What It Does

- Executes single-file submissions through HTTP.
- Stores submissions and callback attempts in PostgreSQL.
- Runs workers from the same binary.
- Supports `direct`, Docker, and Linux `isolate` sandbox drivers.
- Ships a language catalog for `python`, `node`, `c`, `cpp`, `go`, `rust`, `java`, and `php`.
- Enforces time, memory, process, file, and output limits.
- Supports expected-output judging, repeated runs, zipped `additional_files`, callbacks, pagination, field filtering, startup recovery, OpenAPI drift checks, and e2e fixtures.

## Quick Start

```bash
docker compose up -d postgres
docker compose build runner

env SANDBOX_DRIVER=docker \
  GOCACHE=/tmp/batasd-go-cache \
  GOMODCACHE=/tmp/batasd-go-mod \
  go run ./cmd/batasd
```

The API listens on `:18080` by default.

```bash
curl -H 'Authorization: Bearer dev-token' \
  http://localhost:18080/v1/languages
```

Submit a file:

```bash
scripts/submit.sh --wait \
  --input Akeda \
  --expected-output "hello from python: Akeda" \
  testdata/e2e/programs/languages/hello.py
```

Run the fixture suite against a running API:

```bash
scripts/e2e.sh --url http://localhost:18080
```

## Useful Links

- [Development Guide](docs/development.md)
- [OpenAPI Contract](api/openapi.yaml)
- [Post-MVP TODO](todo.md)
- [Agent Guide](AGENTS.md)

## License

MIT
