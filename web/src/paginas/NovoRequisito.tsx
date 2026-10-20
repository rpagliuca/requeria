import { useMemo, useState } from "react"
import { Link, useNavigate, useSearchParams } from "react-router-dom"

import { useCriarRequisito, useRequisitos } from "@/api/cliente"
import { EscolherOrigem } from "@/componentes/EscolherOrigem"
import { SelecaoMultipla } from "@/componentes/SelecaoMultipla"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"

export function NovoRequisito() {
  const navegar = useNavigate()
  const [params] = useSearchParams()
  const criar = useCriarRequisito()
  const requisitos = useRequisitos()
  const organizacoes = useMemo(() => [...new Set((requisitos.data ?? []).map((r) => r.organizacao))].sort(), [requisitos.data])
  const [titulo, setTitulo] = useState("")
  const [organizacao, setOrganizacao] = useState("")
  const [origem, setOrigem] = useState("")
  const [descricao, setDescricao] = useState("")
  // "?deriva_de=R-0001" vem do botão "Derivar novo requisito" de outro requisito.
  const [pais, setPais] = useState<string[]>(params.get("deriva_de")?.split(",").filter(Boolean) ?? [])

  return (
    <form
      className="space-y-4"
      onSubmit={(e) => {
        e.preventDefault()
        criar.mutate(
          {
            titulo,
            descricao,
            organizacao: organizacao.trim() || undefined,
            origem: origem || undefined,
            deriva_de: pais,
          },
          { onSuccess: (r) => navegar(`/requisitos/${r.id}`) },
        )
      }}
    >
      <Link to="/requisitos" className="text-muted-foreground text-sm hover:underline">
        ← Requisitos
      </Link>
      <h1 className="font-semibold text-xl">Novo requisito</h1>
      <div className="space-y-1.5">
        <Label htmlFor="titulo">Título</Label>
        <Input id="titulo" placeholder="O que precisa ser (e continuar) verdade" value={titulo} onChange={(e) => setTitulo(e.target.value)} required />
      </div>
      <div className="space-y-1.5">
        <Label>Deriva de</Label>
        <SelecaoMultipla
          rotulo="Requisitos"
          opcoes={(requisitos.data ?? []).map((r) => ({ valor: r.id, rotulo: `${r.id} ${r.titulo}` }))}
          selecionados={pais}
          alterar={setPais}
        />
      </div>
      <div className="space-y-1.5">
        <Label>Origem</Label>
        <EscolherOrigem valor={origem} alterar={setOrigem} />
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="organizacao">Organização</Label>
        <Input
          id="organizacao"
          list="organizacoes-requisito"
          placeholder={pais.length > 0 ? "Padrão: a do primeiro requisito de que deriva" : ""}
          value={organizacao}
          onChange={(e) => setOrganizacao(e.target.value)}
        />
        <datalist id="organizacoes-requisito">
          {organizacoes.map((o) => (
            <option key={o} value={o} />
          ))}
        </datalist>
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="descricao">Descrição</Label>
        <Textarea
          id="descricao"
          rows={8}
          placeholder="Inclua o pedido como foi feito (verbatim), quem pediu e quando."
          value={descricao}
          onChange={(e) => setDescricao(e.target.value)}
        />
      </div>
      <Button type="submit" disabled={criar.isPending}>
        Criar
      </Button>
      {criar.error && <p className="text-destructive text-sm">{criar.error.message}</p>}
    </form>
  )
}
