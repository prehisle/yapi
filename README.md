# yapi

LLM 智能代理网关的 Go 实现，提供统一入口、动态规则路由和管理后台，支持上游模型 API 的流式转发能力。

## 目录结构

- `cmd/gateway/`：网关服务入口，负责启动 HTTP 服务与路由挂载。
- `internal/proxy/`：核心代理逻辑，基于规则匹配请求并转发至上游。
- `internal/admin/`：后台管理接口，提供规则 CRUD 和健康检查。
- `pkg/rules/`：规则模型、验证逻辑以及默认的内存存储实现。
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

环境变量：
- `GATEWAY_PORT`（默认 `8080`）
- `UPSTREAM_BASE_URL`（默认无，需结合规则配置）
- `DATABASE_DSN`、`REDIS_ADDR` 预留给后续持久化与缓存实现。

## 开发规范

- 新增依赖后运行 `go mod tidy`，保持 `go.mod` / `go.sum` 同步。
- 提交前执行 `golangci-lint run ./...`，规则配置见 `.golangci.yml`。
- 代码统一使用 `gofmt` / `gofumpt` 自动格式化。
- 测试命名遵循 `Test<组件>_<场景>`，集成测试置于 `internal/integration_test`（需带 `-tags compose_test`）。

## 下一步路线

- 接入 PostgreSQL + Redis 的规则存储与缓存，实现 Pub/Sub 热更新。
- 扩展代理匹配器与动作，完善 Header/Body 重写能力。
- 搭建 Web 管理后台 UI，并补充权限、审计等安全措施。
