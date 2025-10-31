# Repository Guidelines

This guide helps contributors work consistently in this Go gateway monolith. Please follow the structure, commands, and conventions below when adding or modifying code.

## Project Structure & Module Organization
- `cmd/gateway/main.go` — service entrypoint.
- `internal/proxy` — core proxy pipeline and forwarding logic.
- `internal/admin` — admin/management APIs.
- `pkg/` — shared rule definitions, persistence models, metrics/monitoring.
- `testdata/` — integration scenarios; `testdata/stream/` holds streaming golden files.
- `web/admin/` — static admin UI assets.
- `deploy/` — container assets (`Dockerfile`, `docker-compose.yml`).
- `docs/` — architecture docs, including `需求规格说明与技术实施方案.md`.

## Build, Test, and Development Commands
- `go mod tidy` — sync `go.mod`/`go.sum` after dependency changes.
- `go build ./cmd/gateway` — build the gateway binary.
- `docker compose -f deploy/docker-compose.yml up gateway` — run gateway with Redis and PostgreSQL.
- `docker compose -f deploy/docker-compose.monitoring.yml up` — launch gateway + Prometheus + Grafana stack for observability testing.
- `go test ./...` — run all tests; focus with `-run TestProxy` when needed.
- `golangci-lint run ./...` — lint per `.golangci.yml` rules.
- `make verify` — start docker compose, hit health/metrics endpoints, and run SSE regression.
- CI pipeline & monitoring docs: see `docs/ci.md`, `docs/monitoring.md`, and the sample Grafana dashboard at `docs/grafana-dashboard.json`.

## Coding Style & Naming Conventions
- Format with `gofmt` or `gofumpt`; CI enforces formatting.
- Follow Go naming (e.g., `RuleMatcher`, `NewProxy`).
- Keep single files ≤ 400 lines; add package-level comments for complex flows.
- YAML/JSON use two-space indentation.
- If a React admin is introduced, use ESLint + Prettier for consistency.

## Testing Guidelines
- Use Go `testing` with `testify` assertions.
- Name tests `Test<Component>_<Scenario>` (e.g., `TestRuleEngine_MatchesHeaders`).
- Maintain ≥80% coverage for `internal/proxy` and `internal/rules`.
- Integration tests in `internal/integration_test`; run with `-tags compose_test` to start Docker deps.

## Commit & Pull Request Guidelines
- Use Conventional Commits (e.g., `feat: add redis-backed rule cache`).
- PRs must describe impact scope, testing approach, linked requirement items, screenshots for admin changes, and config migration notes.
- Request review from CODEOWNERS before merge.

## Security & Configuration Tips
- Do not commit secrets. Use `.env.local` (copy from `.env.local.example`) and Docker Compose overrides for dev; generate strong admin credentials per environment.
- Rotate upstream API keys regularly; store config securely (planned encrypted config service).
- For new dependencies, record threat assessment and mitigations in `docs/security.md`.
