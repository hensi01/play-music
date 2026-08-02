import { useEffect, useRef, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { Search as SearchIcon } from 'lucide-react'
import { endpoints, artworkUrl } from '../api'
import type { SearchResults, Song } from '../api/types'
import { usePlayer } from '../store/player'
import TrackRow from '../components/TrackRow'
import Card from '../components/Card'
import Section from '../components/Section'
import Spinner from '../components/Spinner'

export default function Search() {
  const [params] = useSearchParams()
  const [query, setQuery] = useState(params.get('genre') ?? '')
  const [results, setResults] = useState<SearchResults | null>(null)
  const [busy, setBusy] = useState(false)
  const navigate = useNavigate()
  const debounce = useRef<number | undefined>(undefined)

  const run = (q: string) => {
    if (!q.trim()) {
      setResults(null)
      return
    }
    setBusy(true)
    endpoints
      .search(q)
      .then(setResults)
      .catch(() => setResults(null))
      .finally(() => setBusy(false))
  }

  useEffect(() => {
    if (params.get('genre')) {
      const g = params.get('genre')!
      setQuery(g)
      run(g)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const onChange = (q: string) => {
    setQuery(q)
    window.clearTimeout(debounce.current)
    debounce.current = window.setTimeout(() => run(q), 300)
  }

  const playSong = (_song: Song, index: number) => {
    if (results) usePlayer.getState().playContext(results.songs, index)
  }

  return (
    <div className="px-4 py-6 sm:px-6">
      <div className="relative mb-6 max-w-xl">
        <SearchIcon size={18} className="absolute left-4 top-1/2 -translate-y-1/2 text-faint" />
        <input
          value={query}
          onChange={(e) => onChange(e.target.value)}
          placeholder="O que você quer ouvir?"
          className="w-full rounded-full bg-surface py-3 pl-11 pr-4 text-sm outline-none ring-grid placeholder:text-faint focus:ring-2 focus:ring-accent"
        />
      </div>

      {busy && !results && <Spinner />}

      {results && (
        <div className="space-y-6">
          {results.songs.length > 0 && (
            <div>
              <h2 className="mb-2 text-xl font-bold">Músicas</h2>
              <div className="rounded-xl bg-surface/50 p-2">
                {results.songs.map((s, i) => (
                  <TrackRow key={s.id} song={s} index={i} onPlay={(_song, idx) => playSong(s, idx)} />
                ))}
              </div>
            </div>
          )}

          {results.albums.length > 0 && (
            <Section title="Álbuns">
              {results.albums.map((a) => (
                <Card
                  key={a.id}
                  image={artworkUrl(a.id, 300)}
                  title={a.name}
                  subtitle={a.artist}
                  onClick={() => navigate(`/album/${a.id}`)}
                />
              ))}
            </Section>
          )}

          {results.artists.length > 0 && (
            <Section title="Artistas">
              {results.artists.map((ar) => (
                <Card
                  key={ar.id}
                  image={artworkUrl(ar.id, 300)}
                  title={ar.name}
                  subtitle={`${ar.albumCount} álbuns`}
                  onClick={() => navigate(`/artist/${ar.id}`)}
                  square={false}
                />
              ))}
            </Section>
          )}

          {results.playlists.length > 0 && (
            <Section title="Playlists">
              {results.playlists.map((p) => (
                <Card
                  key={p.id}
                  image={artworkUrl(p.id, 300)}
                  title={p.name}
                  subtitle={p.owner}
                  onClick={() => navigate(`/playlist/${p.id}`)}
                />
              ))}
            </Section>
          )}

          {results.songs.length === 0 &&
            results.albums.length === 0 &&
            results.artists.length === 0 &&
            results.playlists.length === 0 && (
              <p className="pt-8 text-center text-subtext">Nada encontrado para “{query}”.</p>
            )}
        </div>
      )}
    </div>
  )
}
