import { useEffect, useState } from 'react'
import { LogOut, Music2, Plus } from 'lucide-react'
import { endpoints } from '../api'
import type { Settings as SettingsInfo } from '../api/types'
import { useAuth } from '../store/auth'
import Spinner from '../components/Spinner'

export default function Settings() {
  const { user, logout } = useAuth()
  const [settings, setSettings] = useState<SettingsInfo | null>(null)
  const [message, setMessage] = useState('')

  useEffect(() => {
    endpoints.settings().then(setSettings).catch(() => undefined)
  }, [])

  const createPlaylist = () => {
    const name = window.prompt('Nome da nova playlist:')
    if (!name?.trim()) return
    endpoints
      .createPlaylist(name.trim(), [])
      .then(() => setMessage(`Playlist “${name}” criada!`))
      .catch((err) => setMessage(err.message))
  }

  return (
    <div className="mx-auto max-w-2xl px-4 py-6 pb-24 sm:px-6">
      <h1 className="mb-6 text-3xl font-bold">Configurações</h1>

      <section className="mb-6 rounded-2xl bg-surface p-6">
        <h2 className="mb-4 text-lg font-bold">Conta</h2>
        <p className="text-sm text-subtext">
          Conectado como <span className="font-semibold text-white">{user?.name}</span> (@{user?.username})
        </p>
        <div className="mt-4 flex flex-wrap gap-3">
          <button
            onClick={() => void createPlaylist()}
            className="flex items-center gap-2 rounded-full bg-accent px-5 py-2 text-sm font-bold text-white"
          >
            <Plus size={16} /> Nova playlist
          </button>
          <button
            onClick={logout}
            className="flex items-center gap-2 rounded-full bg-surface2 px-5 py-2 text-sm font-semibold text-subtext hover:text-white"
          >
            <LogOut size={16} /> Sair
          </button>
        </div>
        {message && <p className="mt-3 text-sm text-accent">{message}</p>}
      </section>

      <section className="rounded-2xl bg-surface p-6">
        <h2 className="mb-4 text-lg font-bold">Servidor</h2>
        {!settings ? (
          <Spinner />
        ) : (
          <dl className="space-y-3 text-sm">
            <div className="flex justify-between gap-4">
              <dt className="text-subtext">Servidor</dt>
              <dd className="flex items-center gap-1.5 font-medium">
                <Music2 size={14} className="text-accent" /> {settings.appName}
              </dd>
            </div>
            <div className="flex justify-between gap-4">
              <dt className="text-subtext">Versão</dt>
              <dd className="font-medium">{settings.version}</dd>
            </div>
            <div className="flex justify-between gap-4">
              <dt className="text-subtext">Biblioteca</dt>
              <dd className="truncate font-medium">{settings.libraryName}</dd>
            </div>
            <div className="flex justify-between gap-4">
              <dt className="text-subtext">Pasta de música</dt>
              <dd className="truncate font-mono text-xs">{settings.musicFolder}</dd>
            </div>
          </dl>
        )}
      </section>
    </div>
  )
}
