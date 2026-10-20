import { ChevronDown, ChevronRight, ExternalLink } from "lucide-react"
import { useState } from "react"
import { Link } from "react-router-dom"

import type { Requisito } from "@/api/cliente"
import { EstadoDoRequisito } from "@/componentes/rotulos"
import { descendentes } from "@/componentes/trilhas"
import { cn } from "@/lib/utils"

// Recorte do grafo: os requisitos que casam com o filtro e os ancestrais deles até o zero.
export type Recorte = { casam: Set<string>; visiveis: Set<string> }

export function recortar(porId: Map<string, Requisito>, casa: (r: Requisito) => boolean): Recorte {
  const casam = new Set([...porId.values()].filter(casa).map((r) => r.id))
  const visiveis = new Set<string>()
  const subir = (id: string) => {
    if (visiveis.has(id)) return
    visiveis.add(id)
    for (const p of porId.get(id)?.deriva_de ?? []) subir(p)
  }
  casam.forEach(subir)
  return { casam, visiveis }
}

// Navegação pelo grafo, sempre colapsado de início: clicar num requisito abre os que derivam dele.
// Com recorte, só aparecem os do recorte. Um requisito com vários pais aparece sob cada um; a chave
// do nó é o caminho até ele.
export function GrafoDeRequisitos({
  raizes,
  porId,
  recorte,
}: {
  raizes: string[]
  porId: Map<string, Requisito>
  recorte?: Recorte
}) {
  const [abertos, setAbertos] = useState<Set<string>>(() => new Set())
  const alternar = (chave: string) =>
    setAbertos((atual) => {
      const novo = new Set(atual)
      if (novo.has(chave)) novo.delete(chave)
      else novo.add(chave)
      return novo
    })
  const aberto = (chave: string) => abertos.has(chave)
  const mostradas = recorte ? raizes.filter((id) => recorte.visiveis.has(id)) : raizes
  if (mostradas.length === 0) return <p className="text-muted-foreground text-sm">Nenhum.</p>
  return (
    <ul className="space-y-0.5">
      {mostradas.map((id) => (
        <No key={id} id={id} chave={id} porId={porId} aberto={aberto} alternar={alternar} recorte={recorte} />
      ))}
    </ul>
  )
}

function No({
  id,
  chave,
  porId,
  aberto,
  alternar,
  recorte,
}: {
  id: string
  chave: string
  porId: Map<string, Requisito>
  aberto: (chave: string) => boolean
  alternar: (chave: string) => void
  recorte?: Recorte
}) {
  const r = porId.get(id)
  if (!r) return null
  const filhos = recorte ? r.derivados.filter((f) => recorte.visiveis.has(f)) : r.derivados
  const estaAberto = aberto(chave)
  const abaixo = r.derivados.length > 0 ? descendentes(id, porId) : new Set<string>()
  const emTrabalhoAbaixo = [...abaixo].filter((x) => porId.get(x)?.estado === "em_trabalho").length
  const atencaoAbaixo = [...abaixo].filter((x) => (porId.get(x)?.atencao.length ?? 0) > 0).length
  const soCaminho = recorte && !recorte.casam.has(id)

  return (
    <li>
      <div className="group flex items-start gap-1 rounded-md hover:bg-muted/60">
        <button
          type="button"
          className={cn("flex flex-1 items-start gap-1.5 px-1.5 py-1.5 text-left text-sm disabled:cursor-default", soCaminho && "opacity-60")}
          onClick={() => filhos.length > 0 && alternar(chave)}
          disabled={filhos.length === 0}
        >
          <span className="mt-0.5 w-4 shrink-0 text-muted-foreground">
            {filhos.length > 0 && (estaAberto ? <ChevronDown className="size-4" /> : <ChevronRight className="size-4" />)}
          </span>
          <span className="flex-1">
            <span>{r.titulo}</span>
            <span className="ml-2 inline-flex flex-wrap items-center gap-x-2 text-muted-foreground text-xs">
              <EstadoDoRequisito estado={r.estado} className="h-5 px-1.5 text-[11px]" />
              {filhos.length > 0 && <span>{recorte ? [...abaixo].filter((x) => recorte.casam.has(x)).length : abaixo.size} abaixo</span>}
              {r.tarefas.length > 0 && <span>{r.tarefas.length} tarefas</span>}
              {!recorte && emTrabalhoAbaixo > 0 && <span className="text-blue-600 dark:text-blue-400">● {emTrabalhoAbaixo} em trabalho abaixo</span>}
              {r.atencao.length > 0 && (
                <span className="text-amber-600 dark:text-amber-400" title={r.atencao.map((a) => a.detalhe).join("\n")}>
                  ⚠ {r.atencao.length}
                </span>
              )}
              {!recorte && atencaoAbaixo > 0 && <span className="text-amber-600/80 dark:text-amber-400/80">{atencaoAbaixo} abaixo pedem atenção</span>}
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
      {estaAberto && filhos.length > 0 && (
        <ul className="ml-3.5 space-y-0.5 border-l pl-1.5">
          {filhos.map((f) => (
            <No key={f} id={f} chave={`${chave}>${f}`} porId={porId} aberto={aberto} alternar={alternar} recorte={recorte} />
          ))}
        </ul>
      )}
    </li>
  )
}
