import { useEffect, useState } from 'react'
import { Clock } from 'lucide-react'
import { endpoints } from '../api'
import type { Song } from '../api/types'
import { usePlayer } from '../store/player'
import TrackRow from '../components/TrackRow'
import Spinner from '../components/Spinner'

export default function History() {
  const [songs, setSongs] = useState<Song[] | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    endpoints
      .history()
      .then(setSongs)
      .catch((err) => setError(err.message))
  }, [])

  if (error) return <div className="p-10 text-center text-subtext">{error}</div>
  if (!songs) return <Spinner />

  const player = usePlayer.getState()

  return (
    <div className="px-4 py-6 pb-24 sm:px-6">
      <h1 className="mb-4 text-3xl font-bold">Histórico</h1>
      <p className="mb-6 flex items-center gap-2 text-sm text-subtext">
        <Clock size={16} /> Músicas que você tocou recentemente
      </p>

      {songs.length === 0 ? (
        <p className="pt-8 text-center text-subtext">Nenhuma música tocada ainda.</p>
      ) : (
        <div className="rounded-xl bg-surface/30 p-2">
          {songs.map((s, i) => (
            <TrackRow key={s.id} song={s} index={i} onPlay={(_song, idx) => player.playContext(songs, idx)} />
          ))}
        </div>
      )}
    </div>
  )
}
