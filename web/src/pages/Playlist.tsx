import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { Play, Trash2 } from 'lucide-react'
import { endpoints, artworkUrl } from '../api'
import type { PlaylistDetail, Song } from '../api/types'
import { usePlayer } from '../store/player'
import { formatDurationLong } from '../lib/format'
import TrackRow from '../components/TrackRow'
import Spinner from '../components/Spinner'

export default function Playlist() {
  const { id } = useParams()
  const navigate = useNavigate()
  const [playlist, setPlaylist] = useState<PlaylistDetail | null>(null)
  const [error, setError] = useState('')

  const load = () => {
    if (!id) return
    endpoints
      .playlist(id)
      .then(setPlaylist)
      .catch((err) => setError(err.message))
  }

  useEffect(load, [id])

  if (error) return <div className="p-10 text-center text-subtext">{error}</div>
  if (!playlist) return <Spinner />

  const player = usePlayer.getState()
  const songs: Song[] = playlist.songs.map((ps) => ps.song)

  const playAll = () => player.playContext(songs, 0)

  const remove = (entryId: string) => {
    void endpoints.removePlaylistTrack(playlist.id, entryId).then(load)
  }

  return (
    <div className="pb-24">
      <div className="flex flex-col gap-6 bg-gradient-to-b from-grid/40 to-transparent p-6 sm:flex-row sm:items-end">
        <img src={artworkUrl(playlist.id, 320)} alt="" className="h-44 w-44 shrink-0 rounded-xl object-cover shadow-2xl" />
        <div className="min-w-0">
          <p className="text-xs font-bold uppercase tracking-widest text-subtext">Playlist</p>
          <h1 className="mt-1 break-words text-4xl font-bold">{playlist.name}</h1>
          {playlist.comment && <p className="mt-2 text-sm text-subtext">{playlist.comment}</p>}
          <p className="mt-2 text-sm text-subtext">
            {playlist.owner} • {playlist.songCount} músicas • {formatDurationLong(playlist.duration)}
          </p>
          <button
            onClick={playAll}
            className="mt-4 flex items-center gap-2 rounded-full bg-accent px-6 py-2.5 text-sm font-bold text-white transition-transform hover:scale-105"
          >
            <Play size={18} fill="currentColor" /> Tocar
          </button>
        </div>
      </div>

      <div className="px-4 sm:px-6">
        {playlist.songs.length === 0 ? (
          <p className="pt-8 text-center text-subtext">
            Esta playlist está vazia. Toque numa música e a adicione a uma playlist pela página do álbum.
          </p>
        ) : (
          <div className="rounded-xl bg-surface/30 p-2">
            {playlist.songs.map((ps, i) => (
              <div key={ps.entryId} className="group flex items-center gap-2">
                <div className="flex-1">
                  <TrackRow song={ps.song} index={i} onPlay={(_song, idx) => player.playContext(songs, idx)} />
                </div>
                <button
                  onClick={() => remove(ps.entryId)}
                  className="p-2 text-subtext opacity-0 hover:text-red-400 group-hover:opacity-100"
                  aria-label="Remover da playlist"
                >
                  <Trash2 size={16} />
                </button>
              </div>
            ))}
          </div>
        )}
        <button
          onClick={() => navigate('/library')}
          className="mt-4 text-sm text-subtext hover:text-white"
        >
          ← Voltar para a biblioteca
        </button>
      </div>
    </div>
  )
}
