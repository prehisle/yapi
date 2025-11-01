# yapi

LLM 智能代理网关的 Go 实现，提供统一入口、动态规则路由和管理后台，支持上游模型 API 的流式转发能力，并内置账户服务与凭据绑定能力。

## 目录结构

- `cmd/gateway/`：网关服务入口，负责启动 HTTP 服务与路由挂载。
- `internal/proxy/`：核心代理逻辑，基于规则匹配请求并转发至上游。
- `internal/admin/`：后台管理接口（服务层 + HTTP handler），提供规则 CRUD、账户/凭据管理与刷新通知等能力。
- `pkg/rules/`：规则模型、验证逻辑，包含 PostgreSQL 存储、Redis 缓存与事件通知实现。
- `pkg/accounts/`：用户、API Key 与上游凭据领域模型及服务封装。
- `deploy/`：容器化与本地集成环境定义（`Dockerfile`、`docker-compose.yml`）。
- `docs/`：《需求规格说明与技术实施方案》等架构文档归档目录。
- `testdata/`：后续用于存放黄金文件与集成测试场景。
- `web/admin/`：静态管理界面或前端工程的占位目录。

## 快速开始

1. 安装依赖并构建：
   ```bash
   go build ./cmd/gateway
   ```
2. 运行测试：
   ```bash
   go test ./...
   ```
3. 启动开发环境（需要 Docker 与 Docker Compose）：
   ```bash
   docker compose -f deploy/docker-compose.yml up gateway
   ```
4. 启动监控观测（可选）：
   ```bash
   docker compose -f deploy/docker-compose.monitoring.yml up
   ```
   该命令将同时拉起 gateway、Prometheus 与 Grafana，默认 Grafana 访问地址为 `http://localhost:3000`（账号/密码：`admin`/`admin`）。

首次运行建议复制环境变量模板并按需调整：
```bash
cp .env.example .env.local
```
随后将 `.env.local` 中的管理员账号、密码与 JWT 密钥替换为强随机值；可使用命令 `openssl rand -hex 32` 生成密钥。建议为开发、测试、生产环境分别维护独立的 `.env.<env>.local` 并通过 CI/CD 注入，避免共享凭据。

## 配置说明

核心环境变量（详见 `.env.example`）：
- `GATEWAY_PORT`：HTTP 服务监听端口，默认为 `8080`。
- `UPSTREAM_BASE_URL`：兜底上游地址，可为空，具体路由由规则决定。
- `DATABASE_DSN`：PostgreSQL 连接串，用于规则持久化（示例：`postgres://user:pass@host:5432/db?sslmode=disable`）。
- `REDIS_ADDR`：Redis 地址，默认 `localhost:6379`，用于缓存规则与发布刷新事件。
- `REDIS_CHANNEL`：规则变更通知频道，默认 `rules:sync`。
- `ADMIN_USERNAME` / `ADMIN_PASSWORD`：管理后台 Basic Auth 凭据，留空则允许匿名访问（仅限开发环境）。
- `ADMIN_TOKEN_SECRET`：用于签发管理后台 JWT 的 HMAC 密钥；留空则禁用 token 登录。
- `ADMIN_TOKEN_TTL`：JWT 过期时间（默认 `30m`，支持 `1h`、`3600` 等格式）。
- `ADMIN_ALLOWED_ORIGINS`：允许访问 `/admin` API 的前端域名白名单，留空则回显请求 `Origin`。

服务启动时会自动执行规则表结构迁移，并在无法连接 Redis 时退化为单实例内存缓存。

## 规则动作能力

网关根据后台配置的规则执行动作，当前支持：

- `set_target_url`：重定向请求目标地址。
- `set_headers` / `add_headers` / `remove_headers`：统一改写或剔除请求头。
- `set_authorization`：直接注入 `Authorization` 头，避免在客户端分发密钥。
- `rewrite_path_regex`：基于正则重写请求路径。
- `override_json`：对 JSON 请求体指定字段赋值，支持点号与 `[]` 数组索引（如 `messages[1].role`），自动补齐缺失节点。
- `remove_json`：从 JSON 请求体移除指定字段或数组元素。

> JSON 改写仅在 `Content-Type` 为 `application/json` 时生效，发生错误会在请求头附加 `X-YAPI-Body-Rewrite-Error` 并输出结构化日志（`slog`），便于排查。

## 高级匹配条件

除了基础的路径 / 方法 / 请求头匹配外，规则还可以依据账户上下文进行差异化路由：

- `api_key_ids` / `api_key_prefixes`：仅对指定 API Key（完整 ID 或 8 位前缀）生效。
- `user_ids`：限制命中用户 ID 列表；`user_metadata` 可校验用户元数据中的键值对。
- `binding_upstream_ids` / `binding_providers`：根据绑定到的上游凭据 ID 或 Provider 精准路由。
- `require_binding`：要求请求成功解析出 API Key 绑定信息，否则不会命中该规则。

所有字段均可组合使用，满足多租户或多上游场景下的细粒度控制。详见管理端“规则”页面的“账户上下文匹配”配置分组。

## 管理 API（简要）

管理端暴露在 `/admin` 路径下，核心接口：

- 规则管理：
  - `GET /admin/rules`：列出全部规则，按优先级降序返回。
  - `POST /admin/rules`：创建规则，提交 JSON 结构体（参考 `pkg/rules/Rule`）。
  - `PUT /admin/rules/:id`：更新指定规则，若请求体缺少 `id` 将按路径补齐。
  - `DELETE /admin/rules/:id`：删除规则；若不存在返回 404。
- 账户管理：
  - `GET /admin/users`：列出所有运营用户，返回描述与元数据。
  - `POST /admin/users`：创建用户，可配置名称、描述与 JSON 元数据。
  - `DELETE /admin/users/:id`：删除用户，若存在关联资源需先处理。
- API Key 生命周期：
  - `GET /admin/users/:id/api-keys`：查看指定用户的全部密钥及最近使用时间。
  - `POST /admin/users/:id/api-keys`：生成新密钥，响应中包含一次性返回的完整密钥。
  - `DELETE /admin/api-keys/:id`：吊销密钥，实时阻止代理层继续透传请求。
  - `POST /admin/api-keys/:id/binding`：将密钥绑定到上游凭据，限定唯一绑定。
  - `GET /admin/api-keys/:id/binding`：查看绑定信息与目标上游详情。
- 上游凭据：
  - `GET /admin/users/:id/upstreams`：列出指定用户的上游凭据及元数据。
  - `POST /admin/users/:id/upstreams`：录入上游访问凭据，支持配置标签与可用 Endpoint。
  - `DELETE /admin/upstreams/:id`：删除凭据并解除所有关联绑定。
- 公共接口：
  - `GET /admin/healthz`：健康检查。
  - `POST /admin/login`：传入用户名/密码获取短期 Bearer Token（需配置 `ADMIN_TOKEN_SECRET`）。

认证说明：
- 若设置了用户名/密码，所有受保护接口必须携带 `Authorization` 头，可使用 Bearer Token（推荐）或 Basic Auth。
- Token 模式默认有效期 `ADMIN_TOKEN_TTL`，需使用 `Authorization: Bearer <token>` 访问。
- 代理入口会识别来自客户端的 `Authorization: Bearer <api-key>`，校验密钥是否已绑定上游，并根据绑定结果注入上游所需凭据。

CORS 与部署注意：
- `internal/middleware.CORS` 会基于 `ADMIN_ALLOWED_ORIGINS` 白名单回显 `Access-Control-Allow-*` 头；留空则允许任意来源。
- 生产环境需同步配置前置 Nginx，示例见 `deploy/nginx/accounts.conf`，更多安全建议详见 `docs/security.md`。

所有接口返回 `X-Request-ID`，可配合日志排查；错误响应包含 `error` 字段描述原因。

## 可观测性

- 所有请求都会生成并透传 `X-Request-ID`，同时在访问日志和代理日志中输出。
- 代理日志记录规则命中、目标上游、响应状态与耗时（毫秒），便于排查上游性能问题。
- 管理操作会通过 `gateway_admin_actions_total` 指标统计 action/outcome，可在 `docs/monitoring.md`、`docs/security.md` 查阅接入指引。

## 管理后台前端

前端位于 `web/admin/`，采用 React + TypeScript + Vite 实现。主要功能：

- 登录页：调用 `POST /admin/login` 获得短期 Bearer Token，并持久化至浏览器。
- 规则列表：支持搜索、分页展示、刷新，提供规则的启用/停用、编辑、删除操作（带确认）。
- 规则表单：支持新建或更新规则，配置路径、方法、目标地址、头部与 JSON 动作等高级字段。
- 用户与凭据：支持录入用户、生成/吊销 API Key、配置上游凭据并查看绑定状态。
- 通知与确认：统一 Toast 提示操作结果，删除等敏感操作提供二次确认。

开发/运行：

```bash
cd web/admin
npm install        # 首次安装依赖
npm run dev        # 本地开发，默认代理请求至 http://localhost:8080
npm run build      # 产出静态资源
npm run lint       # 代码质量检查
npm run test:e2e   # 端到端验证（需预先启动 docker compose 并执行 npm run build）
```
默认开发代理会将 `/admin` 请求转发至后端服务；如需定制可设置 `VITE_API_BASE_URL`。

端到端测试说明：

- 需事先运行 `docker compose -f deploy/docker-compose.yml up -d` 启动 PostgreSQL、Redis 及网关，并在 `web/admin` 目录执行一次 `npm run build`。
- Playwright 默认使用 `http://localhost:8080` 作为管理 API，可通过 `PLAYWRIGHT_API_BASE_URL` 覆盖；登录凭据取自环境变量 `ADMIN_USERNAME` / `ADMIN_PASSWORD`。
- 脚本会轮询 `/admin/users` API 等待数据刷新，避免依赖固定的 `sleep`。
- 首次执行前请运行 `npx playwright install --with-deps chromium` 准备浏览器依赖。

## 开发规范

- 新增依赖后运行 `go mod tidy`，保持 `go.mod` / `go.sum` 同步。
- 安装开发工具：`go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`
- 提交前执行 `golangci-lint run ./...`，规则配置见 `.golangci.yml`；也可以使用 `make lint`、`make test`、`make verify` 快速运行常规检查，其中 `make verify` 会自动拉起 Docker Compose 并执行健康检查、指标校验与 SSE 回归测试。
- 代码统一使用 `gofmt` / `gofumpt` 自动格式化。
- 测试命名遵循 `Test<组件>_<场景>`，集成测试置于 `internal/integration_test`（需带 `-tags compose_test`）。
- 持续集成流程详见 `docs/ci.md`，部署监控参考 `docs/monitoring.md` 与 `docs/grafana-dashboard.json`。

## 下一步路线

- 规划账户缓存与 API Key 速率限制策略，减轻数据库与上游负载。
- 扩展 compose 集成测试，覆盖 Redis 故障、上游凭据失效等异常路径。
- 引入更细粒度的访问控制与操作审计，完善管理端安全模型。
