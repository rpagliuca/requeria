import { useMemo, useState } from "react"

import { type Template, useRequisitos, useTarefas } from "@/api/cliente"
import { SelecaoMultipla } from "@/componentes/SelecaoMultipla"
import { Button } from "@/components/ui/button"
import { DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"

export type DadosDoTemplate = {
  titulo: string
  organizacao: string
  descricao: string
  periodicidade_dias: number
  requisitos: string[]
}

// Criar e editar usam o mesmo formulário; periodicidade vazia é sob demanda.
export function FormularioDeTemplate({
  titulo: tituloDoDialogo,
  inicial,
  salvar,
  fechar,
  pendente,
  erro,
}: {
  titulo: string
  inicial?: Template
  salvar: (dados: DadosDoTemplate) => void
  fechar: () => void
  pendente: boolean
  erro?: Error | null
}) {
  const tarefas = useTarefas()
  const requisitos = useRequisitos()
  const organizacoes = useMemo(() => [...new Set((tarefas.data ?? []).map((t) => t.organizacao))].sort(), [tarefas.data])
  const [titulo, setTitulo] = useState(inicial?.titulo ?? "")
  const [organizacao, setOrganizacao] = useState(inicial?.organizacao ?? "")
  const [descricao, setDescricao] = useState(inicial?.descricao ?? "")
  const [dias, setDias] = useState(inicial?.periodicidade_dias?.toString() ?? "")
  const [reqs, setReqs] = useState<string[]>(inicial?.requisitos ?? [])
  return (
    <form
      className="space-y-4"
      onSubmit={(e) => {
        e.preventDefault()
        salvar({ titulo, organizacao, descricao, periodicidade_dias: dias ? Number(dias) : 0, requisitos: reqs })
      }}
    >
      <DialogHeader>
        <DialogTitle>{tituloDoDialogo}</DialogTitle>
      </DialogHeader>
      <div className="space-y-1.5">
        <Label htmlFor="tp-titulo">Título</Label>
        <Input id="tp-titulo" value={titulo} onChange={(e) => setTitulo(e.target.value)} required />
      </div>
      <div className="flex gap-3">
        <div className="flex-1 space-y-1.5">
          <Label htmlFor="tp-org">Organização</Label>
          <Input id="tp-org" list="tp-orgs" value={organizacao} onChange={(e) => setOrganizacao(e.target.value)} required />
          <datalist id="tp-orgs">
            {organizacoes.map((o) => (
              <option key={o} value={o} />
            ))}
          </datalist>
        </div>
        <div className="w-40 space-y-1.5">
          <Label htmlFor="tp-dias">A cada N dias</Label>
          <Input id="tp-dias" type="number" min={1} placeholder="sob demanda" value={dias} onChange={(e) => setDias(e.target.value)} />
        </div>
      </div>
      <div className="space-y-1.5">
        <Label>Requisitos que cada instância atende</Label>
        <SelecaoMultipla
          rotulo="Requisitos"
          opcoes={(requisitos.data ?? []).map((r) => ({ valor: r.id, rotulo: `${r.id} ${r.titulo}` }))}
          selecionados={reqs}
          alterar={setReqs}
        />
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="tp-desc">Instruções (viram a descrição de cada instância)</Label>
        <Textarea id="tp-desc" rows={8} value={descricao} onChange={(e) => setDescricao(e.target.value)} />
      </div>
      {erro && <p className="text-destructive text-sm">{erro.message}</p>}
      <DialogFooter>
        <Button type="button" variant="ghost" onClick={fechar}>
          Cancelar
        </Button>
        <Button type="submit" disabled={pendente}>
          Salvar
        </Button>
      </DialogFooter>
    </form>
  )
}

// Quando um periódico vence, ou "sob demanda".
export function Periodicidade({ template: t }: { template: Template }) {
  if (!t.periodicidade_dias) return <span>sob demanda</span>
  return <span>a cada {t.periodicidade_dias} dias</span>
}
