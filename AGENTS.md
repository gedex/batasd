# AGENTS.md

Concise guide for AI agents working on `batasd`.

## Project Snapshot

`batasd` is a Go code-execution service. It accepts source submissions over HTTP, stores them in PostgreSQL, queues work in memory, executes with a pluggable sandbox driver, then persists and returns structured results.

Supported sandbox drivers:

- `direct`: development only, best-effort limits.
- `docker`: portable local/staging sandbox.
- `isolate`: Linux production-style sandbox.

Supported catalog languages: `python`, `node`, `c`, `cpp`, `go`, `rust`, `java`, `php`.

## Architecture Map

- `cmd/batasd`: binary entrypoint and wiring.
- `internal/config`: environment config and validation.
- `internal/httpapi`: chi routes, auth, JSON parsing, field filtering, OpenAPI drift tests.
- `internal/submission`: service layer, validation, models, public responses.
- `internal/repository/postgres`: PostgreSQL persistence.
- `internal/queue/memory`: local in-memory queue.
- `internal/worker`: worker loop, recovery, callback dispatch.
- `internal/execution`: compile/run workflow, judging, additional file extraction.
- `internal/sandbox`: driver interface and shared output handling.
- `internal/sandbox/direct`: direct development runner.
- `internal/sandbox/docker`: Docker runner.
- `internal/sandbox/isolate`: isolate runner.
- `internal/language`: embedded language catalog.
- `internal/migrations`: embedded SQL migrations.
- `internal/callback`: callback delivery and attempt recording.
- `scripts`: local helpers for submit, callbacks, and e2e.
- `testdata/e2e`: source fixtures for end-to-end tests.

## Rules For Changes

- Keep HTTP mechanics in `internal/httpapi`; business workflow belongs in `internal/submission`, `internal/worker`, or `internal/execution`.
- Keep storage behind repository interfaces; production/local storage is PostgreSQL.
- Keep sandbox-specific behavior inside `internal/sandbox/{direct,docker,isolate}`.
- Update `api/openapi.yaml` when routes, request fields, or response fields change. Tests catch chi route drift, public DTO property drift, and status enum drift.
- Update migrations for schema changes. Do not edit old migrations after they have shipped.
- Update `internal/language/catalog/languages.json` plus tests when changing languages, versions, commands, or defaults.
- Keep `scripts/submit.sh` as the black-box client used by e2e scripts.
- Prefer small, focused tests near the changed package.
- Do not use port `8080` in examples or tests unless explicitly asked; it is commonly used by the proxy. Prefer `18080` or throwaway ports like `18086`.

## Verification Commands

Commands below are POSIX/Linux friendly. Cache paths use disposable directories under `/tmp`; use another writable temp directory if your environment requires it.

Go tests:

```bash
env GOCACHE=/tmp/batasd-go-cache \
  GOMODCACHE=/tmp/batasd-go-mod \
  go test ./...
```

Start local Docker-backed API:

```bash
docker compose up -d postgres
docker compose build runner
env SANDBOX_DRIVER=docker \
  GOCACHE=/tmp/batasd-go-cache \
  GOMODCACHE=/tmp/batasd-go-mod \
  go run ./cmd/batasd
```

Full e2e:

```bash
scripts/e2e.sh --url http://localhost:18080
```

Isolate playground:

```bash
ssh root@do-playground
cd /root/code/batasd
env PATH=/usr/local/bin:/usr/local/jdk-21/bin:/usr/local/node-v22/bin:/usr/local/go/bin:/usr/bin:/bin \
  HTTP_ADDR=127.0.0.1:18085 \
  SANDBOX_DRIVER=isolate \
  GOCACHE=/tmp/batasd-go-cache \
  GOMODCACHE=/tmp/batasd-go-mod \
  go run ./cmd/batasd
```

Then, in another SSH command:

```bash
cd /root/code/batasd
PATH=/usr/local/bin:/usr/local/jdk-21/bin:/usr/local/node-v22/bin:/usr/local/go/bin:/usr/bin:/bin \
  scripts/e2e.sh --url http://127.0.0.1:18085 --interval 0.2
```

Stop temporary servers after verification.

## Important Behaviors

- Auth uses `Authorization: Bearer <token>`.
- `wait=true` returns `202 Accepted` if the configured wait timeout expires before terminal status.
- List responses clamp large `limit` values to `100`.
- Output is capped and marked with `stdout_truncated`, `stderr_truncated`, or `compile_output_truncated`.
- `limits.runs` compiles once and executes repeatedly.
- Startup recovery requeues unfinished `queued` or `processing` submissions.
- Callback delivery is single-attempt in MVP; retry/backoff is post-MVP.
- Python memory exhaustion can be `memory_limit_exceeded` or `runtime_error` with `MemoryError`, depending on sandbox behavior.

## Documentation

- Human README: `README.md`
- Development details: `docs/development.md`
- API contract: `api/openapi.yaml`
- Post-MVP backlog: `todo.md`
