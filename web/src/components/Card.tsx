import { useState } from 'react'
import { Play } from 'lucide-react'

interface CardProps {
  image: string
  title: string
  subtitle: string
  onClick?: () => void
  onPlay?: () => void
  square?: boolean
}

export default function Card({ image, title, subtitle, onClick, onPlay, square = true }: CardProps) {
  const [hover, setHover] = useState(false)

  return (
    <div
      className="group w-36 shrink-0 cursor-pointer rounded-lg bg-surface p-3 transition-colors hover:bg-hover sm:w-40"
      onMouseEnter={() => setHover(true)}
      onMouseLeave={() => setHover(false)}
      onClick={onClick}
    >
      <div className={`relative overflow-hidden rounded-md ${square ? 'aspect-square' : ''}`}>
        <img src={image} alt="" className="h-full w-full object-cover" loading="lazy" />
        {onPlay && (
          <button
            onClick={(e) => {
              e.stopPropagation()
              onPlay()
            }}
            className={`absolute bottom-2 right-2 rounded-full bg-accent p-2.5 text-white shadow-lg transition-all ${
              hover ? 'translate-y-0 opacity-100' : 'translate-y-2 opacity-0'
            }`}
            aria-label="Tocar"
          >
            <Play size={18} fill="currentColor" className="ml-0.5" />
          </button>
        )}
      </div>
      <p className="mt-2 truncate text-sm font-semibold">{title}</p>
      <p className="mt-0.5 truncate text-xs text-subtext">{subtitle}</p>
    </div>
  )
}
