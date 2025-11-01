import { expect, test } from '@playwright/test'
import type { APIRequestContext } from '@playwright/test'
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

    await loginViaAPIOrSkip(request)

    const userName = `e2e-user-${Date.now()}`
    const provider = `mock-${Date.now()}`
    const upstreamSecret = `sk-upstream-${Date.now()}`

    await page.goto('/')
    await page.locator('input[autocomplete="username"]').fill(adminUsername)
    await page.locator('input[autocomplete="current-password"]').fill(adminPassword)
    await page.getByRole('button', { name: '登录' }).click()

    await expect(page.getByRole('heading', { name: '规则管理' })).toBeVisible()
    await page.getByRole('link', { name: '用户管理' }).click()
    await expect(page.getByRole('heading', { name: '用户管理' })).toBeVisible()

    await page.getByRole('button', { name: '新建用户' }).click()
    const createUserDialog = page.locator('.dialog').first()
    await expect(createUserDialog).toBeVisible()
    await createUserDialog.locator('.field').filter({ hasText: '用户名称' }).locator('input').fill(userName)
    await createUserDialog.locator('.field').filter({ hasText: '描述' }).locator('textarea').fill('端到端测试用户')
    await createUserDialog.getByRole('button', { name: '保存' }).click()
    await expect(createUserDialog).not.toBeVisible()

    const userRow = page.locator('table tbody tr', { hasText: userName })
    await expect(userRow).toBeVisible()
    await userRow.getByRole('button', { name: '查看详情' }).click()

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
    const apiKeyRow = page.locator('table').first().locator('tbody tr').filter({ hasText: apiKeyPrefix })
    await expect(apiKeyRow).toBeVisible()

    const select = apiKeyRow.locator('select')
    await expect(select).toBeVisible()
    await select.selectOption({ index: 1 })

    await expect(apiKeyRow.getByText('已绑定')).toBeVisible()
    await expect(apiKeyRow.getByText(`${provider} · primary`)).toBeVisible()
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
