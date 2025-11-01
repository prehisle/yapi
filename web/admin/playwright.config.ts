import { defineConfig, devices } from '@playwright/test'

const uiBaseURL = process.env.PLAYWRIGHT_BASE_URL ?? 'http://127.0.0.1:4173'
const apiBaseEnv = process.env.PLAYWRIGHT_API_BASE_URL

export default defineConfig({
  testDir: './tests',
  timeout: 60_000,
  expect: {
    timeout: 15_000,
  },
  fullyParallel: false,
  reporter: process.env.CI ? 'dot' : 'list',
  use: {
    baseURL: uiBaseURL,
    trace: 'on-first-retry',
    video: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  webServer: {
    command: 'npm run preview -- --host 127.0.0.1 --port 4173',
    port: 4173,
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
    env: apiBaseEnv
      ? {
          ...process.env,
          VITE_API_BASE_URL: apiBaseEnv,
        }
      : process.env,
  },
  projects: [
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome'],
      },
    },
  ],
})
