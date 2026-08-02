import { create } from 'zustand'
import { apiFetch, getToken, setToken } from '../api/client'
import type { UserProfile } from '../api/types'

interface AuthState {
  user: UserProfile | null
  loading: boolean
  login: (username: string, password: string) => Promise<void>
  createAdmin: (username: string, password: string) => Promise<void>
  logout: () => void
  refresh: () => Promise<boolean>
}

export const useAuth = create<AuthState>((set) => ({
  user: null,
  loading: true,

  login: async (username, password) => {
    const res = await apiFetch<{ token: string; username: string; name: string; isAdmin: boolean; id: string }>(
      '/auth/login',
      {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password }),
      },
    )
    setToken(res.token)
    set({
      user: { id: res.id, username: res.username, name: res.name, isAdmin: res.isAdmin },
    })
  },

  createAdmin: async (username, password) => {
    const res = await apiFetch<{ token: string; username: string; name: string; isAdmin: boolean; id: string }>(
      '/auth/createAdmin',
      {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password }),
      },
    )
    setToken(res.token)
    set({
      user: { id: res.id, username: res.username, name: res.name, isAdmin: res.isAdmin },
    })
  },

  logout: () => {
    setToken(null)
    set({ user: null })
  },

  refresh: async () => {
    if (!getToken()) {
      set({ loading: false, user: null })
      return false
    }
    try {
      const user = await apiFetch<UserProfile>('/api/me')
      set({ user, loading: false })
      return true
    } catch {
      set({ user: null, loading: false })
      return false
    }
  },
}))
