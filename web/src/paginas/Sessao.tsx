import { Link, useParams } from "react-router-dom"

import { useSessao, useTarefas } from "@/api/cliente"
import { CartaoSessao } from "@/componentes/CartaoSessao"
import { Estado } from "@/componentes/Estado"
import { ForaDaOrganizacao } from "@/componentes/ForaDaOrganizacao"
import { useOrganizacaoAtual } from "@/hooks/useOrganizacaoAtual"
import { Voltar } from "@/componentes/Voltar"
import { Eventos } from "@/componentes/Eventos"
import { StatusDaTarefa } from "@/componentes/rotulos"
import { Secao } from "@/componentes/Secao"
import { Texto } from "@/componentes/Texto"
import { TituloDasTarefas } from "@/componentes/TituloDasTarefas"

export function Sessao() {
  const { id = "" } = useParams()
  const sessao = useSessao(id)
  const { organizacao } = useOrganizacaoAtual()
  const tarefas = useTarefas()
  const s = sessao.data
  if (!s) return <Estado carregando={sessao.isLoading} erro={sessao.error} />
  const porId = new Map((tarefas.data ?? []).map((t) => [t.id, t]))
  if (organizacao && tarefas.data && (s.tarefas.length === 0 || s.tarefas.some((t) => !porId.has(t)))) return <ForaDaOrganizacao />

  return (
    <div className="space-y-6">
      <Voltar para="/sessoes" rotulo="Sessões" />
      <h1 className="font-semibold text-xl">
        <TituloDasTarefas ids={s.tarefas} link={false} />
      </h1>
      <CartaoSessao sessao={s} comResumo={false} comTarefa={false} />

      <Secao titulo="Tarefas">
        <ul className="divide-y rounded-md border">
          {s.tarefas.map((tid) => {
            const t = porId.get(tid)
            return (
              <li key={tid}>
                <Link to={`/tarefas/${tid}`} className="flex items-start justify-between gap-2 px-3 py-2.5 hover:bg-muted/50">
                  <span>
                    <span className="font-mono text-muted-foreground text-xs">{tid}</span> {t?.titulo}
                  </span>
                  {t && <StatusDaTarefa status={t.status} />}
                </Link>
              </li>
            )
          })}
        </ul>
      </Secao>
      <Secao titulo="Resumo da sessão">
        <Texto>{s.resumo ?? ""}</Texto>
      </Secao>
      {s.artefatos.length > 0 && (
        <Secao titulo="Artefatos">
          <ul className="list-disc pl-5 font-mono text-xs break-all">
            {s.artefatos.map((a) => (
              <li key={a}>{a}</li>
            ))}
          </ul>
        </Secao>
      )}
      <Secao titulo="Eventos" recolhida>
        <Eventos entidade={s.id} />
      </Secao>
    </div>
  )
}
