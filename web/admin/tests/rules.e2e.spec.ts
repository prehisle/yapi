import { expect, test, type Locator, type APIRequestContext } from '@playwright/test'
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

test.describe('管理后台端到端', () => {
  test('管理员可登录并完成规则 CRUD 流程', async ({ page, request }) => {
    await waitForGatewayReady(request)

    const token = await loginViaAPIOrSkip(request)
    const ruleId = `e2e-${randomUUID().slice(0, 8)}`
    await ensureRuleAbsent(request, token, ruleId)

    await page.goto('/')
    await page.locator('input[autocomplete="username"]').fill(adminUsername)
    await page.locator('input[autocomplete="current-password"]').fill(adminPassword)
    await page.getByRole('button', { name: '登录' }).click()

    await expect(page.getByRole('heading', { name: '规则管理' })).toBeVisible()

    await page.getByRole('button', { name: '新建规则' }).click()
    const dialog = page.locator('.dialog--wide').first()
    await expect(dialog).toBeVisible()

    await fillField(dialog, '规则 ID', ruleId)
    await fillField(dialog, '优先级', '900')
    await fillField(dialog, '路径前缀', `/e2e/${ruleId}`)
    await fillField(dialog, '方法列表', 'POST')
    await fillField(dialog, '目标地址', 'https://example.com')
    await fillField(dialog, 'override_json', '{"model":"gpt-4o"}')
    await fillField(dialog, 'remove_json', 'metadata.debug')

    await dialog.evaluate((form: HTMLFormElement) => {
      form.requestSubmit()
    })

    await expect(page.getByRole('status').getByText(`规则 ${ruleId} 已保存`)).toBeVisible()

    const ruleRow = page.locator('table tbody tr', { hasText: ruleId })
    await expect(ruleRow).toBeVisible()
    await expect(ruleRow.locator('td').nth(2)).toHaveText(`/e2e/${ruleId}`)
    await expect(ruleRow.getByText('已启用')).toBeVisible()

    await ruleRow.getByRole('button', { name: '删除' }).click()
    const confirmDialog = page.getByRole('alertdialog')
    await expect(confirmDialog).toBeVisible()
    await confirmDialog.getByRole('button', { name: '删除' }).click()

    await expect(page.getByRole('status').getByText(`规则 ${ruleId} 已删除`)).toBeVisible()
    await expect(ruleRow).toHaveCount(0)
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
  const response = await request.delete(`${backendBaseURL}/admin/rules/${ruleId}`, {
    headers: { Authorization: `Bearer ${token}` },
  })
  if (response.status() === 404) {
    return
  }
  expect(response.ok()).toBeTruthy()
}

async function fillField(dialog: Locator, label: string, value: string) {
  const field = dialog.locator('.field').filter({ hasText: label }).first()
  const input = field.locator('input, textarea').first()
  await expect(input).toBeVisible()
  await input.fill(value)
}
