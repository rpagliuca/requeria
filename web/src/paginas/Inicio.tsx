import { useMemo } from "react"
import { Link } from "react-router-dom"

import { type Requisito, type Tarefa, useRequisitos, useSessoes, useTarefas } from "@/api/cliente"
import { Estado } from "@/componentes/Estado"
import { nomesMotivoDeAtencao } from "@/componentes/rotulos"
import { Secao } from "@/componentes/Secao"
import { GrafoDeRequisitos } from "@/componentes/GrafoDeRequisitos"
import { Trilha, trilhasAteOZero } from "@/componentes/trilhas"

// A home: o que está sendo trabalhado agora, sempre com a cadeia até o zero, e a navegação a partir dele.
export function Inicio() {
  const requisitos = useRequisitos()
  const tarefas = useTarefas()
  const sessoes = useSessoes()
  const porId = useMemo(() => new Map((requisitos.data ?? []).map((r) => [r.id, r])), [requisitos.data])
  const agentes = useMemo(() => new Map((sessoes.data ?? []).map((s) => [s.id, s.agente])), [sessoes.data])

  const emAndamento = (tarefas.data ?? []).filter((t) => t.status === "em_andamento")
  const zero = (requisitos.data ?? []).find((r) => r.deriva_de.length === 0)

  // Requisito → tarefas em andamento que tentam atendê-lo.
  const trabalhados = useMemo(() => {
    const m = new Map<string, Tarefa[]>()
    for (const r of requisitos.data ?? []) {
      const ts = emAndamento.filter((t) => r.tarefas.includes(t.id))
      if (ts.length > 0) m.set(r.id, ts)
    }
    return m
  }, [requisitos.data, tarefas.data]) // eslint-disable-line react-hooks/exhaustive-deps
  const comRequisito = new Set([...trabalhados.values()].flat().map((t) => t.id))
  const semRequisito = emAndamento.filter((t) => !comRequisito.has(t.id))

  if (!requisitos.data) return <Estado carregando={requisitos.isLoading} erro={requisitos.error} />

  return (
    <div className="space-y-8">
      <Secao titulo="Em trabalho agora">
        {trabalhados.size === 0 && <p className="text-muted-foreground text-sm">Nenhum requisito com tarefa em andamento.</p>}
        <div className="space-y-3">
          {[...trabalhados.entries()].map(([rid, ts]) => (
            <CartaoTrabalhado key={rid} requisito={porId.get(rid)!} tarefas={ts} porId={porId} agentes={agentes} />
          ))}
        </div>
        {semRequisito.length > 0 && (
          <div className="space-y-1 pt-2">
            <p className="text-muted-foreground text-xs">Em andamento sem requisito (vincule para dar contexto):</p>
            <ul className="space-y-1">
              {semRequisito.map((t) => (
                <li key={t.id}>
                  <Link to={`/tarefas/${t.id}`} className="text-sm hover:underline">
                    <span className="font-mono text-muted-foreground text-xs">{t.id}</span> {t.titulo}
                    {t.sessao_ativa && <span className="text-muted-foreground text-xs"> · {agentes.get(t.sessao_ativa)}</span>}
                  </Link>
                </li>
              ))}
            </ul>
          </div>
        )}
      </Secao>

      <PrecisamDeAtencao porId={porId} />

      <Secao titulo="A partir do zero">
        {!zero ? (
          <p className="text-sm">
            Nenhum requisito ainda.{" "}
            <Link to="/requisitos/novo" className="underline">
              Criar o zero
            </Link>
          </p>
        ) : (
          <Navegacao zero={zero} porId={porId} trabalhados={trabalhados} />
        )}
      </Secao>
    </div>
  )
}

function CartaoTrabalhado({
  requisito,
  tarefas,
  porId,
  agentes,
}: {
  requisito: Requisito
  tarefas: Tarefa[]
  porId: Map<string, Requisito>
  agentes: Map<string, string>
}) {
  const trilhas = trilhasAteOZero(requisito.id, porId)
  return (
    <div className="space-y-2 rounded-md border p-3">
      <div className="space-y-0.5">
        {trilhas.map((t) => (
          <div key={t.join(">")}>
            <Trilha ids={t} porId={porId} incluirUltimo={false} />
          </div>
        ))}
      </div>
      <Link to={`/requisitos/${requisito.id}`} className="block font-medium hover:underline">
        {requisito.titulo}
      </Link>
      <ul className="space-y-1">
        {tarefas.map((t) => (
          <li key={t.id} className="text-sm">
            <Link to={`/tarefas/${t.id}`} className="hover:underline">
              <span className="font-mono text-muted-foreground text-xs">{t.id}</span> {t.titulo}
            </Link>
            <span className="ml-2 text-blue-600 text-xs dark:text-blue-400">
              {t.sessao_ativa ? `● ${agentes.get(t.sessao_ativa) ?? t.sessao_ativa}` : "sem agente"}
            </span>
          </li>
        ))}
      </ul>
    </div>
  )
}

// O zero aberto, com os filhos; cada clique abre o nível seguinte.
function Navegacao({
  zero,
  porId,
  trabalhados,
}: {
  zero: Requisito
  porId: Map<string, Requisito>
  trabalhados: Map<string, Tarefa[]>
}) {
  return <GrafoDeRequisitos raizes={[zero.id]} porId={porId} emTrabalho={new Set(trabalhados.keys())} abertosNoInicio={[zero.id]} />
}

// Requisitos que não estão bem cuidados, agrupados pelo motivo; cada grupo abre a lista.
function PrecisamDeAtencao({ porId }: { porId: Map<string, Requisito> }) {
  const grupos = new Map<string, Requisito[]>()
  for (const r of porId.values()) for (const a of r.atencao) grupos.set(a.motivo, [...(grupos.get(a.motivo) ?? []), r])
  const total = [...porId.values()].filter((r) => r.atencao.length > 0).length
  return (
    <Secao titulo={`Precisam de atenção (${total})`}>
      {total === 0 ? (
        <p className="text-muted-foreground text-sm">Todos os requisitos estão bem cuidados.</p>
      ) : (
        <div className="space-y-1">
          {Object.keys(nomesMotivoDeAtencao)
            .filter((m) => grupos.has(m))
            .map((m) => (
              <details key={m} className="rounded-md border">
                <summary className="flex cursor-pointer items-center justify-between px-3 py-2 text-sm">
                  <span className="text-amber-700 dark:text-amber-400">⚠ {nomesMotivoDeAtencao[m]}</span>
                  <span className="text-muted-foreground text-xs">{grupos.get(m)!.length}</span>
                </summary>
                <ul className="space-y-2 border-t px-3 py-2">
                  {grupos.get(m)!.map((r) => (
                    <li key={r.id}>
                      <Trilha ids={trilhasAteOZero(r.id, porId)[0]} porId={porId} incluirUltimo={false} />
                      <Link to={`/requisitos/${r.id}`} className="block text-sm hover:underline">
                        {r.titulo}
                      </Link>
                    </li>
                  ))}
                </ul>
              </details>
            ))}
        </div>
      )}
    </Secao>
  )
}
