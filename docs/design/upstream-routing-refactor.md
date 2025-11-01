## 目标

依据新增需求，重构账户与路由模型，使系统能够：

- 直接管理“上游 Key”（包含名称、服务类型、可用端点与启用状态），不再暴露“服务商”概念；
- 为“用户 Key”提供启用/停用/删除能力，并维护其可访问的上游 Key 列表；
- 在规则层面依据请求匹配出的服务名，从用户 Key 绑定列表中挑选首个启用的上游 Key 进行转发。

## 数据模型调整

| 原模型 | 调整后 | 关键变化 |
| --- | --- | --- |
| `accounts.APIKey` | `accounts.APIKey` | 新增 `Enabled bool` 字段；默认 `true`；停用时拒绝认证与路由。 |
| `accounts.UpstreamCredential` | `accounts.UpstreamKey`（结构保留同表名） | 字段调整：`Provider` → `Service`；`Label` 作为“名称”展示；新增 `Enabled bool`；端点继续使用 `Endpoints` JSON；保留表名 `upstream_credentials` 以复用既有数据。 |
| `accounts.UserAPIKeyBinding` | `accounts.UserKeyBinding` | 允许同一用户 Key 绑定多个上游 Key；新增 `Service string`（匹配规则使用的服务名）；新增 `Position int` 用于排序；保留 `Metadata` 以描述额外限流/备注信息。 |

> **迁移策略**：通过 GORM `AutoMigrate` 自动添加新增列；`UserAPIKeyBinding` 新增字段会自动补零值。原 `Provider` 字段迁移到 `Service` 时需使用 `gorm:"column:provider"` 标签兼容旧列，再在服务层转换表示。

## 认证与路由流程

1. 请求到达时提取用户 API Key，若 `Enabled=false`，直接返回 401。
2. 加载该 Key 所绑定的 `UserKeyBinding` 列表（按 `Position`、`CreatedAt` 升序）。
3. 规则匹配阶段新增 `matcher.service`（字符串或 header/url 正则派生出的服务名）：
   - 首先依据配置在 matcher 中提取服务名（支持固定值、URL 前缀或 Header 正则）。
   - 选取绑定列表中 `Service` 与命中服务名相同且上游 Key 启用的首条记录。
4. 将上游 Key 注入请求头（`Authorization`、`X-Upstream-Service` 等）并使用其端点列表进行转发。

## API & 前端改动概览

- 用户 Key 管理接口新增 `enabled` 字段并暴露 PATCH 端点用于启用/停用。
- 上游 Key 接口使用 `service` 字段替换 `provider`；新增启用/停用操作。
- 绑定接口改为创建/更新 `UserKeyBinding`（包含 `service`、`upstream_key_id`、可选排序）。
- 管理后台页面需同步重命名文案（“服务商”→“服务”/“上游 Key”），并提供开关按钮。

## 规则配置扩展

- `rules.Matcher` 新增 `Service string` 与可选的 `ServiceFromHeader`/`ServiceFromPath` 配置，用于在规则内声明如何确定服务名。
- `binding_providers`/`binding_upstream_ids` 将被替换为 `service` 匹配逻辑和可选的 `allowed_upstream_key_ids`。
- 需要更新相关单元测试及 `docs/test_plan_account_context_rules.md` 中的规则说明。

## 后续步骤

1. 更新 `pkg/accounts` 模型、服务与测试，确保 AutoMigrate 添加新列。
2. 重构管理端 Handler / Service 及前端 UI，打通启用/停用、绑定管理流程。
3. 改写代理中间件，将规则匹配结果与用户 Key 绑定结合，实现“首个匹配服务上游 Key”路由。
4. 调整文档、集成测试与演示配置，覆盖新的字段与流程。
