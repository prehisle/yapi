import { expect, test, type APIRequestContext, type Locator } from '@playwright/test'
import { randomUUID } from 'node:crypto'
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

test.describe('账户上下文字段', () => {
  test('规则表单展示与校验账户上下文字段', async ({ page, request }) => {
    await waitForGatewayReady(request)
    const token = await loginViaAPIOrSkip(request)
    const ruleId = `acct-${randomUUID().slice(0, 8)}`
    await ensureRuleAbsent(request, token, ruleId)

    const pathPrefix = `/ui-test/${ruleId}`
    const targetURL = 'http://localhost:9107'
    const apiKeyPrefix = 'abcd1234'
    const userId = `user-${ruleId}`
    const upstreamId = `cred-${ruleId}`
    const provider = `openai-${ruleId.slice(0, 4)}`

    try {
      await page.goto('/')
      await page.locator('input[autocomplete="username"]').fill(adminUsername)
      await page.locator('input[autocomplete="current-password"]').fill(adminPassword)
      await page.getByRole('button', { name: '登录' }).click()

      await expect(page.getByRole('heading', { name: '规则管理' })).toBeVisible()
      await page.getByRole('button', { name: '新建规则' }).click()

      const dialog = page.locator('.dialog.dialog--wide').first()
      await expect(dialog).toBeVisible()
      await expect(dialog.getByText('账户上下文匹配')).toBeVisible()

      await fillField(dialog, '规则 ID', ruleId)
      await fillField(dialog, '优先级', '950')
      await fillField(dialog, '路径前缀', pathPrefix)
      await fillField(dialog, '目标地址', targetURL)

      await fillField(dialog, 'API Key 前缀', 'short')
      const prefixError = dialog
        .locator('.field')
        .filter({ hasText: 'API Key 前缀' })
        .locator('.field__error', { hasText: 'API Key 前缀需为 8 位字母或数字' })
      await expect(prefixError).toBeVisible()

      await fillField(dialog, 'API Key 前缀', apiKeyPrefix)
      await fillField(dialog, 'API Key ID', `key-${ruleId}`)
      await fillField(dialog, '用户 ID', userId)
      await fillField(dialog, '用户元数据匹配', 'tier=gold\nregion=us')
      await fillField(dialog, '上游凭据 ID', upstreamId)
      await fillField(dialog, '上游 Provider', provider)

      const requireBindingCheckbox = dialog
        .locator('label.field--inline', { hasText: '仅在存在上游绑定时匹配' })
        .locator('input[type="checkbox"]')
      await requireBindingCheckbox.check()

      await dialog.evaluate((form: HTMLFormElement) => {
        form.requestSubmit()
      })
      await expect(page.getByRole('status').getByText(`规则 ${ruleId} 已保存`)).toBeVisible()

      const ruleRow = page.locator('table tbody tr', { hasText: ruleId })
      await expect(ruleRow).toBeVisible()
      await ruleRow.getByRole('button', { name: '详情' }).click()

      const drawer = page.locator('.drawer').first()
      await expect(drawer).toBeVisible()
      await expect(drawer.getByText('账户上下文匹配')).toBeVisible()

      const requireBindingValue = drawer
        .locator('dt', { hasText: 'Require Binding' })
        .locator('..')
        .locator('dd')
      await expect(requireBindingValue).toHaveText('是')

      await expect(
        drawer
          .locator('.drawer-table')
          .filter({ hasText: 'API Key 前缀' })
          .locator('li'),
      ).toContainText(apiKeyPrefix)
      await expect(
        drawer
          .locator('.drawer-table')
          .filter({ hasText: '用户 ID' })
          .locator('li'),
      ).toContainText(userId)
      const metadataRows = drawer
        .locator('.drawer-table')
        .filter({ hasText: '用户 Metadata' })
        .locator('table tr')
      await expect(metadataRows).toHaveCount(2)
      const metadataEntries = await metadataRows.evaluateAll((rows) =>
        rows.map((row) =>
          Array.from(row.querySelectorAll('td')).map((cell) => cell.textContent?.trim() ?? ''),
        ),
      )
      expect(metadataEntries).toContainEqual(['tier', 'gold'])
      expect(metadataEntries).toContainEqual(['region', 'us'])
      await expect(
        drawer
          .locator('.drawer-table')
          .filter({ hasText: '上游凭据 ID' })
          .locator('li'),
      ).toContainText(upstreamId)
      await expect(
        drawer
          .locator('.drawer-table')
          .filter({ hasText: '上游 Provider' })
          .locator('li'),
      ).toContainText(provider)

      await drawer.locator('.drawer__close').click()
    } finally {
      await ensureRuleAbsent(request, token, ruleId)
    }
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

async function ensureRuleAbsent(request: APIRequestContext, token: string, ruleId: string) {
  try {
    const response = await request.delete(`${backendBaseURL}/admin/rules/${ruleId}`, {
      headers: { Authorization: `Bearer ${token}` },
    })
    if (response.status() === 404) {
      return
    }
    expect(response.ok()).toBeTruthy()
  } catch (error) {
    if (`${error}`.includes('Target page, context or browser has been closed')) {
      return
    }
    throw error
  }
}

async function fillField(container: Locator, label: string, value: string) {
  const field = container.locator('.field').filter({ hasText: label }).first()
  const input = field.locator('input, textarea').first()
  await expect(input).toBeVisible()
  await input.fill(value)
  await input.press('Tab')
}
