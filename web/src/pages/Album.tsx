import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { Clock, Heart, Play, Shuffle } from 'lucide-react'
import { endpoints, artworkUrl } from '../api'
import type { AlbumDetail } from '../api/types'
import { usePlayer } from '../store/player'
import { formatDurationLong, musicas } from '../lib/format'
import TrackRow from '../components/TrackRow'
import Spinner from '../components/Spinner'

export default function Album() {
  const { id } = useParams()
  const navigate = useNavigate()
  const [album, setAlbum] = useState<AlbumDetail | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!id) return
    setAlbum(null)
    endpoints
      .album(id)
      .then(setAlbum)
      .catch((err) => setError(err.message))
  }, [id])

  if (error) return <div className="p-10 text-center text-subtext">{error}</div>
  if (!album) return <Spinner />

  const player = usePlayer.getState()
  const totalDuration = album.songs.reduce((acc, s) => acc + s.duration, 0)

  const toggleLike = () => {
    const next = !album.liked
    setAlbum({ ...album, liked: next })
    if (next) void endpoints.like(album.id)
    else void endpoints.unlike(album.id)
  }

  const playShuffle = () => {
    const songs = [...album.songs].sort(() => Math.random() - 0.5)
    player.playContext(songs, 0)
  }

  return (
    <div className="pb-24">
      {/* Header */}
      <div className="flex flex-col gap-6 bg-gradient-to-b from-grid/40 to-transparent p-6 sm:flex-row sm:items-end">
        <img
          src={artworkUrl(album.id, 640)}
          alt=""
          className="h-44 w-44 shrink-0 rounded-xl object-cover shadow-2xl sm:h-56 sm:w-56"
        />
        <div className="min-w-0">
          <p className="text-xs font-bold uppercase tracking-widest text-subtext">Álbum</p>
          <h1 className="mt-1 break-words text-4xl font-bold sm:text-5xl">{album.name}</h1>
          <div className="mt-3 flex items-center gap-1 text-sm text-subtext">
            <span
              className="cursor-pointer font-medium text-white hover:underline"
              onClick={() => album.artistId && navigate(`/artist/${album.artistId}`)}
            >
              {album.artist}
            </span>
            {album.year > 0 && (
              <>
                <span>•</span>
                <span>{album.year}</span>
              </>
            )}
            <span>•</span>
            <span>{musicas(album.songCount)}</span>
            <span>•</span>
            <span>{formatDurationLong(totalDuration)}</span>
          </div>
          <div className="mt-4 flex items-center gap-4">
            <button
              onClick={() => player.playContext(album.songs, 0)}
              className="flex items-center gap-2 rounded-full bg-accent px-6 py-2.5 text-sm font-bold text-white transition-transform hover:scale-105"
            >
              <Play size={18} fill="currentColor" /> Tocar
            </button>
            <button
              onClick={playShuffle}
              className="rounded-full p-2.5 text-subtext hover:bg-hover hover:text-white"
              aria-label="Aleatório"
            >
              <Shuffle size={22} />
            </button>
            <button
              onClick={toggleLike}
              className={`rounded-full p-2.5 ${album.liked ? 'text-accent' : 'text-subtext hover:text-white'}`}
              aria-label="Curtir álbum"
            >
              <Heart size={22} fill={album.liked ? 'currentColor' : 'none'} />
            </button>
          </div>
        </div>
      </div>

      {/* Track list */}
      <div className="px-4 sm:px-6">
        <div className="mb-2 hidden grid-cols-[2.5rem_1fr_1fr_auto] gap-3 border-b border-grid/50 px-3 pb-2 text-xs font-medium uppercase tracking-wider text-faint sm:grid">
          <span>#</span>
          <span>Título</span>
          <span>Álbum</span>
          <span className="flex items-center gap-2">
            <Clock size={14} /> Duração
          </span>
        </div>
        <div className="rounded-xl bg-surface/30 p-2">
          {album.songs.map((s, i) => (
            <TrackRow key={s.id} song={s} index={i} onPlay={(_song, idx) => player.playContext(album.songs, idx)} showAlbum={false} />
          ))}
        </div>
      </div>
    </div>
  )
}
