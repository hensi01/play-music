// Playwright config — e2e suite for Play Music.
// The Go server usually runs on :4533 (started by m1-start.ps1). We never
// spawn `go run`; we reuse the running server (reuseExistingServer: true).
// PM_E2E_BASEURL overrides the target (validation servers on other ports).
// If the server is down, start-server.ps1 boots it via m1-start.ps1.

const { defineConfig } = require('playwright/test')

const BASE = process.env.PM_E2E_BASEURL || 'http://localhost:4533'

module.exports = defineConfig({
  testDir: './tests',
  timeout: 45_000,
  expect: { timeout: 10_000 },
  fullyParallel: false,
  workers: 1, // shared DB state + PWA overlay timing: serialize
  retries: 0,
  reporter: [['list']],
  use: {
    baseURL: BASE,
    headless: true,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    viewport: { width: 1280, height: 720 },
    launchOptions: {
      // The audio element must be allowed to play without a user gesture in
      // the player spec (delegated pointer seek/play assertions).
      args: ['--autoplay-policy=no-user-gesture-required'],
    },
  },
  projects: [
    { name: 'desktop', use: { viewport: { width: 1280, height: 720 } } },
    // Mobile uses Chromium (the WebKit build is not installed in this env)
    // with a 375px viewport, touch + mobile UA to exercise the responsive layout.
    {
      name: 'mobile',
      use: {
        browserName: 'chromium',
        viewport: { width: 375, height: 812 },
        isMobile: true,
        hasTouch: true,
      },
    },
  ],
  webServer: {
    command: 'powershell -NoProfile -ExecutionPolicy Bypass -File e2e\\start-server.ps1',
    url: BASE,
    reuseExistingServer: true,
    timeout: 60_000,
  },
})
