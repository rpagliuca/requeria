import { useMemo } from "react"
import { Link } from "react-router-dom"

import { useOrigens, useRequisitos } from "@/api/cliente"
import { Estado } from "@/componentes/Estado"
import { EscolherOrigem } from "@/componentes/EscolherOrigem"

// De onde vêm os requisitos: pessoas e documentos, com quantos requisitos cada uma originou.
export function Origens() {
  const { data, isLoading, error } = useOrigens()
  const requisitos = useRequisitos()
  const contagem = useMemo(() => {
    const c = new Map<string, number>()
    for (const r of requisitos.data ?? []) if (r.origem) c.set(r.origem, (c.get(r.origem) ?? 0) + 1)
    return c
  }, [requisitos.data])

  return (
    <div className="space-y-4">
      <Link to="/requisitos" className="text-muted-foreground text-sm hover:underline">
        ← Requisitos
      </Link>
      <h1 className="font-semibold text-xl">Origens</h1>
      <EscolherOrigem valor="" alterar={() => {}} />
      <Estado carregando={isLoading} erro={error} />
      <ul className="divide-y rounded-md border">
        {(data ?? []).map((o) => (
          <li key={o.id}>
            <Link to={`/requisitos?origem=${o.id}`} className="flex items-center justify-between gap-2 px-3 py-2.5 hover:bg-muted/50">
              <span>
                <span className="font-mono text-muted-foreground text-xs">{o.id}</span> {o.nome}{" "}
                <span className="text-muted-foreground text-xs">({o.tipo})</span>
              </span>
              <span className="text-muted-foreground text-xs">{contagem.get(o.id) ?? 0} requisitos</span>
            </Link>
          </li>
        ))}
      </ul>
    </div>
  )
}
