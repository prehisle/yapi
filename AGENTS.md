# Repository Guidelines

## Project Structure & Module Organization
- `cmd/gateway/main.go` — entrypoint wiring proxy, admin, and config layers.
- `internal/proxy` — request pipeline, routing rules, and upstream forwarding.
- `internal/admin` — management APIs served alongside the gateway.
- `pkg/` — shared libraries (rules, persistence, metrics) used across packages.
- `testdata/` — integration scenarios; streaming goldens under `testdata/stream/`.
- `web/admin/` — static UI bundle for the admin console.
- `deploy/` — Docker assets (`Dockerfile`, `docker-compose.yml`, monitoring stack).
- `docs/` — architectural references, CI/monitoring notes, and Grafana dashboard JSON.

## Build, Test, and Development Commands
- `go build ./cmd/gateway` builds the service binary.
- `go test ./...` exercises unit tests; focus with `-run TestProxy` when iterating.
- `golangci-lint run ./...` enforces lint and formatting rules.
- `docker compose -f deploy/docker-compose.yml up gateway` runs the gateway with Redis/PostgreSQL.
- `docker compose -f deploy/docker-compose.monitoring.yml up` spins up the observability stack (Prometheus, Grafana).
- `make verify` launches dependencies, checks health/metrics endpoints, and runs SSE regression.

## Coding Style & Naming Conventions
- Format Go with `gofmt` or `gofumpt`; CI rejects unformatted files.
- Adhere to idiomatic Go names (`RuleMatcher`, `NewProxy`); avoid abbreviations in public APIs.
- Keep files under 400 lines and document complex flows with package comments.
- Use two-space indentation for YAML/JSON under `deploy/` and `docs/`.

## Testing Guidelines
- Write tests with Go `testing` plus `testify` assertions.
- Name cases `Test<Component>_<Scenario>` (e.g., `TestRuleEngine_MatchesHeaders`).
- Maintain ≥80% coverage in `internal/proxy` and `internal/rules`.
- Place integration suites in `internal/integration_test` and run with `-tags compose_test` to provision Docker deps.

## Commit & Pull Request Guidelines
- Use Conventional Commits such as `feat: add redis-backed rule cache` or `fix: patch SSE notifier`.
- PRs must state scope, testing performed, linked requirements, admin UI screenshots, and config migration notes.
- Request CODEOWNER review before merge and ensure `go test ./...` plus `golangci-lint run ./...` succeed locally.

## Security & Configuration Tips
- Never commit secrets; use `.env.local` sourced from `.env.local.example`.
- Rotate upstream API keys and document mitigations for new dependencies in `docs/security.md`.
- Store environment credentials securely and prefer Docker Compose overrides for local overrides.

## Agent-Specific Instructions
- 与用户交互时始终使用中文回复，包括讨论代码、测试结果或提交建议。
- 启动需要常驻运行的进程（如 `go run ./cmd/gateway`）时，务必在同一个 shell 调用中以后台方式运行并立即返回（例如包裹在 `(...) & echo $!` 中），避免阻塞会话；必要时记录 PID 或提供停止方法。
- 若需运行网关守护进程，优先使用 Docker Compose 后台模式：`docker compose -f deploy/docker-compose.yml up -d gateway`；查看日志用 `logs -f gateway`，停止使用 `stop gateway` 或 `down`，避免直接 `go run` 卡住会话。
