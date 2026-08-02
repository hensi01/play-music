import { useEffect, useState } from 'react'
import { Route, Routes } from 'react-router-dom'
import { Menu, Music2 } from 'lucide-react'
import { useAuth } from './store/auth'
import { usePlayer } from './store/player'
import Sidebar from './components/Sidebar'
import BottomBar from './components/BottomBar'
import PlayerFull from './components/PlayerFull'
import Login from './pages/Login'
import Home from './pages/Home'
import Search from './pages/Search'
import Library from './pages/Library'
import Album from './pages/Album'
import Artist from './pages/Artist'
import Playlist from './pages/Playlist'
import Liked from './pages/Liked'
import History from './pages/History'
import Queue from './pages/Queue'
import Lyrics from './pages/Lyrics'
import Settings from './pages/Settings'

export default function App() {
  const { user, loading, refresh, logout } = useAuth()
  const fullScreen = usePlayer((s) => s.fullScreen)
  const [menuOpen, setMenuOpen] = useState(false)

  useEffect(() => {
    void refresh()
  }, [refresh])

  useEffect(() => {
    const onUnauthorized = () => logout()
    window.addEventListener('pm:unauthorized', onUnauthorized)
    return () => window.removeEventListener('pm:unauthorized', onUnauthorized)
  }, [logout])

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center text-subtext">
        <Music2 size={28} className="mr-2 text-accent" /> Carregando…
      </div>
    )
  }

  if (!user) {
    return <Login />
  }

  return (
    <div className="flex h-full flex-col">
      <div className="flex flex-1 overflow-hidden">
        <Sidebar mobileOpen={menuOpen} onClose={() => setMenuOpen(false)} />
        <main className="relative flex-1 overflow-y-auto bg-gradient-to-b from-surface2/60 to-bg">
          {/* Mobile top bar */}
          <div className="sticky top-0 z-20 flex items-center gap-3 bg-bg/95 p-3 backdrop-blur md:hidden">
            <button
              onClick={() => setMenuOpen(true)}
              className="rounded-full p-2 text-subtext hover:bg-hover hover:text-white"
              aria-label="Abrir menu"
            >
              <Menu size={22} />
            </button>
            <span className="flex items-center gap-2 text-sm font-semibold">
              <Music2 size={20} className="text-accent" /> Play Music
            </span>
          </div>
          <Routes>
            <Route path="/" element={<Home />} />
            <Route path="/search" element={<Search />} />
            <Route path="/library" element={<Library />} />
            <Route path="/album/:id" element={<Album />} />
            <Route path="/artist/:id" element={<Artist />} />
            <Route path="/playlist/:id" element={<Playlist />} />
            <Route path="/liked" element={<Liked />} />
            <Route path="/history" element={<History />} />
            <Route path="/queue" element={<Queue />} />
            <Route path="/lyrics" element={<Lyrics />} />
            <Route path="/settings" element={<Settings />} />
            <Route path="*" element={<Home />} />
          </Routes>
        </main>
      </div>
      <BottomBar />
      {fullScreen && <PlayerFull />}
    </div>
  )
}
