# yapi

LLM 智能代理网关的 Go 实现，提供统一入口、动态规则路由和管理后台，支持上游模型 API 的流式转发能力。

## 目录结构

- `cmd/gateway/`：网关服务入口，负责启动 HTTP 服务与路由挂载。
- `internal/proxy/`：核心代理逻辑，基于规则匹配请求并转发至上游。
- `internal/admin/`：后台管理接口，提供规则 CRUD 和健康检查。
- `pkg/rules/`：规则模型、验证逻辑，包含 PostgreSQL 存储、Redis 缓存与事件通知实现。
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

首次运行建议复制环境变量模板并按需调整：
```bash
cp .env.example .env.local
```

## 配置说明

核心环境变量（详见 `.env.example`）：
- `GATEWAY_PORT`：HTTP 服务监听端口，默认为 `8080`。
- `UPSTREAM_BASE_URL`：兜底上游地址，可为空，具体路由由规则决定。
- `DATABASE_DSN`：PostgreSQL 连接串，用于规则持久化（示例：`postgres://user:pass@host:5432/db?sslmode=disable`）。
- `REDIS_ADDR`：Redis 地址，默认 `localhost:6379`，用于缓存规则与发布刷新事件。
- `REDIS_CHANNEL`：规则变更通知频道，默认 `rules:sync`。

服务启动时会自动执行规则表结构迁移，并在无法连接 Redis 时退化为单实例内存缓存。

## 规则动作能力

网关根据后台配置的规则执行动作，当前支持：

- `set_target_url`：重定向请求目标地址。
- `set_headers` / `add_headers` / `remove_headers`：统一改写或剔除请求头。
- `set_authorization`：直接注入 `Authorization` 头，避免在客户端分发密钥。
- `rewrite_path_regex`：基于正则重写请求路径。
- `override_json`：对 JSON 请求体指定字段赋值，支持点号表示的多级嵌套键（仅对象类型）。
- `remove_json`：从 JSON 请求体移除指定字段。

> JSON 改写仅对对象类型请求体生效，解析失败时会在请求头附加 `X-YAPI-Body-Rewrite-Error` 便于排查。

## 开发规范

- 新增依赖后运行 `go mod tidy`，保持 `go.mod` / `go.sum` 同步。
- 提交前执行 `golangci-lint run ./...`，规则配置见 `.golangci.yml`。
- 代码统一使用 `gofmt` / `gofumpt` 自动格式化。
- 测试命名遵循 `Test<组件>_<场景>`，集成测试置于 `internal/integration_test`（需带 `-tags compose_test`）。

## 下一步路线

- 扩展代理匹配器与动作，支持请求体重写、鉴权注入等高级场景。
- 构建 Web 管理后台 UI，补充角色权限、操作审计能力。
- 接入系统监控（Prometheus 指标、结构化日志）与链路追踪。
