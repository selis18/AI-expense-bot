import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  workers: 1,
  use: {
    baseURL: process.env.PANEL_URL || 'http://127.0.0.1:8080',
    channel: process.env.BROWSER_CHANNEL || undefined,
    screenshot: 'only-on-failure',
  },
});
