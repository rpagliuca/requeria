import { Link } from "react-router-dom"

import type { Sessao } from "@/api/cliente"
import { EncerrarSessao } from "@/componentes/EncerrarSessao"
import { Data, StatusDaSessao } from "@/componentes/rotulos"
import { Texto } from "@/componentes/Texto"
import { TituloDasTarefas } from "@/componentes/TituloDasTarefas"

// Uma sessão no histórico: em que tarefa, quando, onde está o journal, quem trabalhou e o resumo.
export function CartaoSessao({ sessao, comResumo = true, comTarefa = true }: { sessao: Sessao; comResumo?: boolean; comTarefa?: boolean }) {
  return (
    <div className="space-y-2 rounded-md border p-3">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div className="font-medium">
          {comTarefa ? <TituloDasTarefas ids={sessao.tarefas} /> : <Data valor={sessao.iniciada_em} />}
        </div>
        <StatusDaSessao status={sessao.status} />
      </div>
      <div className="flex flex-wrap gap-x-3 text-muted-foreground text-xs">
        <Link to={`/sessoes/${sessao.id}`} className="font-mono hover:underline">
          {sessao.id}
        </Link>
        <span>
          <Data valor={sessao.iniciada_em} /> → <Data valor={sessao.encerrada_em} />
        </span>
        <span>{sessao.agente}</span>
        <span>{sessao.servidor}</span>
        {sessao.anterior && (
          <span>
            continua{" "}
            <Link className="font-mono hover:underline" to={`/sessoes/${sessao.anterior}`}>
              {sessao.anterior}
            </Link>
          </span>
        )}
      </div>
      {sessao.journal && <div className="font-mono text-xs break-all">journal: {sessao.journal}</div>}
      {comResumo && sessao.resumo && (
        <details>
          <summary className="cursor-pointer text-sm">Resumo da sessão</summary>
          <div className="mt-2">
            <Texto>{sessao.resumo}</Texto>
          </div>
        </details>
      )}
      {sessao.status === "ativa" && <EncerrarSessao id={sessao.id} />}
    </div>
  )
}
