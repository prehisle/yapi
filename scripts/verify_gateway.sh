#!/usr/bin/env bash

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")"/.. && pwd)"
cd "$PROJECT_ROOT"

BASE_URL="${GATEWAY_URL:-http://localhost:8080}"
HEALTH_URL="${BASE_URL%/}/admin/healthz"
METRICS_URL="${BASE_URL%/}/metrics"

COMPOSE_FILE="${PROJECT_ROOT}/deploy/docker-compose.yml"
start_compose() {
  echo "启动 docker compose 栈..." >&2
  docker compose -f "$COMPOSE_FILE" up -d gateway >/dev/null
}

stop_compose() {
  echo "停止 docker compose 栈..." >&2
  docker compose -f "$COMPOSE_FILE" down >/dev/null
}

if [[ "${START_COMPOSE:-}" == "true" ]]; then
  trap stop_compose EXIT
  start_compose
  # 等待服务启动
  for _ in {1..10}; do
    if curl -fsS -o /dev/null "$HEALTH_URL" 2>/dev/null; then
      break
    fi
    sleep 2
  done
fi

echo "[1/3] 校验健康检查: ${HEALTH_URL}" >&2
curl -fsS -o /dev/null "$HEALTH_URL"

echo "[2/3] 检查 Prometheus 指标: ${METRICS_URL}" >&2
METRICS_PAYLOAD="$(curl -fsS "$METRICS_URL")"
if ! grep -q "gateway_http_requests_total" <<<"$METRICS_PAYLOAD"; then
	echo "未在 /metrics 输出中检测到 gateway_http_requests_total 指标" >&2
	exit 1
fi

echo "[3/3] 执行 SSE 回归测试 (TestHandler_StreamPassthrough)" >&2
go test ./internal/proxy -run TestHandler_StreamPassthrough -count=1

echo "验证完成" >&2
