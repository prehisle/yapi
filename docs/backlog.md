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

> Estimates are rough ideal days; refine during sprint planning.
