import { useMemo } from "react"
import { Link } from "react-router-dom"

import { useOrigens, useRequisitos } from "@/api/cliente"
import { GrafoDeRequisitos } from "@/componentes/GrafoDeRequisitos"
import { Estado } from "@/componentes/Estado"
import { SelecaoMultipla } from "@/componentes/SelecaoMultipla"
import { useFiltrosLembrados } from "@/hooks/useFiltrosLembrados"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"

// Sem filtro, a cadeia inteira em árvore a partir dos requisitos que não derivam de nenhum; com
// filtro ou busca, a lista dos que casam.
export function Requisitos() {
  const { data, isLoading, error } = useRequisitos()
  const origens = useOrigens()
  const [params, setParams] = useFiltrosLembrados("requisitos")
  const texto = params.get("q") ?? ""
  const lista = (chave: string) => (params.get(chave) ?? "").split(",").filter(Boolean)
  const alterar = (chave: string, valor: string | null) => {
    const novos = new URLSearchParams(params)
    if (valor === null) novos.delete(chave)
    else novos.set(chave, valor)
    setParams(novos, { replace: true })
  }

  const porId = useMemo(() => new Map((data ?? []).map((r) => [r.id, r])), [data])
  const nomesOrigem = useMemo(() => new Map((origens.data ?? []).map((o) => [o.id, o.nome])), [origens.data])
  const organizacoes = useMemo(() => [...new Set((data ?? []).map((r) => r.organizacao))].sort(), [data])
  const orgs = params.has("org") ? lista("org") : organizacoes
  const idsOrigem = (origens.data ?? []).map((o) => o.id)
  const origensSel = params.has("origem") ? lista("origem") : idsOrigem
  const filtrando = !!texto || params.has("org") || params.has("origem")

  const filtrados = useMemo(() => {
    const termos = texto.toLowerCase().split(/\s+/).filter(Boolean)
    return (data ?? [])
      .filter((r) => !params.has("org") || orgs.includes(r.organizacao))
      .filter((r) => !params.has("origem") || (r.origem !== undefined && origensSel.includes(r.origem)))
      .filter((r) => {
        const alvo = `${r.id} ${r.titulo} ${r.descricao} ${r.organizacao} ${nomesOrigem.get(r.origem ?? "") ?? ""}`.toLowerCase()
        return termos.every((t) => alvo.includes(t))
      })
  }, [data, params, nomesOrigem]) // eslint-disable-line react-hooks/exhaustive-deps

  const raizes = (data ?? []).filter((r) => r.deriva_de.length === 0).map((r) => r.id)

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-2 sm:flex-row">
        <Button asChild>
          <Link to="/requisitos/novo">Novo requisito</Link>
        </Button>
        <Input placeholder="Buscar em requisitos" value={texto} onChange={(e) => alterar("q", e.target.value || null)} />
        <SelecaoMultipla
          rotulo="Organização"
          opcoes={organizacoes.map((o) => ({ valor: o, rotulo: o }))}
          selecionados={orgs}
          alterar={(v) => alterar("org", v.length === organizacoes.length ? null : v.join(","))}
          atalhos={[{ rotulo: "Todas", valores: organizacoes }]}
        />
        <SelecaoMultipla
          rotulo="Origem"
          opcoes={(origens.data ?? []).map((o) => ({ valor: o.id, rotulo: o.nome }))}
          selecionados={origensSel}
          alterar={(v) => alterar("origem", v.length === idsOrigem.length ? null : v.join(","))}
          atalhos={[{ rotulo: "Todas", valores: idsOrigem }]}
        />
      </div>
      <p className="text-muted-foreground text-xs">
        {filtrando ? `${filtrados.length} requisitos` : `${data?.length ?? 0} requisitos · clique para abrir os que derivam de cada um`} ·{" "}
        <Link to="/origens" className="underline">
          origens
        </Link>
      </p>

      <Estado carregando={isLoading} erro={error} />
      {data &&
        (filtrando ? (
          <ul className="divide-y rounded-md border">
            {filtrados.map((r) => (
              <li key={r.id}>
                <Link to={`/requisitos/${r.id}`} className="block px-3 py-2.5 hover:bg-muted/50">
                  <div className="font-medium">{r.titulo}</div>
                  <div className="mt-1 flex flex-wrap gap-x-3 text-muted-foreground text-xs">
                    <span className="font-mono">{r.id}</span>
                    <span>{r.organizacao}</span>
                    {r.origem && <span>origem: {nomesOrigem.get(r.origem)}</span>}
                    {r.deriva_de.length > 0 && <span>deriva de {r.deriva_de.join(", ")}</span>}
                    {r.tarefas.length > 0 && <span>{r.tarefas.length} tarefas</span>}
                  </div>
                </Link>
              </li>
            ))}
          </ul>
        ) : data.length === 0 ? (
          <p className="text-muted-foreground text-sm">Nenhum requisito ainda.</p>
        ) : (
          <div className="rounded-md border p-2">
            <GrafoDeRequisitos raizes={raizes} porId={porId} abertosNoInicio={raizes} />
          </div>
        ))}
    </div>
  )
}
