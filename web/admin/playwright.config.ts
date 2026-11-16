import { defineConfig } from "@playwright/test";
import { apiOrigin, uiOrigin } from "./tests/stack";
// Runs only through `make e2e` (scripts/e2e.py), which starts a disposable stack.
export default defineConfig({
  testDir: "tests",
  webServer: {
    command: `npm run dev -- --strictPort --port ${new URL(uiOrigin).port}`,
    url: uiOrigin + "/console/",
    env: { LASTAPP_API_ORIGIN: apiOrigin },
    // Never reuse a Vite server: it could proxy to the development API.
    reuseExistingServer: false,
    timeout: 30000,
  },
  fullyParallel: false,
  workers: 1,
  timeout: 60000,
  use: {
    baseURL: apiOrigin,
    headless: true,
    channel: process.env.CI ? "chromium" : "chrome",
    screenshot: "only-on-failure",
    trace: "retain-on-failure",
  },
  reporter: [["list"], ["html", { open: "never" }]],
});
