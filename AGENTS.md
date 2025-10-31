# Repository Guidelines

## 项目结构与模块划分
网关采用单体 Go 服务实现：入口放在 `cmd/gateway/main.go`，核心代理逻辑位于 `internal/proxy`，后台管理接口放在 `internal/admin`。跨模块共享的规则定义、持久化模型与监控工具放入 `pkg/`。集成场景用例放在 `testdata/`，静态管理界面资源存放于 `web/admin/`，容器化资源如 `Dockerfile`、`docker-compose.yml` 放在 `deploy/`。包含《需求规格说明与技术实施方案.md》的架构资料统一归档至 `docs/`。

## 构建测试与开发命令
新增依赖后执行 `go mod tidy`，保持 `go.mod` 与 `go.sum` 同步。使用 `go build ./cmd/gateway` 生成二进制。快速联调可运行 `docker compose -f deploy/docker-compose.yml up gateway`，一次性拉起 Redis、PostgreSQL 及网关。常规测试执行 `go test ./...`，聚焦排查可加 `-run TestProxy`。在提交前运行 `golangci-lint run ./...`（规则见 `.golangci.yml`）以捕捉潜在回归。

## 代码风格与命名规范
提交前使用 `gofmt` 或 `gofumpt` 自动格式化，CI 会阻挡未格式化代码。命名遵循 Go 惯例，例如 `RuleMatcher`、`NewProxy`。单文件建议控制在 400 行以内，针对复杂逻辑（如流式转发管线）补充包级注释。YAML、JSON 配置统一使用两个空格缩进；若引入 React 管理端，遵循 ESLint + Prettier 统一风格。

## 测试规范
基于 Go `testing` 标准库，配合 `testify` 断言提升可读性。测试命名采用 `Test<组件>_<场景>`（如 `TestRuleEngine_MatchesHeaders`）。流式响应的黄金文件放置在 `testdata/stream/`。`internal/proxy` 与 `internal/rules` 需保持至少 80% 的覆盖率；集成测试置于 `internal/integration_test`，通过 `-tags compose_test` 启动 Docker 依赖。

## 提交与合并请求指引
遵循 Conventional Commits，如 `feat: add redis-backed rule cache`，便于自动生成变更日志。每个 PR 需说明影响范围、测试方式、关联需求条目，并提供管理后台改动的截图及配置迁移说明。合并前请邀请 CODEOWNERS 列表中的责任人评审。

## 安全与配置注意事项
严禁在 Git 中提交密钥，开发环境使用 `.env.local` 并通过 Docker Compose 覆盖加载。定期轮换上游 API Key，并在计划中的配置服务中以加密形式存储。引入新依赖时，在 `docs/security.md` 记录威胁评估与缓解措施。
