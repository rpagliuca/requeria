import { Link, useParams } from "react-router-dom"

import { useSessoesDaTarefa, useTarefa } from "@/api/cliente"
import { AlterarStatus } from "@/componentes/AlterarStatus"
import { CartaoSessao } from "@/componentes/CartaoSessao"
import { Estado } from "@/componentes/Estado"
import { Eventos } from "@/componentes/Eventos"
import { Data } from "@/componentes/rotulos"
import { RequisitosDaTarefa } from "@/componentes/RequisitosDaTarefa"
import { Secao } from "@/componentes/Secao"
import { Texto } from "@/componentes/Texto"

export function Tarefa() {
  const { id = "" } = useParams()
  const tarefa = useTarefa(id)
  const sessoes = useSessoesDaTarefa(id)
  const t = tarefa.data
  if (!t) return <Estado carregando={tarefa.isLoading} erro={tarefa.error} />

  return (
    <div className="space-y-6">
      <header className="space-y-2">
        <Link to="/tarefas" className="text-muted-foreground text-sm hover:underline">
          ← Tarefas
        </Link>
        <h1 className="font-semibold text-xl">{t.titulo}</h1>
        <AlterarStatus tarefa={t} />
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-muted-foreground text-sm">
          <span className="font-mono">{t.id}</span>
          <span>{t.organizacao}</span>
          <span>
            criada <Data valor={t.criada_em} />
          </span>
          <span>
            atualizada <Data valor={t.atualizada_em} />
          </span>
          {t.sessao_ativa && (
            <Link to={`/sessoes/${t.sessao_ativa}`} className="text-blue-600 hover:underline dark:text-blue-400">
              ● sessão ativa {t.sessao_ativa}
            </Link>
          )}
        </div>
      </header>

      <Secao titulo="Requisitos que ela tenta atender">
        <RequisitosDaTarefa tarefa={t} />
      </Secao>
      <Secao titulo="Resumo para retomar">
        <Texto>{t.resumo}</Texto>
      </Secao>
      <Secao titulo="Descrição">
        <Texto>{t.descricao}</Texto>
      </Secao>
      <Secao titulo={`Sessões de trabalho${sessoes.data ? ` (${sessoes.data.length})` : ""}`}>
        <Estado carregando={sessoes.isLoading} erro={sessoes.error} />
        <div className="space-y-2">
          {sessoes.data?.map((s) => <CartaoSessao key={s.id} sessao={s} comTarefa={s.tarefas.length > 1} />)}
        </div>
      </Secao>
      <Secao titulo="Eventos" recolhida>
        <Eventos entidade={t.id} />
      </Secao>
    </div>
  )
}
