// Runtime configuration injected by the Go server into index.html.
// In dev (vite), __APP_CONFIG__ is absent and these defaults apply.

interface AppConfig {
  version: string
  firstTime: boolean
  baseURL: string
  auth?: { username: string; name: string; isAdmin: boolean }
}

declare global {
  interface Window {
    __APP_CONFIG__?: AppConfig | string
  }
}

function readConfig(): AppConfig | undefined {
  const raw = window.__APP_CONFIG__
  if (!raw) return undefined
  if (typeof raw === 'string') {
    try {
      return JSON.parse(raw) as AppConfig
    } catch {
      return undefined
    }
  }
  return raw
}

const cfg = readConfig()

export const appConfig: AppConfig = {
  version: cfg?.version ?? 'dev',
  firstTime: cfg?.firstTime ?? false,
  baseURL: cfg?.baseURL ?? '',
}
