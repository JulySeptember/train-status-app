import { defineConfig, devices } from "@playwright/test";

// E2E テスト。/api は e2e/mock.ts で固定の応答を返すので、バックエンドは起動しない
const PORT = 5174;

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [["list"], ["html", { open: "never" }]] : "list",

  use: {
    baseURL: `http://localhost:${PORT}`,
    locale: "ja-JP",
    timezoneId: "Asia/Tokyo",
    trace: "retain-on-failure",
  },

  // mobile.spec.ts はスマホの幅（画面下のタブバー・時刻表のタブ）だけを確かめるので、mobile でだけ動かす
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
      testIgnore: /mobile\.spec\.ts/,
    },
    {
      name: "mobile",
      use: { ...devices["Pixel 7"] },
      testMatch: /mobile\.spec\.ts/,
    },
  ],

  // 開発用の vite（5173）と重ならないポートで起動する
  webServer: {
    command: `npx vite --host localhost --port ${PORT} --strictPort`,
    url: `http://localhost:${PORT}`,
    reuseExistingServer: !process.env.CI,
  },
});
