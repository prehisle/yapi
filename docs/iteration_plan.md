# Architecture & Iteration Plan

## Architecture Snapshot
- Components: Go gateway (monolith) hosting proxy + admin, PostgreSQL (rules source of truth), Redis (cache + pub/sub).
- Hot path: client → gateway proxy → rule cache (Redis) → upstream LLM.
- Cold path: admin UI → gateway admin APIs → PostgreSQL, invalidate via Redis pub/sub.
- Observability: structured access/error logs, Prometheus metrics endpoints, tracing hooks for upstream latency.

## Responsibilities
- `internal/proxy`: request routing, rule evaluation, streaming forwarding.
- `pkg/rules`: shared rule models, matcher/action evaluation helpers.
- `internal/admin`: REST APIs for rule CRUD, auth, validation.
- `pkg/persistence`: GORM models, Postgres migrations, Redis cache bridge.

## Sprint 1 Objective (2 Weeks)
Deliver a functional proxy that forwards streaming requests with declarative rules and admin CRUD, deployable via Docker Compose.

### Sprint Backlog
1. Project scaffolding with Gin, GORM, reverse proxy plumbing.
2. Rule model, Postgres migrations, Redis cache loader.
3. Proxy middleware applying matchers for path/method/header, actions for target URL + headers.
4. Streaming-safe reverse proxy with request/response logging.
5. Admin API + basic web UI for rule CRUD and enable/disable.
6. Redis pub/sub to refresh in-memory rules instantly.
7. CI jobs: `go test ./...`, `golangci-lint run ./...`, Docker image build.

### Definition of Done
- 所有冲刺 backlog 条目合并且附带单元 / 集成测试。
- `go test ./...`、`go test -tags compose_test ./internal/integration_test` 与 `golangci-lint run ./...` 全部通过。
- Docker Compose 栈（`docker compose -f deploy/docker-compose.yml up gateway`）可一键启动并在健康检查脚本中返回 200。
- `/metrics` 端点暴露请求计数、错误率、上游延迟直方图，并在 Compose 环境内通过临时 curl 校验。
- 管理端开启 JWT 鉴权（或在配置中显式禁用）且无敏感信息打印在日志中。
- SSE/Chunk 流式代理在 `testdata/stream/` 黄金用例中验证无丢字节，日志格式化输出通过集中式采集校验。

## Risks & Mitigations
- Streaming regression: add integration test with SSE fixture from `testdata/stream/`.
- Rule cache staleness: add TTL fallback, ensure pub/sub reconnect logic.
- Secret exposure: require `.env.local`, document in `docs/security.md`, enforce vault integration in later sprint.

## Progress Update · 2025-11-01
- ✅ 引入 `pkg/accounts` 域服务与数据库迁移，提供用户、API Key、上游凭据和绑定的管理接口，并在管理端 API 中公开 CRUD 能力。
- ✅ Web 管理后台新增导航框架与“用户管理”界面，可完成用户创建、密钥生成、上游凭据绑定等主流程。
- ✅ 代理链路集成 API Key 鉴权中间件，可依据绑定信息选择上游并自动注入凭据；集成测试覆盖账户→上游→代理请求全链路。
- ✅ Playwright 端到端脚本覆盖用户全流程，已在 `web/admin/tests/users.e2e.spec.ts` 中通过轮询 API 等待列表刷新，替代脆弱的固定等待。
- ⏳ 账户与上游管理相关的负载与缓存策略（API Key 缓存、速率限制）尚未落地，待后续迭代。
- ✅ 已通过 `ADMIN_ALLOWED_ORIGINS` 与 `deploy/nginx/accounts.conf` 示例同步账户 API 的 CORS 白名单策略，并在 `docs/security.md` 记录部署指引。
- ✅ `internal/integration_test/gateway_compose_test.go` 新增异常路径覆盖绑定缺失、密钥吊销等场景，`go test -tags compose_test` 已通过。
- 🚧 规则匹配虽已支持账户上下文，但密钥提取入口仍固定在标准头，且元数据匹配缺乏去重/嵌套处理，需要补齐配置化提取与匹配策略。

### 即刻推进事项
- 密钥提取扩展：实现可配置 API Key 读取（自定义头、可选 JSON 路径），同步文档与运维指引。
- 规则匹配体验：优化管理端输入校验与 API 过滤能力，提供示例与去重逻辑，确保多字段配置易用。
- 自动化覆盖：补充 Playwright 与集成测试，验证多租户/多上游场景；完善测试计划与示例数据脚本。
