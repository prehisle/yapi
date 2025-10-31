# 监控与告警指引

本文档为运维/值班人员提供基于网关 `/metrics` 指标的监控基线。

## Prometheus 抓取配置

```yaml
scrape_configs:
  - job_name: 'yapi-gateway'
    scrape_interval: 15s
    metrics_path: /metrics
    static_configs:
      - targets: ['yapi-gateway.default.svc.cluster.local:8080']
```

若部署使用 TLS/自定义路径，可通过 `relabel_configs` 调整 URL，或在 Ingress/Nginx 层做转发。

> 本地联调可直接运行 `docker compose -f deploy/docker-compose.monitoring.yml up`，将同时启动 Gateway、Prometheus、Grafana。Grafana 默认监听 `http://localhost:3000`，使用 `admin/admin` 登录后即可看到自动导入的 “YAPI Gateway Overview” 面板。

## 关键指标

- `gateway_http_requests_total{route="/admin/healthz"}`：健康检查的请求量，结合 `increase()` 判断实例是否存活。
- `gateway_http_request_duration_seconds_bucket`：网关请求延迟分布，建议关注 `route="<unmatched>"`（命中默认Proxy）与核心业务路由。
- `gateway_upstream_latency_seconds_bucket{upstream="api.openai.com"}`：上游 LLM 延迟直方图，可拆分成功/失败 outcome。
- `process_open_fds`、`go_goroutines`：Go runtime 默认指标，辅助判断资源泄漏。

## Grafana 面板示例

1. **整体 QPS**：`sum(rate(gateway_http_requests_total[5m])) by (route)`。
2. **P95 延迟**：`histogram_quantile(0.95, sum(rate(gateway_http_request_duration_seconds_bucket[5m])) by (le, route))`。
3. **上游错误率**：`sum(rate(gateway_upstream_latency_seconds_count{outcome="error"}[5m])) / sum(rate(gateway_upstream_latency_seconds_count[5m]))`。
4. **Redis 事件**：监控 `redis_up`、`redis_connected_clients`（由外部 exporter 提供）与网关日志 EventBus 失败次数。

## 告警建议

- **实例无请求**：`absent_over_time(gateway_http_requests_total{route="/admin/healthz"}[5m])`，提示实例被摘或健康检查异常。
- **高延迟**：`histogram_quantile(0.95, sum(rate(gateway_http_request_duration_seconds_bucket[5m])) by (le)) > 1` 持续 10m。
- **上游错误率过高**：`(sum(rate(gateway_upstream_latency_seconds_count{outcome="error"}[5m])) / sum(rate(gateway_upstream_latency_seconds_count[5m]))) > 0.1` 持续 5m。
- **Redis 事件总线失败**：根据日志或未来的 `gateway_rules_event_errors_total` 指标（预留），超过阈值时告警并自动切换本地缓存策略。

## 定期校验

- 每次发布前执行 `make verify`，确认健康检查、指标与 SSE 回归正常。
- 一周一次校验 Grafana 面板时间范围、告警渠道是否生效，并更新运行手册。
- 若新增指标/规则，记得更新本文档以保持运维一致性。
