import { useMemo } from "react"
import { Link } from "react-router-dom"

import { type StatusSessao, useSessoes, useTarefas } from "@/api/cliente"
import { Estado } from "@/componentes/Estado"
import { useFiltrosLembrados } from "@/hooks/useFiltrosLembrados"
import { Data, StatusDaSessao, nomesStatusSessao } from "@/componentes/rotulos"
import { TituloDasTarefas } from "@/componentes/TituloDasTarefas"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"

const todosOsStatus = Object.keys(nomesStatusSessao) as StatusSessao[]

export function Sessoes() {
  const { data, isLoading, error } = useSessoes()
  const tarefas = useTarefas()
  const [params, setParams] = useFiltrosLembrados("sessoes")
  const texto = params.get("q") ?? ""
  const status = (params.get("status") ?? "ativa") as StatusSessao | "todas"

  const alterar = (chave: string, valor: string | null) => {
    const novos = new URLSearchParams(params)
    if (valor === null) novos.delete(chave)
    else novos.set(chave, valor)
    setParams(novos, { replace: true })
  }

  const filtradas = useMemo(() => {
    const titulos = new Map((tarefas.data ?? []).map((t) => [t.id, t.titulo]))
    const termos = texto.toLowerCase().split(/\s+/).filter(Boolean)
    return (data ?? [])
      .filter((s) => status === "todas" || s.status === status)
      .filter((s) => {
        const alvo = `${s.id} ${s.agente} ${s.servidor} ${s.tarefas.map((t) => `${t} ${titulos.get(t) ?? ""}`).join(" ")} ${s.journal ?? ""}`.toLowerCase()
        return termos.every((termo) => alvo.includes(termo))
      })
      .sort((a, b) => (b.encerrada_em ?? b.iniciada_em).localeCompare(a.encerrada_em ?? a.iniciada_em))
  }, [data, tarefas.data, texto, status])

  return (
    <div className="space-y-4">
      <Input
        placeholder="Buscar por tarefa, id, agente, máquina ou journal"
        value={texto}
        onChange={(e) => alterar("q", e.target.value || null)}
      />
      <div className="flex flex-wrap gap-1.5">
        {[...todosOsStatus, "todas" as const].map((s) => (
          <Button key={s} size="sm" variant={status === s ? "default" : "outline"} onClick={() => alterar("status", s)}>
            {s === "todas" ? "Todas" : nomesStatusSessao[s]}
          </Button>
        ))}
      </div>

      <Estado carregando={isLoading} erro={error} />
      {data && (
        <>
          <p className="text-muted-foreground text-xs">{filtradas.length} sessões</p>
          <ul className="divide-y rounded-md border">
            {filtradas.map((s) => (
              <li key={s.id}>
                <Link to={`/sessoes/${s.id}`} className="block px-3 py-2.5 hover:bg-muted/50">
                  <div className="flex items-start justify-between gap-2">
                    <span className="font-medium">
                      <TituloDasTarefas ids={s.tarefas} link={false} />
                    </span>
                    <StatusDaSessao status={s.status} />
                  </div>
                  <div className="mt-1 flex flex-wrap gap-x-3 text-muted-foreground text-xs">
                    <span className="font-mono">{s.id}</span>
                    <span className="font-mono">{s.tarefas.join(", ")}</span>
                    <span>{s.agente}</span>
                    <span>{s.servidor}</span>
                    <span>
                      <Data valor={s.iniciada_em} /> → <Data valor={s.encerrada_em} />
                    </span>
                  </div>
                </Link>
              </li>
            ))}
          </ul>
        </>
      )}
    </div>
  )
}
