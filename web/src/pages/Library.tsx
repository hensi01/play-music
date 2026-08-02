import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { endpoints, artworkUrl } from '../api'
import type { Album, Artist, Playlist } from '../api/types'
import Card from '../components/Card'
import Spinner from '../components/Spinner'
import { albuns, musicas } from '../lib/format'

type Tab = 'albums' | 'artists' | 'playlists'

export default function Library() {
  const [tab, setTab] = useState<Tab>('albums')
  const [albums, setAlbums] = useState<Album[]>([])
  const [artists, setArtists] = useState<Artist[]>([])
  const [playlists, setPlaylists] = useState<Playlist[]>([])
  const [loading, setLoading] = useState(true)
  const navigate = useNavigate()

  useEffect(() => {
    Promise.allSettled([endpoints.albums(), endpoints.artists(), endpoints.playlists()])
      .then(([a, ar, p]) => {
        if (a.status === 'fulfilled') setAlbums(a.value)
        if (ar.status === 'fulfilled') setArtists(ar.value)
        if (p.status === 'fulfilled') setPlaylists(p.value)
      })
      .finally(() => setLoading(false))
  }, [])

  const tabs: { id: Tab; label: string }[] = [
    { id: 'albums', label: 'Álbuns' },
    { id: 'artists', label: 'Artistas' },
    { id: 'playlists', label: 'Playlists' },
  ]

  return (
    <div className="px-4 py-6 sm:px-6">
      <h1 className="mb-4 text-3xl font-bold">Sua Biblioteca</h1>
      <div className="mb-6 flex gap-2">
        {tabs.map((t) => (
          <button
            key={t.id}
            onClick={() => setTab(t.id)}
            className={`rounded-full px-4 py-1.5 text-sm font-medium transition-colors ${
              tab === t.id ? 'bg-white text-black' : 'bg-surface text-subtext hover:text-white'
            }`}
          >
            {t.label}
          </button>
        ))}
      </div>

      {loading ? (
        <Spinner />
      ) : (
        <div className="flex flex-wrap gap-3">
          {tab === 'albums' &&
            albums.map((a) => (
              <Card
                key={a.id}
                image={artworkUrl(a.id, 300)}
                title={a.name}
                subtitle={a.artist}
                onClick={() => navigate(`/album/${a.id}`)}
              />
            ))}
          {tab === 'artists' &&
            artists.map((ar) => (
              <Card
                key={ar.id}
                image={artworkUrl(ar.id, 300)}
                title={ar.name}
                subtitle={albuns(ar.albumCount)}
                onClick={() => navigate(`/artist/${ar.id}`)}
                square={false}
              />
            ))}
          {tab === 'playlists' &&
            playlists.map((p) => (
              <Card
                key={p.id}
                image={artworkUrl(p.id, 300)}
                title={p.name}
                subtitle={musicas(p.songCount)}
                onClick={() => navigate(`/playlist/${p.id}`)}
              />
            ))}

          {tab === 'albums' && albums.length === 0 && <p className="text-subtext">Nenhum álbum na biblioteca.</p>}
          {tab === 'artists' && artists.length === 0 && <p className="text-subtext">Nenhum artista na biblioteca.</p>}
          {tab === 'playlists' && playlists.length === 0 && <p className="text-subtext">Nenhuma playlist criada.</p>}
        </div>
      )}
    </div>
  )
}
