import { useState } from 'react'
import { Heart, Play } from 'lucide-react'
import { artworkUrl, endpoints } from '../api'
import type { Song } from '../api/types'
import { formatDuration } from '../lib/format'

interface TrackRowProps {
  song: Song
  index?: number
  onPlay?: (song: Song, index: number) => void
  showAlbum?: boolean
  showArtist?: boolean
}

export default function TrackRow({ song, index, onPlay, showAlbum = true, showArtist = true }: TrackRowProps) {
  const [hover, setHover] = useState(false)
  const [liked, setLiked] = useState(song.liked)

  const toggleLike = () => {
    const next = !liked
    setLiked(next)
    const request = next ? endpoints.like(song.id) : endpoints.unlike(song.id)
    void request.catch(() => setLiked(!next))
  }

  return (
    <div
      className="group grid grid-cols-[2.5rem_1fr_auto] items-center gap-3 rounded-lg px-3 py-2 hover:bg-hover sm:grid-cols-[2.5rem_1fr_1fr_auto_auto]"
      onMouseEnter={() => setHover(true)}
      onMouseLeave={() => setHover(false)}
    >
      <div className="flex items-center justify-center text-sm tabular-nums text-subtext">
        {onPlay ? (
          <>
          <button
            onClick={() => onPlay(song, index ?? 0)}
            className={`text-white ${hover ? '' : 'sm:hidden'}`}
            aria-label="Tocar"
          >
            <Play size={16} fill="currentColor" />
          </button>
          {!hover && <span className="hidden sm:inline">{(index ?? 0) + 1}</span>}
          </>
        ) : (
          <span>{(index ?? 0) + 1}</span>
        )}
      </div>

      <div className="flex min-w-0 items-center gap-3">
        <img src={artworkUrl(song.albumId, 48)} alt="" className="hidden h-10 w-10 shrink-0 rounded object-cover sm:block" />
        <div className="min-w-0">
          <p className={`truncate text-sm font-medium ${hover ? 'text-white' : ''}`}>{song.title}</p>
          {showArtist && (
            <p className="truncate text-xs text-subtext">{song.artist}</p>
          )}
        </div>
      </div>

      {showAlbum && (
        <p className="hidden truncate pr-2 text-sm text-subtext sm:block">{song.album}</p>
      )}

      <div className="flex items-center justify-end gap-3">
        <button
          onClick={toggleLike}
          className={`p-1 ${liked ? 'text-accent' : 'text-subtext hover:text-white sm:opacity-0 sm:group-hover:opacity-100'}`}
          aria-label={liked ? 'Descurtir' : 'Curtir'}
          aria-pressed={liked}
        >
          <Heart size={16} fill={liked ? 'currentColor' : 'none'} />
        </button>
        <span className="w-10 text-right text-xs tabular-nums text-subtext">{formatDuration(song.duration)}</span>
      </div>
    </div>
  )
}
