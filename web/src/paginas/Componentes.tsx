import { useMemo, useState } from "react"
import { Link } from "react-router-dom"

import { useComponentes, useCriarComponente, useOrganizacoes } from "@/api/cliente"
import { SelecaoMultipla } from "@/componentes/SelecaoMultipla"
import { Estado } from "@/componentes/Estado"
import { useOrganizacaoAtual } from "@/hooks/useOrganizacaoAtual"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"

// Componentes: repositórios, módulos, lugares, funcionalidades… qualquer coisa a que requisitos e
// tarefas se referem. Sem tipo.
export function Componentes() {
  const { data, isLoading, error } = useComponentes()
  const { organizacao: atual } = useOrganizacaoAtual()
  const criar = useCriarComponente()
  const [busca, setBusca] = useState("")
  const [nome, setNome] = useState("")
  const organizacoes = useOrganizacoes()
  const [orgs, setOrgs] = useState<string[]>(atual ? [atual] : [])
  const filtrados = useMemo(() => {
    const termos = busca.toLowerCase().split(/\s+/).filter(Boolean)
    return (data ?? []).filter((c) => termos.every((t) => `${c.id} ${c.nome} ${c.descricao} ${c.organizacoes.join(" ")}`.toLowerCase().includes(t)))
  }, [data, busca])

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-2 sm:flex-row sm:flex-wrap">
        <Input className="sm:w-56" placeholder="Buscar" value={busca} onChange={(e) => setBusca(e.target.value)} />
        <form
          className="flex flex-wrap gap-2"
          onSubmit={(e) => {
            e.preventDefault()
            criar.mutate({ nome, organizacoes: orgs }, { onSuccess: () => setNome("") })
          }}
        >
          <Input className="sm:w-56" placeholder="Novo componente" value={nome} onChange={(e) => setNome(e.target.value)} required />
          <SelecaoMultipla
            rotulo="Organizações"
            opcoes={(organizacoes.data ?? []).filter((o) => o.zero).map((o) => ({ valor: o.nome, rotulo: o.nome }))}
            selecionados={orgs}
            alterar={setOrgs}
          />
          <Button type="submit" variant="outline" disabled={criar.isPending || orgs.length === 0}>
            Criar
          </Button>
        </form>
      </div>
      {criar.error && <p className="text-destructive text-sm">{criar.error.message}</p>}
      <p className="text-muted-foreground text-xs">
        {filtrados.length} de {data?.length ?? 0} componentes
      </p>
      <Estado carregando={isLoading} erro={error} />
      <ul className="divide-y rounded-md border">
        {filtrados.map((c) => (
          <li key={c.id}>
            <Link to={`/componentes/${c.id}`} className="flex items-start justify-between gap-2 px-3 py-2.5 hover:bg-muted/50">
              <span>
                <span className="font-medium">{c.nome}</span>
                <span className="ml-2 font-mono text-muted-foreground text-xs">{c.id}</span>
                <span className="ml-2 text-muted-foreground text-xs">{c.organizacoes.join(", ")}</span>
              </span>
              <span className="shrink-0 text-muted-foreground text-xs">
                {c.requisitos.length} requisitos · {c.tarefas.length} tarefas
              </span>
            </Link>
          </li>
        ))}
      </ul>
    </div>
  )
}
