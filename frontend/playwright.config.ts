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
    // A throwaway local PostgreSQL database per run (production is Postgres,
    // so the end-to-end suite must be too). TEST_MODE enables /auth/test-login
    // and nothing in this command reads the repo's .env secrets for sending:
    // Twilio and Clerk are left unconfigured.
    command:
      'cd .. && dropdb --if-exists pennyai_playwright && createdb pennyai_playwright && npm run --prefix frontend build && ' +
      'env -u TWILIO_ACCOUNT_SID -u TWILIO_AUTH_TOKEN TEST_MODE=true PORT=8080 ' +
      'DATABASE_URL="postgres://localhost:5432/pennyai_playwright?sslmode=disable" go run .',
    url: 'http://localhost:8080/health',
    reuseExistingServer: !process.env.CI,
    timeout: 60_000,
  },
})
