import { useMemo, useState } from "react"
import { Link, useParams } from "react-router-dom"

import {
  type Requisito as TipoRequisito,
  useAlterarRequisito,
  useDerivar,
  useDesfazerDerivacao,
  useOrigens,
  useRequisitos,
  useTarefas,
} from "@/api/cliente"
import { ArvoreDeRequisitos } from "@/componentes/ArvoreDeRequisitos"
import { EscolherOrigem } from "@/componentes/EscolherOrigem"
import { Estado } from "@/componentes/Estado"
import { Eventos } from "@/componentes/Eventos"
import { Data, StatusDaTarefa } from "@/componentes/rotulos"
import { Secao } from "@/componentes/Secao"
import { Texto } from "@/componentes/Texto"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Textarea } from "@/components/ui/textarea"

export function Requisito() {
  const { id = "" } = useParams()
  const requisitos = useRequisitos()
  const origens = useOrigens()
  const tarefas = useTarefas()
  const porId = useMemo(() => new Map((requisitos.data ?? []).map((r) => [r.id, r])), [requisitos.data])
  const r = porId.get(id)
  if (!r) return <Estado carregando={requisitos.isLoading} erro={requisitos.error} />
  const origem = origens.data?.find((o) => o.id === r.origem)
  const tarefasPorId = new Map((tarefas.data ?? []).map((t) => [t.id, t]))

  return (
    <div className="space-y-6">
      <header className="space-y-2">
        <Link to="/requisitos" className="text-muted-foreground text-sm hover:underline">
          ← Requisitos
        </Link>
        <h1 className="font-semibold text-xl">{r.titulo}</h1>
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-muted-foreground text-sm">
          <span className="font-mono">{r.id}</span>
          <span>{r.organizacao}</span>
          {origem && (
            <Link to={`/requisitos?origem=${origem.id}`} className="hover:underline">
              origem: {origem.nome} ({origem.tipo})
            </Link>
          )}
          <span>
            atualizado <Data valor={r.atualizado_em} />
          </span>
          <EditarRequisito requisito={r} />
        </div>
      </header>

      <Secao titulo="Descrição">
        <Texto>{r.descricao}</Texto>
      </Secao>

      <Secao titulo="De onde vem (acima na cadeia)">
        <ArvoreDeRequisitos raizes={r.deriva_de} porId={porId} sentido="acima" />
        <Pais requisito={r} porId={porId} />
      </Secao>

      <Secao titulo="O que deriva dele (abaixo na cadeia)">
        <ArvoreDeRequisitos raizes={r.derivados} porId={porId} sentido="abaixo" />
        <Button asChild size="sm" variant="outline">
          <Link to={`/requisitos/novo?deriva_de=${r.id}`}>Derivar novo requisito</Link>
        </Button>
      </Secao>

      <Secao titulo={`Tarefas que tentam atendê-lo (${r.tarefas.length})`}>
        {r.tarefas.length === 0 ? (
          <p className="text-muted-foreground text-sm">Nenhuma. Vincule pela página da tarefa.</p>
        ) : (
          <ul className="divide-y rounded-md border">
            {r.tarefas.map((tid) => {
              const t = tarefasPorId.get(tid)
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
        )}
      </Secao>

      <Secao titulo="Eventos" recolhida>
        <Eventos entidade={r.id} />
      </Secao>
    </div>
  )
}

// Os pais diretos, com a opção de desfazer cada derivação, e a de acrescentar um pai.
function Pais({ requisito, porId }: { requisito: TipoRequisito; porId: Map<string, TipoRequisito> }) {
  const derivar = useDerivar(requisito.id)
  const desfazer = useDesfazerDerivacao(requisito.id)
  const [pai, setPai] = useState("")
  const candidatos = [...porId.values()].filter((x) => x.id !== requisito.id && !requisito.deriva_de.includes(x.id))
  const erro = derivar.error ?? desfazer.error
  return (
    <div className="space-y-2">
      {requisito.deriva_de.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {requisito.deriva_de.map((p) => (
            <Button key={p} size="sm" variant="ghost" className="h-7 text-xs" onClick={() => desfazer.mutate(p)}>
              não deriva mais de {p} ✕
            </Button>
          ))}
        </div>
      )}
      <div className="flex flex-wrap items-center gap-2">
        <Select value={pai} onValueChange={setPai}>
          <SelectTrigger className="sm:w-80">
            <SelectValue placeholder="Também deriva de…" />
          </SelectTrigger>
          <SelectContent>
            {candidatos.map((c) => (
              <SelectItem key={c.id} value={c.id}>
                {c.id} {c.titulo}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Button size="sm" variant="outline" disabled={!pai || derivar.isPending} onClick={() => derivar.mutate(pai, { onSuccess: () => setPai("") })}>
          Acrescentar
        </Button>
      </div>
      {erro && <p className="text-destructive text-sm">{erro.message}</p>}
    </div>
  )
}

function EditarRequisito({ requisito }: { requisito: TipoRequisito }) {
  const [aberto, setAberto] = useState(false)
  return (
    <Dialog open={aberto} onOpenChange={setAberto}>
      <DialogTrigger asChild>
        <Button size="sm" variant="outline" className="h-7">
          Editar
        </Button>
      </DialogTrigger>
      <DialogContent className="max-h-[90vh] overflow-y-auto">
        {aberto && <FormularioDeEdicao requisito={requisito} fechar={() => setAberto(false)} />}
      </DialogContent>
    </Dialog>
  )
}

function FormularioDeEdicao({ requisito, fechar }: { requisito: TipoRequisito; fechar: () => void }) {
  const alterar = useAlterarRequisito(requisito.id)
  const [titulo, setTitulo] = useState(requisito.titulo)
  const [organizacao, setOrganizacao] = useState(requisito.organizacao)
  const [origem, setOrigem] = useState(requisito.origem ?? "")
  const [descricao, setDescricao] = useState(requisito.descricao)
  return (
    <form
      className="space-y-4"
      onSubmit={(e) => {
        e.preventDefault()
        alterar.mutate({ titulo, organizacao, origem, descricao }, { onSuccess: fechar })
      }}
    >
      <DialogHeader>
        <DialogTitle>Editar {requisito.id}</DialogTitle>
      </DialogHeader>
      <div className="space-y-1.5">
        <Label htmlFor="titulo-req">Título</Label>
        <Input id="titulo-req" value={titulo} onChange={(e) => setTitulo(e.target.value)} required />
      </div>
      <div className="space-y-1.5">
        <Label>Origem</Label>
        <EscolherOrigem valor={origem} alterar={setOrigem} />
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="org-req">Organização</Label>
        <Input id="org-req" value={organizacao} onChange={(e) => setOrganizacao(e.target.value)} required />
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="desc-req">Descrição</Label>
        <Textarea id="desc-req" rows={8} value={descricao} onChange={(e) => setDescricao(e.target.value)} />
      </div>
      {alterar.error && <p className="text-destructive text-sm">{alterar.error.message}</p>}
      <DialogFooter>
        <Button type="button" variant="ghost" onClick={fechar}>
          Cancelar
        </Button>
        <Button type="submit" disabled={alterar.isPending}>
          Salvar
        </Button>
      </DialogFooter>
    </form>
  )
}
