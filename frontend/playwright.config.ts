import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: './e2e',
  timeout: 30_000,
  retries: 0,
  workers: 1,
  use: {
    baseURL: 'http://localhost:8080',
    ...devices['Pixel 5'],
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Pixel 5'] },
    },
  ],
  webServer: {
    command: 'cd .. && rm -f /tmp/playwright-test.db && npm run --prefix frontend build && TEST_MODE=true DB_PATH=/tmp/playwright-test.db go run .',
    url: 'http://localhost:8080/health',
    reuseExistingServer: !process.env.CI,
    timeout: 60_000,
  },
})
