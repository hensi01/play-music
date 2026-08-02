import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { endpoints, artworkUrl } from '../api'
import type { Album, Home } from '../api/types'
import { usePlayer } from '../store/player'
import { greeting } from '../lib/format'
import Section from '../components/Section'
import Card from '../components/Card'
import Spinner from '../components/Spinner'

export default function Home() {
  const [home, setHome] = useState<Home | null>(null)
  const [error, setError] = useState('')
  const navigate = useNavigate()

  useEffect(() => {
    endpoints
      .home()
      .then(setHome)
      .catch((err) => setError(err.message))
  }, [])

  if (error) return <EmptyState message={error} />
  if (!home) return <Spinner />

  const playAlbum = async (album: Album) => {
    try {
      const detail = await endpoints.album(album.id)
      usePlayer.getState().playContext(detail.songs, 0)
    } catch {
      /* ignore */
    }
  }

  const albumCards = (albums?: Album[]) =>
    (albums ?? []).map((a) => (
      <Card
        key={a.id}
        image={artworkUrl(a.id, 300)}
        title={a.name}
        subtitle={a.artist}
        onClick={() => navigate(`/album/${a.id}`)}
        onPlay={() => void playAlbum(a)}
      />
    ))

  return (
    <div className="pb-24">
      <div className="px-4 pb-4 pt-6 sm:px-6">
        <h1 className="text-3xl font-bold">{greeting()}</h1>
      </div>

      {home.sections.map((s) => (
        <Section key={s.id} title={s.title}>
          {albumCards(s.albums)}
        </Section>
      ))}

      {home.genres.length > 0 && (
        <Section title="Gêneros">
          {home.genres.map((g) => (
            <button
              key={g.name}
              onClick={() => navigate(`/search?genre=${encodeURIComponent(g.name)}`)}
              className="flex h-24 w-36 shrink-0 flex-col justify-end rounded-lg bg-gradient-to-br from-accent/40 to-grid p-3 text-left text-sm font-bold transition-transform hover:scale-105"
            >
              {g.name}
              <span className="text-xs font-normal text-white/70">{g.songCount} músicas</span>
            </button>
          ))}
        </Section>
      )}

      {home.sections.length === 0 && home.genres.length === 0 && (
        <div className="px-6 pt-10 text-center text-subtext">
          <p className="text-lg">Sua biblioteca está vazia.</p>
          <p className="mt-1 text-sm">Adicione músicas à pasta configurada e a varredura as importará automaticamente.</p>
        </div>
      )}
    </div>
  )
}

function EmptyState({ message }: { message: string }) {
  return <div className="p-10 text-center text-subtext">{message}</div>
}
