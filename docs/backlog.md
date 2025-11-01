# MVP Backlog

| ID | Task | Priority | Owner | Estimate |
| --- | --- | --- | --- | --- |
| FR-01 | Build HTTP reverse proxy service skeleton using Go with streaming passthrough | High | Alice (Backend) | 3d |
| FR-02 | Ensure chunked/SSE streaming survives through proxy with zero buffering | High | Alice (Backend) | 2d |
| FR-03 | Implement declarative rule engine (match + action) | High | Bob (Platform) | 3d |
| FR-04 | Add matchers for path prefix, method, and header regex | High | Bob (Platform) | 2d |
| FR-05 | Support actions for target URL rewrite and header manipulation | High | Chen (Integration) | 2d |
| FR-06 | Deliver basic web admin (CRUD for rules, enable/disable) | High | Diana (Admin UI) | 4d |
| FR-07 | Wire Redis pub/sub so rule changes apply instantly | High | Bob (Platform) | 2d |
| FR-08 | Apply rule priority resolution | Medium | Bob (Platform) | 1d |
| FR-09 | Allow optional JSON body mutation before forward | Medium | Chen (Integration) | 2d |
| FR-10 | Research sandboxed JavaScript action support via goja | Low | Eva (R&D) | 3d |
| NFR-03 | Add admin auth + secret management flow | High | Frank (Security) | 3d |
| NFR-05 | Expose metrics/logging for observability | Medium | Grace (Observability) | 2d |

## Delivered · 2025-11-02
- ✅ FR-11：账户服务与管理端 API 已落地，覆盖用户、API Key、上游凭据及绑定。
- ✅ FR-12：代理链路引入 API Key 鉴权及上游绑定选路逻辑，集成测试通过。
- ✅ FR-13：管理后台用户/凭据管理 UI 及端到端脚本发布，支持 Playwright 回归。

> Estimates are rough ideal days; refine during sprint planning.
