import { Link } from "react-router-dom"

import type { Sessao } from "@/api/cliente"
import { Data, StatusDaSessao } from "@/componentes/rotulos"
import { Texto } from "@/componentes/Texto"

// Uma sessão no histórico: quem trabalhou, quando, onde está o journal e o resumo que deixou.
export function CartaoSessao({ sessao, comResumo = true }: { sessao: Sessao; comResumo?: boolean }) {
  return (
    <div className="space-y-2 rounded-md border p-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <Link to={`/sessoes/${sessao.id}`} className="font-medium hover:underline">
          <span className="font-mono">{sessao.id}</span> · {sessao.agente}
        </Link>
        <StatusDaSessao status={sessao.status} />
      </div>
      <div className="flex flex-wrap gap-x-3 text-muted-foreground text-xs">
        <span>{sessao.servidor}</span>
        <span>
          <Data valor={sessao.iniciada_em} /> → <Data valor={sessao.encerrada_em} />
        </span>
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
    </div>
  )
}
