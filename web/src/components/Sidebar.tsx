import { useEffect, useState } from 'react'
import { NavLink, useNavigate } from 'react-router-dom'
import { Clock, Heart, Home, Library, ListMusic, LogOut, Music2, Search, Settings, X } from 'lucide-react'
import { endpoints } from '../api'
import type { Playlist } from '../api/types'
import { useAuth } from '../store/auth'

interface SidebarProps {
  mobileOpen: boolean
  onClose: () => void
}

function navClass({ isActive }: { isActive: boolean }) {
  return isActive
    ? 'flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium text-white bg-hover'
    : 'flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium text-subtext hover:text-white transition-colors'
}

export default function Sidebar({ mobileOpen, onClose }: SidebarProps) {
  const [playlists, setPlaylists] = useState<Playlist[]>([])
  const { user, logout } = useAuth()
  const navigate = useNavigate()

  useEffect(() => {
    const loadPlaylists = () => endpoints.playlists().then(setPlaylists).catch(() => undefined)
    void loadPlaylists()
    window.addEventListener('pm:playlists-changed', loadPlaylists)
    return () => window.removeEventListener('pm:playlists-changed', loadPlaylists)
  }, [])

  const content = (
    <div className="flex h-full flex-col">
      <div className="flex items-center justify-between p-4">
        <span className="flex items-center gap-2 text-lg font-bold tracking-tight">
          <Music2 size={26} className="text-accent" /> Play Music
        </span>
        <button onClick={onClose} className="rounded-full p-1 text-subtext hover:text-white md:hidden" aria-label="Fechar">
          <X size={20} />
        </button>
      </div>

      <nav className="flex flex-col gap-1 px-3">
        <NavLink to="/" end className={navClass} onClick={onClose}>
          <Home size={20} /> Início
        </NavLink>
        <NavLink to="/search" className={navClass} onClick={onClose}>
          <Search size={20} /> Buscar
        </NavLink>
        <NavLink to="/library" className={navClass} onClick={onClose}>
          <Library size={20} /> Sua Biblioteca
        </NavLink>
        <NavLink to="/liked" className={navClass} onClick={onClose}>
          <Heart size={20} /> Curtidas
        </NavLink>
        <NavLink to="/history" className={navClass} onClick={onClose}>
          <Clock size={20} /> Histórico
        </NavLink>
      </nav>

      <div className="mt-5 flex items-center justify-between px-5">
        <span className="text-xs font-bold uppercase tracking-widest text-faint">Playlists</span>
        <ListMusic size={16} className="text-faint" />
      </div>
      <div className="mt-1 flex-1 overflow-y-auto px-3 pb-3">
        {playlists.map((pl) => (
          <NavLink
            key={pl.id}
            to={`/playlist/${pl.id}`}
            className={({ isActive }) =>
              `block truncate rounded-lg px-3 py-1.5 text-sm ${
                isActive ? 'text-white bg-hover' : 'text-subtext hover:text-white'
              }`
            }
            onClick={onClose}
          >
            {pl.name}
          </NavLink>
        ))}
        {playlists.length === 0 && <p className="px-3 py-1.5 text-xs text-faint">Nenhuma playlist ainda</p>}
      </div>

      <div className="border-t border-grid/50 p-3">
        <div className="flex items-center justify-between px-2 py-1">
          <div className="min-w-0">
            <p className="truncate text-sm font-medium">{user?.name ?? user?.username}</p>
            <p className="truncate text-xs text-faint">@{user?.username}</p>
          </div>
          <div className="flex items-center gap-1">
            <button
              onClick={() => {
                onClose()
                navigate('/settings')
              }}
              className="rounded-full p-2 text-subtext hover:bg-hover hover:text-white"
              aria-label="Configurações"
            >
              <Settings size={18} />
            </button>
            <button
              onClick={() => {
                logout()
                navigate('/login')
              }}
              className="rounded-full p-2 text-subtext hover:bg-hover hover:text-white"
              aria-label="Sair"
            >
              <LogOut size={18} />
            </button>
          </div>
        </div>
      </div>
    </div>
  )

  return (
    <>
      {/* Desktop */}
      <aside className="hidden w-60 shrink-0 border-r border-grid/50 bg-surface md:block">{content}</aside>
      {/* Mobile overlay */}
      {mobileOpen && (
        <div className="fixed inset-0 z-50 md:hidden">
          <div className="absolute inset-0 bg-black/60" onClick={onClose} />
          <aside className="absolute inset-y-0 left-0 w-72 bg-surface shadow-2xl">{content}</aside>
        </div>
      )}
    </>
  )
}
