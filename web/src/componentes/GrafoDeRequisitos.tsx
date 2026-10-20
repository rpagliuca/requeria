import { ChevronDown, ChevronRight, ExternalLink } from "lucide-react"
import { useState } from "react"
import { Link } from "react-router-dom"

import type { Requisito } from "@/api/cliente"
import { descendentes } from "@/componentes/trilhas"

// Navegação pelo grafo sob demanda: clicar num requisito abre os que derivam dele, logo abaixo.
// Um requisito com vários pais aparece sob cada um; a chave do nó é o caminho até ele.
export function GrafoDeRequisitos({
  raizes,
  porId,
  emTrabalho,
  abertosNoInicio = [],
}: {
  raizes: string[]
  porId: Map<string, Requisito>
  emTrabalho?: Set<string>
  abertosNoInicio?: string[]
}) {
  const [abertos, setAbertos] = useState<Set<string>>(() => new Set(abertosNoInicio))
  const alternar = (chave: string) =>
    setAbertos((atual) => {
      const novo = new Set(atual)
      if (novo.has(chave)) novo.delete(chave)
      else novo.add(chave)
      return novo
    })
  if (raizes.length === 0) return <p className="text-muted-foreground text-sm">Nenhum.</p>
  return (
    <ul className="space-y-0.5">
      {raizes.map((id) => (
        <No key={id} id={id} chave={id} porId={porId} abertos={abertos} alternar={alternar} emTrabalho={emTrabalho} />
      ))}
    </ul>
  )
}

function No({
  id,
  chave,
  porId,
  abertos,
  alternar,
  emTrabalho,
}: {
  id: string
  chave: string
  porId: Map<string, Requisito>
  abertos: Set<string>
  alternar: (chave: string) => void
  emTrabalho?: Set<string>
}) {
  const r = porId.get(id)
  if (!r) return null
  const filhos = r.derivados
  const aberto = abertos.has(chave)
  const abaixo = filhos.length > 0 ? descendentes(id, porId) : new Set<string>()
  const ativos = emTrabalho ? [id, ...abaixo].filter((x) => emTrabalho.has(x)).length : 0
  const atencaoAbaixo = [...abaixo].filter((x) => (porId.get(x)?.atencao.length ?? 0) > 0).length

  return (
    <li>
      <div className="group flex items-start gap-1 rounded-md hover:bg-muted/60">
        <button
          type="button"
          className="flex flex-1 items-start gap-1.5 px-1.5 py-1.5 text-left text-sm disabled:cursor-default"
          onClick={() => filhos.length > 0 && alternar(chave)}
          disabled={filhos.length === 0}
        >
          <span className="mt-0.5 w-4 shrink-0 text-muted-foreground">
            {filhos.length > 0 && (aberto ? <ChevronDown className="size-4" /> : <ChevronRight className="size-4" />)}
          </span>
          <span className="flex-1">
            <span>{r.titulo}</span>
            <span className="ml-2 inline-flex flex-wrap gap-x-2 text-muted-foreground text-xs">
              {filhos.length > 0 && <span>{abaixo.size} abaixo</span>}
              {r.tarefas.length > 0 && <span>{r.tarefas.length} tarefas</span>}
              {ativos > 0 && <span className="text-blue-600 dark:text-blue-400">● {ativos} em trabalho</span>}
              {r.atencao.length > 0 && (
                <span className="text-amber-600 dark:text-amber-400" title={r.atencao.map((a) => a.detalhe).join("\n")}>
                  ⚠ {r.atencao.length}
                </span>
              )}
              {atencaoAbaixo > 0 && <span className="text-amber-600/80 dark:text-amber-400/80">{atencaoAbaixo} abaixo pedem atenção</span>}
            </span>
          </span>
        </button>
        <Link
          to={`/requisitos/${id}`}
          className="px-2 py-1.5 text-muted-foreground hover:text-foreground"
          title={`Abrir ${id}`}
          aria-label={`Abrir ${id}`}
        >
          <ExternalLink className="size-4" />
        </Link>
      </div>
      {aberto && (
        <ul className="ml-3.5 space-y-0.5 border-l pl-1.5">
          {filhos.map((f) => (
            <No key={f} id={f} chave={`${chave}>${f}`} porId={porId} abertos={abertos} alternar={alternar} emTrabalho={emTrabalho} />
          ))}
        </ul>
      )}
    </li>
  )
}
