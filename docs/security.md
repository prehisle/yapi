# 安全评估记录

本文档用于跟踪仓库中引入的新依赖与关键安全决策，评估潜在风险并记录缓解措施。

## 2025-10-31：JSON 改写能力依赖

- **依赖名称**：`github.com/tidwall/sjson`（间接依赖 `gjson`、`pretty`、`match`）
- **用途**：在代理阶段对 JSON 请求体执行字段覆盖、数组索引更新与删除操作，满足规则动作“override_json”“remove_json”的实现需求。
- **风险评估**：
  - 仅在代理后端进程内运行，不执行脚本或动态代码，主要风险为非预期的 JSON 处理缺陷。
  - 库作者 Tidwall 社区活跃，版本稳定（v1.2.5），广泛用于生产项目。
  - JSON 改写属于用户可配置功能，若配置错误可能导致请求体被错误修改。
- **缓解措施**：
  - 仅在 `Content-Type` 为 `application/json` 时启用改写，若解析失败会返回错误并附带诊断头 `X-YAPI-Body-Rewrite-Error`。
  - 通过结构化日志 (`slog`) 记录失败原因与请求 ID，便于审计与回滚规则。
  - 建议在后续版本引入集成测试覆盖关键 API 供应商的典型请求体，避免改写逻辑回归。

## 2025-10-31：管理后台认证升级

- **策略**：在 Basic Auth 的基础上，新增基于 HMAC 的短期 JWT，凭据来源于 `ADMIN_USERNAME` / `ADMIN_PASSWORD`，JWT 签名密钥由 `ADMIN_TOKEN_SECRET` 提供。
- **安全考虑**：
  - 若用户名或密码未配置，依旧允许匿名访问，仅限本地开发；生产环境必须设置强密码并确保 HTTPS 传输。
  - `ADMIN_TOKEN_SECRET` 应保存在安全的密钥管理系统中，并定期轮换。JWT 默认有效期 30 分钟，建议结合登出及服务端轮转策略。
- **后续计划**：
  - 评估引入多租户 / RBAC 模块，细化权限控制。
  - 调研对接企业身份提供商（OIDC/SAML），减少静态凭据依赖。

## 2025-11-02：账户 API CORS 配置同步

- **背景**：管理端 / 账户 API 在本地通过 Gin 中间件回显请求 Origin；生产前置层（Nginx / Gateway）未同步相同策略，导致浏览器跨域失败。
- **改动**：
  - 新增环境变量 `ADMIN_ALLOWED_ORIGINS`，用于限制允许运营前端访问的白名单，留空则回显请求 `Origin`。
  - `internal/middleware.CORS` 支持基于白名单判断；集成测试沿用统一配置。
  - `deploy/nginx/accounts.conf` 提供示例配置，确保前置 Nginx 与应用层保持一致的 CORS 头。
- **操作指引**：
  - 在生产环境的 Gateway 容器或系统服务中配置 `ADMIN_ALLOWED_ORIGINS=https://admin.example.com,https://ops.example.com`。
  - 更新 Nginx，确保 `map` 中白名单与环境变量一致，并允许 `OPTIONS` 预检请求返回 204。
  - 每次新增前端域名时，同时调整环境变量与 Nginx 配置，避免遗漏。
