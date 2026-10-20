import { Link } from "react-router-dom"

import { useTarefas } from "@/api/cliente"

// O que identifica uma sessão é a tarefa em que ela trabalha; o agente é detalhe.
export function TituloDasTarefas({ ids, link = true }: { ids: string[]; link?: boolean }) {
  const { data } = useTarefas()
  if (ids.length === 0) return <span className="text-muted-foreground">Sem tarefa vinculada</span>
  const porId = new Map((data ?? []).map((t) => [t.id, t]))
  return (
    <span className="space-x-2">
      {ids.map((id, i) => {
        const texto = (
          <>
            {porId.get(id)?.titulo ?? (data ? "tarefa de outra organização" : id)}
            {i < ids.length - 1 && <span className="text-muted-foreground"> ·</span>}
          </>
        )
        return link ? (
          <Link key={id} to={`/tarefas/${id}`} className="hover:underline">
            {texto}
          </Link>
        ) : (
          <span key={id}>{texto}</span>
        )
      })}
    </span>
  )
}
