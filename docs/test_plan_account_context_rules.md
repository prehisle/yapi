# 账户上下文规则测试计划

本文档面向测试团队，覆盖近期新增的「规则匹配支持账户上下文」特性。目标是验证 Gateway、管理端以及账户绑定流程在不同场景下的行为是否符合预期。

## 1. 测试范围
- 规则匹配新增字段的后端行为：`api_key_ids`、`api_key_prefixes`、`user_ids`、`user_metadata`、`binding_upstream_ids`、`binding_providers`、`require_binding`。
- 代理层对账户上下文字段的解析、匹配与回退逻辑。
- 管理后台规则表单/详情对新字段的展示、保存与校验。
- 端到端请求命中正确规则并注入对应上游凭据。

## 2. 前置条件
1. 环境：
   - Docker / Docker Compose 可用。
   - Node.js ≥ 18、npm、Go ≥ 1.21 已安装。
2. 启动依赖：
   ```bash
   docker compose -f deploy/docker-compose.yml up -d postgres redis httpbin
   ```
3. 准备环境变量：
   ```bash
   cp .env.example .env.local
   # 修改为以下关键内容（仅示例）：
   GATEWAY_PORT=9110
   DATABASE_DSN=postgres://yapi:yapi@localhost:9109/yapi?sslmode=disable
   REDIS_ADDR=localhost:9108
   ADMIN_USERNAME=admin
   ADMIN_PASSWORD=adminpass
   ADMIN_TOKEN_SECRET=dev-secret-change-me
   ADMIN_ALLOWED_ORIGINS=http://localhost:5173
   ```
4. 启动后端：
   ```bash
   source .env.local
   go run ./cmd/gateway
   ```
5. 启动管理端（如需）：
   ```bash
   cd web/admin
   npm install
   npm run dev
   ```
6. 初始化测试数据：使用管理端或 API 创建以下资源（可通过 `curl` 或 Postman 完成）：
   - 用户：`user-1`（metadata: `{ "tier": "gold", "region": "us" }`）、`user-2`（metadata: `{ "tier": "silver" }`）。
   - API Key：
     - `user-1`：密钥标签 `primary`，记录生成的前缀（例如 `abcd1234`）与完整密钥。
     - `user-2`：密钥标签 `backup`。
   - 上游凭据：
     - `credential-1`：`user-1` 对应 provider `openai`，endpoint `https://httpbin`。
     - `credential-2`：`user-2` 对应 provider `vertex`，endpoint `https://httpbin`。
   - 绑定：
     - `user-1` 的 `primary` 密钥绑定 `credential-1`。
     - `user-2` 的 `backup` 密钥绑定 `credential-2`。

## 3. 测试用例

### 3.1 API Key 前缀命中规则
| 用例编号 | 描述 | 步骤 | 预期 |
| --- | --- | --- | --- |
| C01 | 按 API Key 前缀命中 | 1）在管理端创建规则 A：<br/>- `matcher.path_prefix=/v1/chat`<br/>- `matcher.api_key_prefixes=["{user1前缀}"]`<br/>- `matcher.require_binding=true`<br/>- `actions.set_target_url=https://httpbin`<br/>2）使用 `user-1` 的 API Key 发起请求：<br/>```curl -H "Authorization: Bearer {user1密钥}" -d '{"messages":[{"role":"user","content":"hi"}]}' http://localhost:9110/v1/chat``` | 接口返回 200，`httpbin` 收到请求，头部包含 `X-Upstream-Provider: openai`、`X-Upstream-Credential-ID: credential-1`。 |
| C02 | 前缀不匹配落到其他规则 | 1）保持规则 A，新增规则 B（优先级更低）只配置 `path_prefix=/v1/chat`<br/>2）使用 `user-2` 的 API Key 调用相同接口 | 请求命中规则 B，`X-Upstream-Provider` 应为 `vertex`（由绑定自动注入），响应 200。 |

### 3.2 用户 ID 精确匹配
| 编号 | 描述 | 步骤 | 预期 |
| --- | --- | --- | --- |
| C03 | 用户 ID 匹配成功 | 1）创建规则 C：`user_ids=["user-2"]`，`require_binding=true`，其他条件同规则 A。<br/>2）使用 `user-2` 密钥请求 `/v1/chat` | 命中规则 C，代理日志显示命中 `user_ids` 条件，响应 200。 |
| C04 | 用户 ID 匹配失败 | 1）沿用规则 C。<br/>2）使用匿名请求（无密钥）或 `user-1` 密钥调用 | 返回 404（默认无命中），或命中其他兜底规则。 |

### 3.3 用户元数据匹配
| 编号 | 描述 | 步骤 | 预期 |
| --- | --- | --- | --- |
| C05 | 元数据匹配成功 | 1）创建规则 D：`user_metadata={"tier":"gold","region":"us"}`。<br/>2）使用 `user-1` 密钥调用 | 命中规则 D，响应 200。 |
| C06 | 元数据缺失 | 使用 `user-2` 密钥调用 | 未命中规则 D。 |

### 3.4 上游凭据与 Provider
| 编号 | 描述 | 步骤 | 预期 |
| --- | --- | --- | --- |
| C07 | 指定上游凭据 | 规则 E：`binding_upstream_ids=["credential-1"]`。使用 `user-1` 密钥请求 | 命中规则 E，`X-Upstream-Credential-ID=credential-1`。 |
| C08 | 指定 Provider | 规则 F：`binding_providers=["vertex"]`。使用 `user-2` 密钥请求 | 命中规则 F，`X-Upstream-Provider=vertex`。|
| C09 | Provider 不匹配 | 使用 `user-1` 密钥请求规则 F | 未命中规则 F。 |

### 3.5 Require Binding 行为
| 编号 | 描述 | 步骤 | 预期 |
| --- | --- | --- | --- |
| C10 | require_binding=true 时无密钥 | 规则 G：`require_binding=true`，仅匹配 `path_prefix=/v1/chat`。匿名请求（无 Authorization）访问 | 返回 404（未命中）。 |
| C11 | require_binding=false 时无密钥 | 将规则 G 的 `require_binding` 改为 false，匿名请求同路径 | 命中规则 G，响应 200（采用默认 target）。 |

### 3.6 管理端交互
| 编号 | 描述 | 步骤 | 预期 |
| --- | --- | --- | --- |
| UI01 | 表单展示 | 打开 “新建规则” 对话框 | “账户上下文匹配” 分组可见，字段为空。 |
| UI02 | 表单校验 | 在 “API Key 前缀” 输入非 8 位字符 | 底部报错 “API Key 前缀需为 8 位字母或数字”。 |
| UI03 | 详情展示 | 保存包含账户字段的规则，打开详情抽屉 | 对应字段展示在“账户上下文匹配”区块。 |

### 3.7 回归 Playwright 测试（可选自动化）
| 编号 | 描述 | 步骤 | 预期 |
| --- | --- | --- | --- |
| PW01 | 新字段保存 | 在 `web/admin/tests` 新增/复用脚本，创建包含 `api_key_prefixes` 的规则，提交后 API 返回 201 | Playwright 检查表格数据显示新字段。 |
| PW02 | 请求命中验证 | 通过 Playwright 调用后端 API 或注入断言 | 请求返回 200，覆盖自动化流程。 |

## 4. 数据恢复与清理
- 测试完成后，可通过管理端或 API 删除新增规则。
- 清理账户数据：
  ```bash
  docker compose -f deploy/docker-compose.yml down -v
  ```
- 若需保留数据库，建议在测试前后执行备份/恢复脚本或使用独立数据库实例。

## 5. 参考资料
- README: “高级匹配条件”章节。
- `pkg/rules/model.go`、`internal/proxy/handler.go` 实现细节。
- `web/admin/src/components/RuleFormDialog.tsx` 与 `RuleDetailDrawer.tsx` 前端交互。
- 账户绑定 API：`/admin/users/:id/api-keys`、`/admin/api-keys/:id/binding`、`/admin/users/:id/upstreams`。
