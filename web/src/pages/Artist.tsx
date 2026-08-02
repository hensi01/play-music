import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { Heart, Play } from 'lucide-react'
import { endpoints, artworkUrl } from '../api'
import type { ArtistDetail } from '../api/types'
import { usePlayer } from '../store/player'
import TrackRow from '../components/TrackRow'
import Card from '../components/Card'
import Spinner from '../components/Spinner'

export default function Artist() {
  const { id } = useParams()
  const navigate = useNavigate()
  const [artist, setArtist] = useState<ArtistDetail | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!id) return
    setArtist(null)
    endpoints
      .artist(id)
      .then(setArtist)
      .catch((err) => setError(err.message))
  }, [id])

  if (error) return <div className="p-10 text-center text-subtext">{error}</div>
  if (!artist) return <Spinner />

  const player = usePlayer.getState()

  const toggleLike = () => {
    const next = !artist.liked
    setArtist({ ...artist, liked: next })
    if (next) void endpoints.like(artist.id)
    else void endpoints.unlike(artist.id)
  }

  return (
    <div className="pb-24">
      <div className="flex flex-col items-center gap-5 bg-gradient-to-b from-grid/40 to-transparent p-8 sm:flex-row sm:items-end sm:p-6">
        <img
          src={artworkUrl(artist.id, 480)}
          alt=""
          className="h-40 w-40 shrink-0 rounded-full object-cover shadow-2xl sm:h-52 sm:w-52"
        />
        <div className="min-w-0 text-center sm:text-left">
          <p className="text-xs font-bold uppercase tracking-widest text-subtext">Artista</p>
          <h1 className="mt-1 text-4xl font-bold sm:text-5xl">{artist.name}</h1>
          <p className="mt-2 text-sm text-subtext">
            {artist.albumCount} álbuns • {artist.songCount} músicas
          </p>
          <div className="mt-4 flex items-center justify-center gap-4 sm:justify-start">
            <button
              onClick={() => artist.topSongs.length > 0 && player.playContext(artist.topSongs, 0)}
              className="flex items-center gap-2 rounded-full bg-accent px-6 py-2.5 text-sm font-bold text-white transition-transform hover:scale-105"
            >
              <Play size={18} fill="currentColor" /> Tocar
            </button>
            <button
              onClick={toggleLike}
              className={`rounded-full p-2.5 ${artist.liked ? 'text-accent' : 'text-subtext hover:text-white'}`}
              aria-label="Seguir"
            >
              <Heart size={22} fill={artist.liked ? 'currentColor' : 'none'} />
            </button>
          </div>
        </div>
      </div>

      {artist.topSongs.length > 0 && (
        <div className="mt-2 px-4 sm:px-6">
          <h2 className="mb-2 text-xl font-bold">Mais tocadas</h2>
          <div className="rounded-xl bg-surface/30 p-2">
            {artist.topSongs.map((s, i) => (
              <TrackRow
                key={s.id}
                song={s}
                index={i}
                onPlay={(_song, idx) => player.playContext(artist.topSongs, idx)}
                showAlbum
              />
            ))}
          </div>
        </div>
      )}

      {artist.albums.length > 0 && (
        <div className="mt-6 px-4 sm:px-6">
          <h2 className="mb-3 text-xl font-bold">Álbuns</h2>
          <div className="flex flex-wrap gap-3">
            {artist.albums.map((a) => (
              <Card
                key={a.id}
                image={artworkUrl(a.id, 300)}
                title={a.name}
                subtitle={a.year ? String(a.year) : artist.name}
                onClick={() => navigate(`/album/${a.id}`)}
              />
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
