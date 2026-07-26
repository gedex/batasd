# Post-MVP TODO

- Add batch submission create/fetch APIs once single-submission execution is stable.
- Generate clients or server stubs from the OpenAPI contract if consumers need them.
- Enable execution for disabled Exercism catalog tracks by adding runner/isolate toolchains, commands, and e2e fixtures.
- Add more concrete versions for languages that already have executable toolchains.
- Add multi-file project mode with user-provided compile and run scripts.
- Add submission deletion APIs and related authorization policy.
- Add durable production queue observability, metrics, and worker dashboards.
- Add richer admin operations for pausing/resuming workers.
- Add alternate sandbox drivers beyond the MVP direct, isolate, and Docker drivers, such as nsjail or gVisor.
- Add object-storage support for large artifacts if database-backed output storage becomes too limiting.
- Add configurable retention and cleanup jobs for completed submissions.
- Add callback retry/backoff controls.
- Add stricter callback allowlist management if proxy deployments need per-tenant callback policies.
