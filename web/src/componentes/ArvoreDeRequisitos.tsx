import { Link } from "react-router-dom"

import type { Requisito } from "@/api/cliente"

// Desenha a cadeia a partir de alguns requisitos, seguindo os pais (acima) ou os derivados (abaixo).
// No grafo, um requisito com vários pais aparece sob cada um deles.
export function ArvoreDeRequisitos({
  raizes,
  porId,
  sentido,
  destacar,
}: {
  raizes: string[]
  porId: Map<string, Requisito>
  sentido: "acima" | "abaixo"
  destacar?: string
}) {
  if (raizes.length === 0) return <p className="text-muted-foreground text-sm">Nenhum.</p>
  return (
    <ul className="space-y-1">
      {raizes.map((id) => (
        <No key={id} id={id} porId={porId} sentido={sentido} destacar={destacar} caminho={[]} />
      ))}
    </ul>
  )
}

function No({
  id,
  porId,
  sentido,
  destacar,
  caminho,
}: {
  id: string
  porId: Map<string, Requisito>
  sentido: "acima" | "abaixo"
  destacar?: string
  caminho: string[]
}) {
  const r = porId.get(id)
  const proximos = r ? (sentido === "acima" ? r.deriva_de : r.derivados).filter((p) => !caminho.includes(p)) : []
  return (
    <li>
      <Link
        to={`/requisitos/${id}`}
        className={`block rounded-md px-2 py-1 text-sm hover:bg-muted ${id === destacar ? "bg-muted font-medium" : ""}`}
      >
        <span className="font-mono text-muted-foreground text-xs">{id}</span> {r?.titulo ?? ""}
        {r && r.tarefas.length > 0 && (
          <span className="ml-2 text-muted-foreground text-xs">
            {r.tarefas.length} tarefa{r.tarefas.length > 1 ? "s" : ""}
          </span>
        )}
      </Link>
      {proximos.length > 0 && (
        <ul className="ml-3 space-y-1 border-l pl-2">
          {proximos.map((p) => (
            <No key={p} id={p} porId={porId} sentido={sentido} destacar={destacar} caminho={[...caminho, id]} />
          ))}
        </ul>
      )}
    </li>
  )
}
