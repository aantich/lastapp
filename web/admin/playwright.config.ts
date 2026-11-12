import { defineConfig } from "@playwright/test";
export default defineConfig({
  testDir: "tests",
  webServer: {
    command: "npm run dev -- --strictPort",
    url: "http://127.0.0.1:5173/console/",
    reuseExistingServer: !process.env.CI,
    timeout: 30000,
  },
  fullyParallel: false,
  workers: 1,
  timeout: 60000,
  use: {
    baseURL: "http://127.0.0.1:8787",
    headless: true,
    channel: process.env.CI ? "chromium" : "chrome",
    screenshot: "only-on-failure",
    trace: "retain-on-failure",
  },
  reporter: [["list"], ["html", { open: "never" }]],
});
