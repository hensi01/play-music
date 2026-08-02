import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  Heart,
  ListMusic,
  Maximize2,
  Pause,
  Play,
  Repeat,
  Shuffle,
  SkipBack,
  SkipForward,
  Volume2,
  VolumeX,
} from 'lucide-react'
import { artworkUrl, endpoints } from '../api'
import { resolveDuration, usePlayer } from '../store/player'
import { formatDuration } from '../lib/format'

export default function BottomBar() {
  const player = usePlayer()
  const navigate = useNavigate()
  const { current, playing, progress, duration } = player

  // Media Session API: system media controls + notifications.
  useEffect(() => {
    if (!('mediaSession' in navigator) || !current) return
    navigator.mediaSession.metadata = new MediaMetadata({
      title: current.title,
      artist: current.artist,
      album: current.album,
      artwork: [{ src: artworkUrl(current.albumId, 512), sizes: '512x512' }],
    })
    navigator.mediaSession.setActionHandler('play', () => player.togglePlay())
    navigator.mediaSession.setActionHandler('pause', () => player.togglePlay())
    navigator.mediaSession.setActionHandler('previoustrack', () => player.prev())
    navigator.mediaSession.setActionHandler('nexttrack', () => player.next())
    navigator.mediaSession.setActionHandler('seekto', (d) => {
      if (d.seekTime != null) player.seek(d.seekTime)
    })
    return () => {
      navigator.mediaSession.setActionHandler('play', null)
      navigator.mediaSession.setActionHandler('pause', null)
      navigator.mediaSession.setActionHandler('previoustrack', null)
      navigator.mediaSession.setActionHandler('nexttrack', null)
      navigator.mediaSession.setActionHandler('seekto', null)
    }
  }, [current, playing])

  if (!current) return null

  const totalDuration = resolveDuration(duration, current.duration)
  const safeProgress = Number.isFinite(progress) ? Math.min(Math.max(progress, 0), totalDuration) : 0
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
    <div className="grid shrink-0 grid-cols-1 items-center gap-1 border-t border-grid/50 bg-surface px-3 py-2 md:h-24 md:grid-cols-3 md:gap-4 md:px-4">
      {/* Left: track info */}
      <div className="flex min-w-0 items-center gap-3">
        <img
          src={artworkUrl(current.albumId, 64)}
          alt=""
          className="h-12 w-12 shrink-0 cursor-pointer rounded-md object-cover md:h-14 md:w-14"
          onClick={() => navigate(`/album/${current.albumId}`)}
        />
        <div className="min-w-0">
          <p className="truncate text-sm font-medium">{current.title}</p>
          <p
            className="truncate cursor-pointer text-xs text-subtext hover:text-white hover:underline"
            onClick={() => navigate(`/artist/${current.artistId}`)}
          >
            {current.artist}
          </p>
        </div>
        <button
          onClick={toggleLike}
          className={`ml-1 shrink-0 p-1 ${current.liked ? 'text-accent' : 'text-subtext hover:text-white'}`}
          aria-label={current.liked ? 'Descurtir' : 'Curtir'}
          aria-pressed={current.liked}
        >
          <Heart size={18} fill={current.liked ? 'currentColor' : 'none'} />
        </button>
      </div>

      {/* Center: controls + progress */}
      <div className="flex flex-col items-center gap-1.5 md:order-none order-2">
        <div className="flex items-center gap-4">
          <button
            onClick={player.toggleShuffle}
            className={`p-1 ${player.shuffle ? 'text-accent' : 'text-subtext hover:text-white'}`}
            aria-label="Aleatório"
          >
            <Shuffle size={16} />
          </button>
          <button onClick={player.prev} className="p-1 text-subtext hover:text-white" aria-label="Anterior">
            <SkipBack size={20} fill="currentColor" />
          </button>
          <button
            onClick={player.togglePlay}
            className="rounded-full bg-white p-2 text-black hover:scale-105"
            aria-label={playing ? 'Pausar' : 'Tocar'}
          >
            {playing ? <Pause size={20} fill="currentColor" /> : <Play size={20} fill="currentColor" className="ml-0.5" />}
          </button>
          <button onClick={player.next} className="p-1 text-subtext hover:text-white" aria-label="Próxima">
            <SkipForward size={20} fill="currentColor" />
          </button>
          <button
            onClick={player.toggleRepeat}
            className={`p-1 ${player.repeat ? 'text-accent' : 'text-subtext hover:text-white'}`}
            aria-label="Repetir"
          >
            <Repeat size={16} />
          </button>
        </div>
        <div className="flex w-full max-w-xl items-center gap-2 text-[11px] tabular-nums text-subtext">
          <span className="w-10 text-right">{formatDuration(safeProgress)}</span>
          <div
            className="group relative h-1 flex-1 cursor-pointer rounded-full bg-grid"
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
          <span className="w-10">{formatDuration(totalDuration)}</span>
        </div>
      </div>

      {/* Right: volume + extra */}
      <div className="hidden items-center justify-end gap-2 md:flex">
        {player.volume === 0 ? (
          <VolumeX size={16} className="text-subtext" />
        ) : (
          <Volume2 size={16} className="text-subtext" />
        )}
        <input
          type="range"
          min={0}
          max={1}
          step={0.01}
          value={player.volume}
          onChange={(e) => player.setVolume(parseFloat(e.target.value))}
          className="w-24 accent-white"
          aria-label="Volume"
        />
        <button
          onClick={() => navigate('/queue')}
          className="p-1.5 text-subtext hover:text-white"
          aria-label="Fila"
        >
          <ListMusic size={18} />
        </button>
        <button
          onClick={() => navigate('/lyrics')}
          className="p-1.5 text-subtext hover:text-white"
          aria-label="Letras"
        >
          <span className="text-xs font-bold tracking-wide">LYR</span>
        </button>
        <button
          onClick={() => player.setFullScreen(true)}
          className="p-1.5 text-subtext hover:text-white"
          aria-label="Tela cheia"
        >
          <Maximize2 size={18} />
        </button>
      </div>
    </div>
  )
}
