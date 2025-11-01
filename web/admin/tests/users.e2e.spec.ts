import { expect, test } from '@playwright/test'
import type { APIRequestContext, Page } from '@playwright/test'
import { config as loadEnv } from 'dotenv'
import { existsSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const rootEnvPath = path.resolve(__dirname, '../../..', '.env.local')
if (existsSync(rootEnvPath)) {
  loadEnv({ path: rootEnvPath })
}

const backendBaseURL = process.env.PLAYWRIGHT_API_BASE_URL ?? 'http://localhost:8080'
const adminUsername = process.env.ADMIN_USERNAME ?? 'admin'
const adminPassword = process.env.ADMIN_PASSWORD ?? 'admin123'

test.describe('用户管理端到端', () => {
  test('管理员可创建用户并绑定上游凭据', async ({ page, request }) => {
    await waitForGatewayReady(request)

    const token = await loginViaAPIOrSkip(request)

    const userName = `e2e-user-${Date.now()}`
    const provider = `mock-${Date.now()}`
    const upstreamSecret = `sk-upstream-${Date.now()}`
    const userId = (await createUserViaAPI(request, token, userName)).id

    await page.goto('/')
    await page.locator('input[autocomplete="username"]').fill(adminUsername)
    await page.locator('input[autocomplete="current-password"]').fill(adminPassword)
    await page.getByRole('button', { name: '登录' }).click()

    await expect(page.getByRole('heading', { name: '规则管理' })).toBeVisible()
    await page.getByRole('link', { name: '用户管理' }).click()
    await expect(page.getByRole('heading', { name: '用户管理' })).toBeVisible()

    await page.goto('/users')
    await expect(page.getByRole('heading', { name: '用户管理' })).toBeVisible()
    await page.getByRole('button', { name: '刷新' }).click()
    await waitForUserListRefresh(page, request, token, userName)
    const userRow = page.locator('table tbody tr', { hasText: userName })
    await userRow.first().getByRole('button', { name: '查看详情' }).click()

    await page.getByRole('button', { name: '生成 API Key' }).click()
    const apiKeyDialog = page.locator('.dialog').filter({ hasText: `为 ${userName} 生成密钥` })
    await expect(apiKeyDialog).toBeVisible()
    await apiKeyDialog.locator('input[name="label"]').fill('primary')
    await apiKeyDialog.getByRole('button', { name: '生成' }).click()

    const secretAlert = apiKeyDialog.locator('.alert--info code')
    await expect(secretAlert).toBeVisible()
    const clientSecret = (await secretAlert.textContent())?.trim()
    expect(clientSecret).toBeTruthy()
    await apiKeyDialog.locator('.dialog__close').click()

    await page.getByRole('button', { name: '新建凭据' }).click()
    const upstreamDialog = page.locator('.dialog').filter({ hasText: '新建上游凭据' })
    await upstreamDialog.locator('.field').filter({ hasText: '服务商' }).locator('input').fill(provider)
    await upstreamDialog.locator('.field').filter({ hasText: '标签' }).locator('input').fill('primary')
    await upstreamDialog.locator('.field').filter({ hasText: '上游 API Key' }).locator('input').fill(upstreamSecret)
    await upstreamDialog
      .locator('.field')
      .filter({ hasText: '可用地址' })
      .locator('textarea')
      .fill('https://example.com/v1')
    await upstreamDialog.getByRole('button', { name: '保存' }).click()

    const upstreamRow = page.locator('table').last().locator('tbody tr').filter({ hasText: provider })
    await expect(upstreamRow).toBeVisible()

    const apiKeyPrefix = clientSecret!.split('_')[1]
    await waitForAPIKeyRow(page, request, token, userId, apiKeyPrefix)
    const apiKeyRow = page
      .locator('.card')
      .filter({ hasText: 'API Key' })
      .locator('table tbody tr', { hasText: apiKeyPrefix })

    const select = apiKeyRow.locator('select')
    await expect(select).toBeVisible()
    await select.selectOption({ index: 1 })

    await expect(apiKeyRow.getByText('已绑定')).toBeVisible()
    await expect(apiKeyRow.getByText(`${provider} · primary`)).toBeVisible()

    const keys = await listUserAPIKeys(request, token, userId)
    const apiKey = keys.items.find((item) => item.prefix === apiKeyPrefix)
    expect(apiKey).toBeTruthy()
    const binding = await getBindingViaAPI(request, token, apiKey!.id)
    expect(binding.upstream.provider).toBe(provider)
    expect(binding.upstream.label).toBe('primary')
  })
})

async function waitForGatewayReady(request: APIRequestContext) {
  for (let attempt = 0; attempt < 20; attempt++) {
    const response = await request.get(`${backendBaseURL}/admin/healthz`)
    if (response.ok()) {
      return
    }
    await new Promise((resolve) => setTimeout(resolve, 500))
  }
  throw new Error(`网关后端 ${backendBaseURL} 未就绪，无法执行端到端测试`)
}

async function waitForUserListRefresh(
  page: Page,
  request: APIRequestContext,
  token: string,
  userName: string,
) {
  const refreshButton = page.getByRole('button', { name: '刷新' })

  // 修复：使用更宽松的方式来检查刷新状态
  // 由于按钮禁用状态很短暂，我们主要等待文本变化和最终完成状态
  try {
    // 尝试等待按钮变为禁用状态（可能很短暂）
    await expect(refreshButton).toBeDisabled({ timeout: 2000 })
  } catch {
    // 如果错过了禁用状态，继续等待按钮恢复启用状态
    // 这表示刷新操作可能已经很快完成了
  }

  // 等待按钮恢复启用状态（表示刷新完成）
  await expect(refreshButton).toBeEnabled({ timeout: 20000 })

  // 验证用户数据已刷新
  await expect
    .poll(
      async () => {
        const users = await listUsersViaAPI(request, token)
        if (!users.items.some((item) => item.name === userName)) {
          return false
        }
        return (await page.locator('table tbody tr', { hasText: userName }).count()) > 0
      },
      {
        message: `用户 ${userName} 未在刷新后的列表中找到`,
        timeout: 20000,
      },
    )
    .toBeTruthy()
}

async function waitForAPIKeyRow(
  page: Page,
  request: APIRequestContext,
  token: string,
  userId: string,
  apiKeyPrefix: string,
) {
  await expect
    .poll(
      async () => {
        const list = await listUserAPIKeys(request, token, userId)
        if (!list.items.some((item) => item.prefix === apiKeyPrefix)) {
          return false
        }
        const rowCount = await page
          .locator('.card')
          .filter({ hasText: 'API Key' })
          .locator('table tbody tr', { hasText: apiKeyPrefix })
          .count()
        return rowCount > 0
      },
      { timeout: 20000, message: `API Key ${apiKeyPrefix} 未出现在列表中` },
    )
    .toBeTruthy()
}

async function loginViaAPIOrSkip(request: APIRequestContext): Promise<string> {
  const response = await request.post(`${backendBaseURL}/admin/login`, {
    data: { username: adminUsername, password: adminPassword },
  })
  if (response.status() === 501) {
    test.skip(true, '当前环境未启用管理端 Token 登录')
  }
  expect(response.ok()).toBeTruthy()
  const payload = await response.json()
  expect(payload?.access_token).toBeTruthy()
  return payload.access_token as string
}

async function createUserViaAPI(request: APIRequestContext, token: string, name: string) {
  const response = await request.post(`${backendBaseURL}/admin/users`, {
    headers: { Authorization: `Bearer ${token}` },
    data: { name, description: 'e2e user' },
  })
  expect(response.ok(), `create user failed: ${await response.text()}`).toBeTruthy()
  return (await response.json()) as { id: string; name: string }
}

async function listUsersViaAPI(request: APIRequestContext, token: string) {
  const response = await request.get(`${backendBaseURL}/admin/users`, {
    headers: { Authorization: `Bearer ${token}` },
  })
  expect(response.ok()).toBeTruthy()
  return (await response.json()) as {
    items: Array<{ id: string; name: string; description?: string }>
  }
}

async function listUserAPIKeys(request: APIRequestContext, token: string, userId: string) {
  const response = await request.get(`${backendBaseURL}/admin/users/${userId}/api-keys`, {
    headers: { Authorization: `Bearer ${token}` },
  })
  expect(response.ok()).toBeTruthy()
  return (await response.json()) as { items: Array<{ id: string; prefix: string }> }
}

async function getBindingViaAPI(request: APIRequestContext, token: string, apiKeyId: string) {
  const response = await request.get(`${backendBaseURL}/admin/api-keys/${apiKeyId}/binding`, {
    headers: { Authorization: `Bearer ${token}` },
  })
  expect(response.ok()).toBeTruthy()
  return (await response.json()) as { upstream: { provider: string; label: string } }
}
