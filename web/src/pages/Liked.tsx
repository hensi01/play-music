import { useEffect, useState } from 'react'
import { Play } from 'lucide-react'
import { endpoints } from '../api'
import type { Song } from '../api/types'
import { usePlayer } from '../store/player'
import TrackRow from '../components/TrackRow'
import Spinner from '../components/Spinner'
import { musicas } from '../lib/format'

export default function Liked() {
  const [songs, setSongs] = useState<Song[] | null>(null)
  const [error, setError] = useState('')

  const load = () => {
    endpoints
      .liked()
      .then(setSongs)
      .catch((err) => setError(err.message))
  }

  useEffect(load, [])

  if (error) return <div className="p-10 text-center text-subtext">{error}</div>
  if (!songs) return <Spinner />

  const player = usePlayer.getState()

  return (
    <div className="pb-24">
      <div className="flex flex-col gap-5 bg-gradient-to-b from-accent/25 to-transparent p-6 sm:flex-row sm:items-end">
        <div className="flex h-44 w-44 shrink-0 items-center justify-center rounded-xl bg-gradient-to-br from-accent to-accent/40 text-6xl shadow-2xl">
          ♥
        </div>
        <div>
          <p className="text-xs font-bold uppercase tracking-widest text-subtext">Playlist</p>
          <h1 className="mt-1 text-4xl font-bold sm:text-5xl">Curtidas</h1>
          <p className="mt-2 text-sm text-subtext">{musicas(songs.length)}</p>
          <button
            onClick={() => songs.length > 0 && player.playContext(songs, 0)}
            disabled={songs.length === 0}
            className="mt-4 flex items-center gap-2 rounded-full bg-accent px-6 py-2.5 text-sm font-bold text-white transition-transform hover:scale-105 disabled:cursor-not-allowed disabled:opacity-40 disabled:hover:scale-100"
          >
            <Play size={18} fill="currentColor" /> Tocar
          </button>
        </div>
      </div>

      <div className="px-4 sm:px-6">
        {songs.length === 0 ? (
          <p className="pt-8 text-center text-subtext">
            Nenhuma música curtida ainda. Toque no coração de uma música para salvá-la aqui.
          </p>
        ) : (
          <div className="rounded-xl bg-surface/30 p-2">
            {songs.map((s, i) => (
              <TrackRow key={s.id} song={s} index={i} onPlay={(_song, idx) => player.playContext(songs, idx)} />
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
