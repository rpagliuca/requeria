import { useMemo, useState } from "react"
import { Link } from "react-router-dom"

import { useDefinirZero, useOrigens, useRequisitos, useZero } from "@/api/cliente"
import { useOrganizacaoAtual } from "@/hooks/useOrganizacaoAtual"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { GrafoDeRequisitos, recortar } from "@/componentes/GrafoDeRequisitos"
import { Estado } from "@/componentes/Estado"
import { nomesEstadoDoRequisito, nomesMotivoDeAtencao, nomesPrioridade, ordemDosEstados } from "@/componentes/rotulos"
import { SelecaoMultipla } from "@/componentes/SelecaoMultipla"
import { useFiltrosLembrados } from "@/hooks/useFiltrosLembrados"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"

// A tela inicial. Sem filtro, a cadeia a partir do zero, aberta sob demanda; com filtro, só os
// requisitos que casam, cada um sob o caminho que leva do zero até ele. O grafo vem colapsado.
export function Requisitos() {
  const { data, isLoading, error } = useRequisitos()
  const { organizacao } = useOrganizacaoAtual()
  const origens = useOrigens()
  const [params, setParams] = useFiltrosLembrados("requisitos", "estado=em_trabalho")
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
  const idsOrigem = (origens.data ?? []).map((o) => o.id)
  const motivos = Object.keys(nomesMotivoDeAtencao)
  const estados = params.has("estado") ? lista("estado") : ordemDosEstados
  const orgs = params.has("org") ? lista("org") : organizacoes
  const origensSel = params.has("origem") ? lista("origem") : idsOrigem
  const atencao = lista("atencao")
  const prioridades = Object.keys(nomesPrioridade)
  const prioSel = params.has("prio") ? lista("prio") : prioridades
  const filtraOrg = !organizacao && params.has("org")
  const filtrando = !!texto || filtraOrg || ["estado", "origem", "atencao", "prio"].some((c) => params.has(c))

  const recorte = useMemo(() => {
    if (!filtrando) return undefined
    const termos = texto.toLowerCase().split(/\s+/).filter(Boolean)
    return recortar(porId, (r) => {
      const alvo = `${r.id} ${r.titulo} ${r.descricao} ${r.organizacao} ${nomesOrigem.get(r.origem ?? "") ?? ""}`.toLowerCase()
      return (
        estados.includes(r.estado) &&
        prioSel.includes(r.prioridade ?? "nao_triado") &&
        (!filtraOrg || orgs.includes(r.organizacao)) &&
        (!params.has("origem") || (r.origem !== undefined && origensSel.includes(r.origem))) &&
        (atencao.length === 0 || r.atencao.some((a) => atencao.includes(a.motivo))) &&
        termos.every((t) => alvo.includes(t))
      )
    })
  }, [porId, params, nomesOrigem]) // eslint-disable-line react-hooks/exhaustive-deps

  const contagem = (estado: string) => (data ?? []).filter((r) => r.estado === estado).length
  const contagemAtencao = (m: string) => (data ?? []).filter((r) => r.atencao.some((a) => a.motivo === m)).length
  const zero = useZero()
  const raizes = zero ? [zero] : []

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-2 sm:flex-row sm:flex-wrap">
        <SelecaoMultipla
          rotulo="Estado"
          opcoes={ordemDosEstados.map((e) => ({ valor: e, rotulo: nomesEstadoDoRequisito[e], contagem: contagem(e) }))}
          selecionados={estados}
          alterar={(v) => alterar("estado", v.length === ordemDosEstados.length ? null : v.join(","))}
          atalhos={[
            { rotulo: "Todos", valores: ordemDosEstados },
            { rotulo: "Pendentes", valores: ["em_trabalho", "nao_atendido", "nao_validado", "validacao_expirada"] },
          ]}
        />
        <SelecaoMultipla
          rotulo="Prioridade"
          opcoes={prioridades.map((p) => ({ valor: p, rotulo: nomesPrioridade[p], contagem: (data ?? []).filter((r) => (r.prioridade ?? "nao_triado") === p).length }))}
          selecionados={prioSel}
          alterar={(v) => alterar("prio", v.length === prioridades.length ? null : v.join(","))}
          atalhos={[
            { rotulo: "Todas", valores: prioridades },
            { rotulo: "Triados", valores: ["alta", "media", "baixa"] },
          ]}
        />
        {!organizacao && <SelecaoMultipla
          rotulo="Organização"
          opcoes={organizacoes.map((o) => ({ valor: o, rotulo: o }))}
          selecionados={orgs}
          alterar={(v) => alterar("org", v.length === organizacoes.length ? null : v.join(","))}
          atalhos={[{ rotulo: "Todas", valores: organizacoes }]}
        />}
        <SelecaoMultipla
          rotulo="Origem"
          opcoes={(origens.data ?? []).map((o) => ({ valor: o.id, rotulo: o.nome }))}
          selecionados={origensSel}
          alterar={(v) => alterar("origem", v.length === idsOrigem.length ? null : v.join(","))}
          atalhos={[{ rotulo: "Todas", valores: idsOrigem }]}
        />
        <SelecaoMultipla
          rotulo="Atenção"
          opcoes={motivos.map((m) => ({ valor: m, rotulo: nomesMotivoDeAtencao[m], contagem: contagemAtencao(m) }))}
          selecionados={atencao}
          alterar={(v) => alterar("atencao", v.length === 0 ? null : v.join(","))}
          atalhos={[
            { rotulo: "Sem filtro", valores: [] },
            { rotulo: "Qualquer motivo", valores: motivos },
          ]}
        />
        <Input className="sm:w-56" placeholder="Buscar" value={texto} onChange={(e) => alterar("q", e.target.value || null)} />
        <Button asChild variant="outline">
          <Link to="/requisitos/novo">Novo requisito</Link>
        </Button>
      </div>
      <p className="text-muted-foreground text-xs">
        {recorte
          ? `${recorte.casam.size} de ${data?.length ?? 0} requisitos, sob o caminho desde o zero (esmaecido) · clique para abrir`
          : `${data?.length ?? 0} requisitos · clique para abrir os que derivam de cada um`}{" "}
        ·{" "}
        <button type="button" className="underline" onClick={() => setParams(new URLSearchParams(), { replace: true })}>
          limpar filtros
        </button>{" "}
        ·{" "}
        <Link to="/origens" className="underline">
          origens
        </Link>
      </p>

      <Estado carregando={isLoading} erro={error} />
      {data &&
        (data.length === 0 ? (
          <p className="text-muted-foreground text-sm">
            Nenhum requisito ainda.{" "}
            <Link to="/requisitos/novo" className="underline">
              Criar o zero
            </Link>
          </p>
        ) : !zero && organizacao ? (
          <DefinirZero organizacao={organizacao} />
        ) : recorte && recorte.casam.size === 0 ? (
          <p className="text-muted-foreground text-sm">Nenhum requisito com esses filtros.</p>
        ) : (
          <div className="rounded-md border p-2">
            <GrafoDeRequisitos key={params.toString()} memoria={`requisitos?${params.toString()}`} raizes={raizes} porId={porId} recorte={recorte} />
          </div>
        ))}
    </div>
  )
}

// Cada organização tem o seu zero; sem ele, a visão da organização não tem por onde começar.
function DefinirZero({ organizacao }: { organizacao: string }) {
  const requisitos = useRequisitos()
  const definir = useDefinirZero(organizacao)
  const [escolhido, setEscolhido] = useState("")
  return (
    <div className="space-y-2 rounded-md border p-3 text-sm">
      <p>{organizacao} ainda não tem requisito zero: escolha o requisito do qual todos os outros dela derivam.</p>
      <div className="flex flex-wrap gap-2">
        <Select value={escolhido} onValueChange={setEscolhido}>
          <SelectTrigger className="sm:w-96">
            <SelectValue placeholder="Requisito zero…" />
          </SelectTrigger>
          <SelectContent>
            {(requisitos.data ?? []).map((r) => (
              <SelectItem key={r.id} value={r.id}>
                {r.id} {r.titulo}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Button disabled={!escolhido || definir.isPending} onClick={() => definir.mutate(escolhido)}>
          Definir zero
        </Button>
      </div>
      {definir.error && <p className="text-destructive">{definir.error.message}</p>}
    </div>
  )
}
