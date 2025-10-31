# GitHub Actions 运行手册

该项目默认启用 `.github/workflows/ci.yml`，在 `main` 分支 push 与任意 PR 时触发。流水线阶段如下：

1. **Checkout + Go 环境**：使用 `actions/checkout@v4` 与 `setup-go@v5`，固定版本 `go1.25.3`。
2. **依赖缓存**：启用 `actions/setup-go` 自带的 module cache，加速 `go mod download`。
3. **单元测试**：执行 `go test ./...`，若存在 `-tags compose_test` 用例，请在后续阶段补充。
4. **静态检查**：通过 `golangci-lint run ./...` 统一执行 lint。
5. **集成验证**：调用 `make verify`，自动启动 Docker Compose、检查 `/admin/healthz`、`/metrics`，并运行 SSE 回归测试。

## 首次运行关注点

- **Secrets**：流水线默认不需要额外密钥。如需连接外部镜像仓库或通知渠道，可在仓库 Settings → Secrets and variables 中配置。
- **Compose 资源**：`make verify` 会在 Actions runner 内部拉起 Redis/PostgreSQL/Gateway，已在脚本结束时自动清理；若流水线失败，请先查看 `verify_gateway.sh` 的日志，确认容器是否退出。
- **执行时间**：首次拉取镜像约 2 分钟，后续缓存后稳定在 1 分钟左右。

## 失败排查流程

1. 打开 GitHub → Actions → 选中失败的 workflow run。
2. 检查 `Go test`、`golangci-lint`、`Gateway verify script` 三个步骤的日志，定位异常。
3. 若 `Gateway verify script` 失败，可本地执行 `START_COMPOSE=true ./scripts/verify_gateway.sh` 复现。
4. 针对镜像拉取或网络超时，可重新运行该 job（Run workflow → Re-run jobs）；若持续失败，建议在企业 Runner 上部署所需镜像缓存。

## 持续改进

- 可追加 SAST/依赖漏洞扫描步骤（如 `github/codeql-action`、`trivy`）。
- 若出现长时间队列，可在组织层级创建自定义 runner 或开启并发缓存。
- 结合 `docs/monitoring.md` 中的指标，后续可将 `make verify` 输出的关键结果推送至 Slack/飞书。
