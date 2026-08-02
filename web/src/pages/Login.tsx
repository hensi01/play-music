import { useState } from 'react'
import { Music2 } from 'lucide-react'
import { appConfig } from '../config'
import { useAuth } from '../store/auth'

export default function Login() {
  const { login, createAdmin } = useAuth()
  const firstTime = appConfig.firstTime
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    if (firstTime && password !== confirm) {
      setError('As senhas não conferem.')
      return
    }
    setBusy(true)
    try {
      if (firstTime) await createAdmin(username, password)
      else await login(username, password)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Erro ao entrar')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex h-full items-center justify-center bg-gradient-to-b from-surface2/60 to-bg p-4">
      <div className="w-full max-w-sm">
        <div className="mb-8 flex flex-col items-center gap-3">
          <Music2 size={56} className="text-accent" />
          <h1 className="text-3xl font-bold tracking-tight">Play Music</h1>
          <p className="text-sm text-subtext">
            {firstTime ? 'Crie a conta de administrador' : 'Entre para ouvir sua música'}
          </p>
        </div>

        <form onSubmit={submit} className="flex flex-col gap-3 rounded-2xl bg-surface p-6 shadow-xl">
          <input
            type="text"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            placeholder="Usuário"
            autoFocus
            className="rounded-lg bg-surface2 px-4 py-3 text-sm outline-none ring-grid placeholder:text-faint focus:ring-2 focus:ring-accent"
          />
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder="Senha"
            className="rounded-lg bg-surface2 px-4 py-3 text-sm outline-none ring-grid placeholder:text-faint focus:ring-2 focus:ring-accent"
          />
          {firstTime && (
            <input
              type="password"
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
              placeholder="Confirmar senha"
              className="rounded-lg bg-surface2 px-4 py-3 text-sm outline-none ring-grid placeholder:text-faint focus:ring-2 focus:ring-accent"
            />
          )}
          {error && <p className="text-xs text-red-400">{error}</p>}
          <button
            type="submit"
            disabled={busy || !username || !password}
            className="mt-2 rounded-full bg-accent py-3 text-sm font-bold text-white transition-opacity hover:opacity-90 disabled:opacity-40"
          >
            {busy ? 'Aguarde…' : firstTime ? 'Criar conta' : 'Entrar'}
          </button>
        </form>
      </div>
    </div>
  )
}
