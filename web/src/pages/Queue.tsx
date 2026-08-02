import { usePlayer } from '../store/player'
import TrackRow from '../components/TrackRow'

export default function Queue() {
  const { queue, currentIndex } = usePlayer()

  return (
    <div className="px-4 py-6 pb-24 sm:px-6">
      <h1 className="mb-1 text-3xl font-bold">Fila</h1>
      <p className="mb-6 text-sm text-subtext">Próximas músicas na reprodução</p>

      {queue.length === 0 ? (
        <p className="pt-8 text-center text-subtext">A fila está vazia. Toque uma música para começar.</p>
      ) : (
        <div className="rounded-xl bg-surface/30 p-2">
          {queue.map((s, i) => (
            <div key={s.id} className="relative">
              {i === currentIndex && (
                <span className="absolute left-0 top-1/2 -translate-y-1/2 text-accent" aria-hidden>
                  <span className="block h-3 w-1 rounded-full bg-accent" />
                </span>
              )}
              <div className="pl-3">
                <TrackRow song={s} index={i} onPlay={(_song, idx) => usePlayer.getState().playContext(queue, idx)} />
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
