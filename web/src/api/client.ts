// Thin fetch wrapper that attaches the JWT and follows token refresh
// (the server renews the token on every authenticated response).

const AUTH_HEADER = 'X-ND-Authorization'
export const TOKEN_KEY = 'pm_token'

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function setToken(token: string | null) {
  if (token) localStorage.setItem(TOKEN_KEY, token)
  else localStorage.removeItem(TOKEN_KEY)
}

export async function apiFetch<T>(path: string, options: RequestInit = {}): Promise<T> {
  const token = getToken()
  const headers = new Headers(options.headers)
  if (token) headers.set(AUTH_HEADER, `Bearer ${token}`)

  const res = await fetch(path, { ...options, headers })

  const refreshed = res.headers.get(AUTH_HEADER)
  if (refreshed) setToken(refreshed)

  if (res.status === 401) {
    setToken(null)
    window.dispatchEvent(new Event('pm:unauthorized'))
    throw new Error('Não autenticado')
  }

  if (!res.ok) {
    let msg = `HTTP ${res.status}`
    try {
      const body = await res.json()
      if (body?.error) msg = body.error
    } catch {
      /* not json */
    }
    throw new Error(msg)
  }

  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

export const api = {
  get: <T>(path: string) => apiFetch<T>(path),
  post: <T>(path: string, body?: unknown) =>
    apiFetch<T>(path, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body),
    }),
  put: <T>(path: string, body?: unknown) =>
    apiFetch<T>(path, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body),
    }),
  del: <T>(path: string) => apiFetch<T>(path, { method: 'DELETE' }),
}
