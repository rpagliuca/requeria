import { useMemo } from "react"
import { Link, useSearchParams } from "react-router-dom"

import { type StatusTarefa, useTarefas } from "@/api/cliente"
import { Estado } from "@/componentes/Estado"
import { Data, StatusDaTarefa, nomesStatusTarefa } from "@/componentes/rotulos"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"

const todosOsStatus = Object.keys(nomesStatusTarefa) as StatusTarefa[]
const abertas = todosOsStatus.filter((s) => s !== "concluida")
const TODAS = "todas"

// Filtros ficam na URL: o link reabre a mesma visão.
export function Tarefas() {
  const { data, isLoading, error } = useTarefas()
  const [params, setParams] = useSearchParams()
  const texto = params.get("q") ?? ""
  const organizacao = params.get("org") ?? TODAS
  const status = params.has("status") ? (params.get("status")!.split(",").filter(Boolean) as StatusTarefa[]) : abertas

  const alterar = (chave: string, valor: string | null) => {
    const novos = new URLSearchParams(params)
    if (valor === null) novos.delete(chave)
    else novos.set(chave, valor)
    setParams(novos, { replace: true })
  }
  // Clique mostra só aquele status; Ctrl (ou Cmd) + clique soma ou tira do filtro.
  const escolherStatus = (s: StatusTarefa, somar: boolean) => {
    if (!somar) {
      alterar("status", s)
      return
    }
    const proximos = status.includes(s) ? status.filter((x) => x !== s) : [...status, s]
    alterar("status", proximos.join(","))
  }
  const mesmoConjunto = (a: StatusTarefa[]) => a.length === status.length && a.every((s) => status.includes(s))

  const organizacoes = useMemo(() => [...new Set((data ?? []).map((t) => t.organizacao))].sort(), [data])
  const contagem = useMemo(() => {
    const c = {} as Record<StatusTarefa, number>
    for (const t of data ?? []) if (organizacao === TODAS || t.organizacao === organizacao) c[t.status] = (c[t.status] ?? 0) + 1
    return c
  }, [data, organizacao])

  const filtradas = useMemo(() => {
    const termos = texto.toLowerCase().split(/\s+/).filter(Boolean)
    return (data ?? [])
      .filter((t) => status.includes(t.status))
      .filter((t) => organizacao === TODAS || t.organizacao === organizacao)
      .filter((t) => {
        const alvo = `${t.id} ${t.titulo} ${t.organizacao} ${t.descricao} ${t.resumo}`.toLowerCase()
        return termos.every((termo) => alvo.includes(termo))
      })
      .sort((a, b) => b.atualizada_em.localeCompare(a.atualizada_em))
  }, [data, texto, status, organizacao])

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-2 sm:flex-row">
        <Button asChild>
          <Link to="/tarefas/nova">Nova tarefa</Link>
        </Button>
        <Input
          placeholder="Buscar em id, título, descrição e resumo"
          value={texto}
          onChange={(e) => alterar("q", e.target.value || null)}
        />
        <Select value={organizacao} onValueChange={(v) => alterar("org", v === TODAS ? null : v)}>
          <SelectTrigger className="sm:w-56">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={TODAS}>Todas as organizações</SelectItem>
            {organizacoes.map((o) => (
              <SelectItem key={o} value={o}>
                {o}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <div className="flex flex-wrap gap-1.5">
        <Button size="sm" variant={mesmoConjunto(abertas) ? "default" : "outline"} onClick={() => alterar("status", null)}>
          Abertas
        </Button>
        <Button
          size="sm"
          variant={mesmoConjunto(todosOsStatus) ? "default" : "outline"}
          onClick={() => alterar("status", todosOsStatus.join(","))}
        >
          Todas
        </Button>
        <span className="mx-1 w-px self-stretch bg-border" />
        {todosOsStatus.map((s) => (
          <Button
            key={s}
            size="sm"
            variant={status.includes(s) ? "default" : "outline"}
            onClick={(e) => escolherStatus(s, e.ctrlKey || e.metaKey)}
          >
            {nomesStatusTarefa[s]} <span className="opacity-60">{contagem[s] ?? 0}</span>
          </Button>
        ))}
      </div>

      <Estado carregando={isLoading} erro={error} />
      {data && (
        <>
          <p className="text-muted-foreground text-xs">
            {filtradas.length} tarefas · Ctrl + clique num status soma ao filtro
          </p>
          <ul className="divide-y rounded-md border">
            {filtradas.map((t) => (
              <li key={t.id}>
                <Link to={`/tarefas/${t.id}`} className="block px-3 py-2.5 hover:bg-muted/50">
                  <div className="flex items-start justify-between gap-2">
                    <span className="font-medium">{t.titulo}</span>
                    <StatusDaTarefa status={t.status} />
                  </div>
                  <div className="mt-1 flex flex-wrap gap-x-3 text-muted-foreground text-xs">
                    <span className="font-mono">{t.id}</span>
                    <span>{t.organizacao}</span>
                    {t.sessao_ativa && <span className="text-blue-600 dark:text-blue-400">● {t.sessao_ativa}</span>}
                    <span>
                      atualizada <Data valor={t.atualizada_em} />
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
