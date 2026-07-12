import { defineConfig, devices } from "@playwright/test";

// E2E tests run against a live instance. Point PLAYWRIGHT_BASE_URL at it, or
// default to the docker-compose app on :8080 (which runs in development mode, so
// the dev-login flow the specs use is available).
export default defineConfig({
  testDir: "./e2e",
  timeout: 30_000,
  expect: { timeout: 10_000 },
  fullyParallel: true,
  retries: process.env.CI ? 1 : 0,
  reporter: [["list"]],
  use: {
    baseURL: process.env.PLAYWRIGHT_BASE_URL || "http://localhost:8080",
    trace: "on-first-retry",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
});
