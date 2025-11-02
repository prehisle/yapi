# Repository Guidelines

## Project Structure & Module Organization
This gateway service is anchored in `cmd/gateway/main.go`, which wires the proxy, admin, and config subsystems. Request handling logic sits in `internal/proxy` (routing, forwarding, streaming). Management APIs and UI bridges live in `internal/admin`. Shared utilities, rule engines, persistence layers, and metrics clients are under `pkg/`. Higher-level integration specs reside in `testdata/` (streaming golden files under `testdata/stream/`). The static admin bundle is bundled in `web/admin/`, while Docker assets and monitoring dashboards live in `deploy/` and `docs/` respectively. Helper scripts and local automation are under `scripts/`.

## Build, Test, and Development Commands
Use the Go toolchain directly: `go build ./cmd/gateway` produces the release binary, and `go test ./...` executes all unit suites (narrow with `-run TestProxy` while iterating). `golangci-lint run ./...` enforces formatting, vetting, and static checks. For a full-stack sandbox, run `docker compose -f deploy/docker-compose.yml up gateway` to launch Redis, PostgreSQL, and the gateway. The observability stack comes via `docker compose -f deploy/docker-compose.monitoring.yml up`. `make verify` chains dependency startup, health checks, and SSE regression tests in one pass.

## Coding Style & Naming Conventions
Go sources must pass `gofmt`/`gofumpt`; commits failing lint are rejected. Favor descriptive, idiomatic names (`RuleMatcher`, `NewProxy`) and avoid abbreviations on exported APIs. Split large flows into packages and keep files under ~400 lines. YAML and JSON artifacts in `deploy/` and `docs/` use two-space indentation. Document complex pipelines with short package or function comments.

## Testing Guidelines
Author tests with the standard library plus `testify` assertions. Name cases `Test<Component>_<Scenario>` (e.g. `TestRuleEngine_MatchesHeaders`). Maintain at least 80% coverage in `internal/proxy` and `internal/rules`; verify with `go test -cover ./internal/...`. Integration suites belong in `internal/integration_test` and run via `go test -tags compose_test ./internal/integration_test`, which provisions Docker dependencies automatically.

## Commit & Pull Request Guidelines
Follow Conventional Commits (`feat: add redis-backed rule cache`, `fix: patch SSE notifier`). Pull requests should capture scope, linked requirements or issues, testing evidence (`go test`, `golangci-lint`), admin UI screenshots when UI changes, and configuration migration notes. Request CODEOWNER review before merge and confirm CI parity locally.

## Security & Configuration Tips
Never commit secrets; clone `.env.local` from `.env.local.example` and store credentials securely. Rotate upstream API keys regularly, documenting mitigations in `docs/security.md`. Prefer Docker Compose overrides for environment-specific tweaks instead of editing tracked manifests.


## Agent-Specific Instructions
- 与用户交互时始终使用中文回复，包括讨论代码、测试结果或提交建议。