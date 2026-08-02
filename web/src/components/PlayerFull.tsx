import { useNavigate } from 'react-router-dom'
import {
  ChevronDown,
  Heart,
  ListMusic,
  Pause,
  Play,
  Repeat,
  Shuffle,
  SkipBack,
  SkipForward,
} from 'lucide-react'
import { artworkUrl, endpoints } from '../api'
import { resolveDuration, usePlayer } from '../store/player'
import { formatDuration } from '../lib/format'

export default function PlayerFull() {
  const player = usePlayer()
  const navigate = useNavigate()
  const { current } = player
  if (!current) return null

  const totalDuration = resolveDuration(player.duration, current.duration)
  const safeProgress = Number.isFinite(player.progress)
    ? Math.min(Math.max(player.progress, 0), totalDuration)
    : 0
  const progressPercent = totalDuration > 0 ? (safeProgress / totalDuration) * 100 : 0

  const toggleLike = () => {
    const next = !current.liked
    player.setLiked(next)
    if (next) void endpoints.like(current.id)
    else void endpoints.unlike(current.id)
  }

  const seekTo = (e: React.MouseEvent<HTMLDivElement>) => {
    const rect = e.currentTarget.getBoundingClientRect()
    if (rect.width === 0 || totalDuration === 0) return
    const ratio = (e.clientX - rect.left) / rect.width
    player.seek(ratio * totalDuration)
  }

  return (
    <div className="fixed inset-0 z-50 flex flex-col bg-gradient-to-b from-surface2 to-bg p-6">
      <div className="flex items-center justify-between">
        <button
          onClick={() => player.setFullScreen(false)}
          className="rounded-full p-2 text-subtext hover:bg-hover hover:text-white"
          aria-label="Fechar"
        >
          <ChevronDown size={26} />
        </button>
        <button
          onClick={() => navigate('/queue')}
          className="rounded-full p-2 text-subtext hover:bg-hover hover:text-white"
          aria-label="Fila"
        >
          <ListMusic size={22} />
        </button>
      </div>

      <div className="flex flex-1 flex-col items-center justify-center gap-8">
        <img
          src={artworkUrl(current.albumId, 640)}
          alt="Capa do álbum"
          className="aspect-square w-full max-w-sm rounded-2xl object-cover shadow-2xl"
        />
        <div className="flex w-full max-w-sm items-end justify-between">
          <div className="min-w-0">
            <h2 className="truncate text-2xl font-bold">{current.title}</h2>
            <p
              className="mt-1 cursor-pointer truncate text-base text-subtext hover:text-white hover:underline"
              onClick={() => navigate(`/artist/${current.artistId}`)}
            >
              {current.artist}
            </p>
          </div>
          <button
            onClick={toggleLike}
            className={`shrink-0 p-2 ${current.liked ? 'text-accent' : 'text-subtext hover:text-white'}`}
            aria-label={current.liked ? 'Descurtir' : 'Curtir'}
            aria-pressed={current.liked}
          >
            <Heart size={26} fill={current.liked ? 'currentColor' : 'none'} />
          </button>
        </div>

        <div className="w-full max-w-sm">
          <div
            className="group relative h-1 cursor-pointer rounded-full bg-grid"
            onClick={seekTo}
            role="slider"
            aria-valuemin={0}
            aria-valuemax={totalDuration}
            aria-valuenow={safeProgress}
          >
            <div
              className="absolute inset-y-0 left-0 rounded-full bg-white group-hover:bg-accent"
              style={{ width: `${progressPercent}%` }}
            />
          </div>
          <div className="mt-1.5 flex justify-between text-xs tabular-nums text-subtext">
            <span>{formatDuration(safeProgress)}</span>
            <span>{formatDuration(totalDuration)}</span>
          </div>
        </div>

        <div className="flex items-center gap-6">
          <button
            onClick={player.toggleShuffle}
            className={`p-1 ${player.shuffle ? 'text-accent' : 'text-subtext'}`}
            aria-label="Aleatório"
          >
            <Shuffle size={20} />
          </button>
          <button onClick={player.prev} className="p-2 text-subtext hover:text-white" aria-label="Anterior">
            <SkipBack size={30} fill="currentColor" />
          </button>
          <button
            onClick={player.togglePlay}
            className="rounded-full bg-white p-4 text-black hover:scale-105"
            aria-label={player.playing ? 'Pausar' : 'Tocar'}
          >
            {player.playing ? (
              <Pause size={32} fill="currentColor" />
            ) : (
              <Play size={32} fill="currentColor" className="ml-0.5" />
            )}
          </button>
          <button onClick={player.next} className="p-2 text-subtext hover:text-white" aria-label="Próxima">
            <SkipForward size={30} fill="currentColor" />
          </button>
          <button
            onClick={player.toggleRepeat}
            className={`p-1 ${player.repeat ? 'text-accent' : 'text-subtext'}`}
            aria-label="Repetir"
          >
            <Repeat size={20} />
          </button>
        </div>
      </div>
    </div>
  )
}
