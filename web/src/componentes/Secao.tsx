import type { ReactNode } from "react"

export function Secao({ titulo, recolhida, children }: { titulo: string; recolhida?: boolean; children: ReactNode }) {
  if (recolhida) {
    return (
      <details className="space-y-2">
        <summary className="cursor-pointer font-semibold text-sm uppercase tracking-wide text-muted-foreground">
          {titulo}
        </summary>
        <div className="pt-2">{children}</div>
      </details>
    )
  }
  return (
    <section className="space-y-2">
      <h2 className="font-semibold text-sm uppercase tracking-wide text-muted-foreground">{titulo}</h2>
      {children}
    </section>
  )
}
