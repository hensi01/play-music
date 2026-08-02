import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { ArrowLeft } from 'lucide-react'
import { endpoints, artworkUrl } from '../api'
import type { LyricsResponse } from '../api/types'
import { usePlayer } from '../store/player'

export default function Lyrics() {
  const { current, progress } = usePlayer()
  const navigate = useNavigate()
  const [lyrics, setLyrics] = useState<LyricsResponse | null>(null)
  const [loaded, setLoaded] = useState(false)

  useEffect(() => {
    setLoaded(false)
    if (!current) {
      setLyrics(null)
      return
    }
    endpoints
      .lyrics(current.id)
      .then((l) => {
        setLyrics(l)
        setLoaded(true)
      })
      .catch(() => setLoaded(true))
  }, [current?.id])

  if (!current) {
    return (
      <div className="p-10 text-center text-subtext">
        Nenhuma música tocando. Inicie uma reprodução para ver as letras.
      </div>
    )
  }

  if (!loaded) {
    return <div className="p-10 text-center text-subtext">Carregando letras…</div>
  }

  if (!lyrics || lyrics.lines.length === 0) {
    return (
      <div className="p-10 text-center text-subtext">
        Nenhuma letra encontrada para esta música.
      </div>
    )
  }

  const activeIndex = lyrics.synced
    ? lyrics.lines.findIndex((l, i) => {
        const next = lyrics.lines[i + 1]
        if (l.start == null) return false
        if (next && next.start != null) return progress * 1000 >= l.start && progress * 1000 < next.start
        return progress * 1000 >= l.start
      })
    : -1

  return (
    <div className="mx-auto max-w-2xl px-6 py-6 pb-24">
      <button
        onClick={() => navigate(-1)}
        className="mb-6 flex items-center gap-2 text-sm text-subtext hover:text-white"
      >
        <ArrowLeft size={16} /> Voltar
      </button>
      <div className="mb-8 flex items-center gap-4">
        <img src={artworkUrl(current.albumId, 96)} alt="" className="h-16 w-16 rounded-lg object-cover" />
        <div>
          <h1 className="text-xl font-bold">{current.title}</h1>
          <p className="text-sm text-subtext">{current.artist}</p>
        </div>
      </div>

      <div className="space-y-4">
        {lyrics.lines.map((line, i) => (
          <p
            key={i}
            className={`transition-colors ${
              lyrics.synced && i === activeIndex
                ? 'text-2xl font-bold text-white'
                : 'text-lg text-subtext'
            }`}
          >
            {line.text || '♪'}
          </p>
        ))}
      </div>
    </div>
  )
}
