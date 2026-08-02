import type { ReactNode } from 'react'

interface SectionProps {
  title: string
  children: ReactNode
}

export default function Section({ title, children }: SectionProps) {
  return (
    <section className="px-4 py-3 sm:px-6">
      <h2 className="mb-3 text-xl font-bold">{title}</h2>
      <div className="flex gap-3 overflow-x-auto pb-2">{children}</div>
    </section>
  )
}
