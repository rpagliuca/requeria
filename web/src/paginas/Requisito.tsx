import { useMemo, useState } from "react"
import { Link, useParams } from "react-router-dom"

import {
  type Requisito as TipoRequisito,
  type Tarefa,
  useExecucoes,
  useRegistrarExecucao,
  useAlterarRequisito,
  useDerivar,
  useDesfazerDerivacao,
  useOrigens,
  useRequisitos,
  useTarefas,
} from "@/api/cliente"
import { GrafoDeRequisitos } from "@/componentes/GrafoDeRequisitos"
import { EscolherOrigem } from "@/componentes/EscolherOrigem"
import { ComponentesLigados } from "@/componentes/ComponentesLigados"
import { Estado } from "@/componentes/Estado"
import { ForaDaOrganizacao } from "@/componentes/ForaDaOrganizacao"
import { Eventos } from "@/componentes/Eventos"
import { Data, EstadoDoRequisito, StatusDaTarefa, nomesMotivoDeAtencao, nomesPrioridade } from "@/componentes/rotulos"
import { Secao } from "@/componentes/Secao"
import { Trilha, trilhasAteOZero } from "@/componentes/trilhas"
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
  if (!r) return requisitos.data ? <ForaDaOrganizacao /> : <Estado carregando={requisitos.isLoading} erro={requisitos.error} />
  const origem = origens.data?.find((o) => o.id === r.origem)
  const tarefasPorId = new Map((tarefas.data ?? []).map((t) => [t.id, t]))

  return (
    <div className="space-y-6">
      <header className="space-y-2">
        <div className="space-y-0.5">
          {trilhasAteOZero(r.id, porId).map((t) => (
            <div key={t.join(">")}>
              <Trilha ids={t} porId={porId} incluirUltimo={false} />
            </div>
          ))}
        </div>
        <h1 className="font-semibold text-xl">{r.titulo}</h1>
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-muted-foreground text-sm">
          <EstadoDoRequisito estado={r.estado} />
          <Triagem requisito={r} />
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

      {r.atencao.length > 0 && (
        <div className="space-y-1 rounded-md border border-tom-atencao-borda bg-tom-atencao/50 p-3 text-sm">
          <p className="font-medium">Precisa de atenção</p>
          <ul className="list-disc pl-5">
            {r.atencao.map((a) => (
              <li key={a.motivo}>
                <span className="font-medium">{nomesMotivoDeAtencao[a.motivo] ?? a.motivo}:</span> {a.detalhe}
              </li>
            ))}
          </ul>
        </div>
      )}

      <Secao titulo="Estado">
        <PorQue requisito={r} tarefasPorId={tarefasPorId} porId={porId} />
        <Execucoes requisito={r} />
      </Secao>

      <Secao titulo="Componentes">
        <ComponentesLigados alvo="requisitos" id={r.id} ligados={r.componentes} organizacao={r.organizacao} />
      </Secao>

      <Secao titulo="Descrição">
        <Texto>{r.descricao}</Texto>
      </Secao>

      <Secao titulo="Deriva de">
        <Pais requisito={r} porId={porId} />
      </Secao>

      <Secao titulo="O que deriva dele">
        <GrafoDeRequisitos key={r.id} memoria={r.id} raizes={r.derivados} porId={porId} />
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
      {requisito.deriva_de.length === 0 ? (
        <p className="text-muted-foreground text-sm">Este é o requisito zero: todos os outros derivam dele.</p>
      ) : (
        <ul className="divide-y rounded-md border">
          {requisito.deriva_de.map((p) => (
            <li key={p} className="flex items-center justify-between gap-2 px-3 py-2">
              <Link to={`/requisitos/${p}`} className="text-sm hover:underline">
                <span className="font-mono text-muted-foreground text-xs">{p}</span> {porId.get(p)?.titulo}
              </Link>
              {requisito.deriva_de.length > 1 && (
                <Button size="sm" variant="ghost" className="h-7 text-xs" onClick={() => desfazer.mutate(p)}>
                  desfazer
                </Button>
              )}
            </li>
          ))}
        </ul>
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

// O que define o estado: os requisitos abaixo que o limitam, as tarefas abertas e a última execução.
function PorQue({
  requisito: r,
  tarefasPorId,
  porId,
}: {
  requisito: TipoRequisito
  tarefasPorId: Map<string, Tarefa>
  porId: Map<string, TipoRequisito>
}) {
  const e = r.ultima_execucao
  const pelosDerivados = r.tarefas.length === 0 && !e && r.derivados.length > 0
  return (
    <div className="space-y-2 text-sm">
      {r.limitado_por.length > 0 && (
        <div className="space-y-1">
          {pelosDerivados ? (
            <p>Sem tarefa nem execução: quem decide são os requisitos abaixo. Os menos maduros:</p>
          ) : (
            <p>
              Limitado pelo que está abaixo (o estado próprio seria <EstadoDoRequisito estado={r.estado_proprio} />):
            </p>
          )}
          <ul className="space-y-1 pl-4">
            {r.limitado_por.map((rid) => (
              <li key={rid}>
                <Link to={`/requisitos/${rid}`} className="hover:underline">
                  <span className="font-mono text-muted-foreground text-xs">{rid}</span> {porId.get(rid)?.titulo}
                </Link>
                {porId.get(rid) && <EstadoDoRequisito estado={porId.get(rid)!.estado} className="ml-2" />}
              </li>
            ))}
          </ul>
        </div>
      )}
      {r.tarefas_abertas.length > 0 && (
        <ul className="space-y-1">
          {r.tarefas_abertas.map((tid) => (
            <li key={tid}>
              Tarefa aberta:{" "}
              <Link to={`/tarefas/${tid}`} className="hover:underline">
                <span className="font-mono text-muted-foreground text-xs">{tid}</span> {tarefasPorId.get(tid)?.titulo}
              </Link>
              {tarefasPorId.get(tid) && (
                <span className="ml-2">
                  <StatusDaTarefa status={tarefasPorId.get(tid)!.status} />
                </span>
              )}
            </li>
          ))}
        </ul>
      )}
      {r.tarefas_abertas.length === 0 &&
        (!e && r.tarefas.length === 0 ? (
          !pelosDerivados && (
            <p className="text-muted-foreground">
              Sem tarefa e sem validação explícita: nada indica que está atendido. Ligue uma tarefa ou registre uma execução.
            </p>
          )
        ) : !e ? (
          <p className="text-muted-foreground">
            Nenhuma tarefa aberta nele, mas nenhuma execução registrada: tarefas concluídas não provam que o requisito está atendido.
          </p>
        ) : (
          <p>
            Última execução em <Data valor={e.realizada_em} />
            {e.valida_ate ? (
              <>
                , {e.valida_ate < new Date().toISOString() ? "expirou em" : "válida até"} <Data valor={e.valida_ate} />
              </>
            ) : (
              ", sem prazo de validade"
            )}
            .
          </p>
        ))}
    </div>
  )
}

function Execucoes({ requisito }: { requisito: TipoRequisito }) {
  const execucoes = useExecucoes(requisito.id)
  return (
    <div className="space-y-2">
      {(execucoes.data ?? []).length > 0 && (
        <ul className="divide-y rounded-md border text-sm">
          {execucoes.data!.map((e) => (
            <li key={e.id} className="space-y-1 px-3 py-2">
              <div className="flex flex-wrap gap-x-3 text-muted-foreground text-xs">
                <span className="font-mono">{e.id}</span>
                <span>
                  em <Data valor={e.realizada_em} />
                </span>
                <span>{e.valida_ate ? <>válida até <Data valor={e.valida_ate} /></> : "sem prazo"}</span>
                <span>por {e.autor}</span>
              </div>
              <Texto>{e.evidencia}</Texto>
            </li>
          ))}
        </ul>
      )}
      <RegistrarExecucao requisito={requisito} />
    </div>
  )
}

function RegistrarExecucao({ requisito }: { requisito: TipoRequisito }) {
  const registrar = useRegistrarExecucao(requisito.id)
  const [aberto, setAberto] = useState(false)
  const [evidencia, setEvidencia] = useState("")
  const [dias, setDias] = useState("90")
  return (
    <Dialog open={aberto} onOpenChange={setAberto}>
      <DialogTrigger asChild>
        <Button size="sm" variant="outline">
          Registrar execução
        </Button>
      </DialogTrigger>
      <DialogContent>
        <form
          className="space-y-4"
          onSubmit={(e) => {
            e.preventDefault()
            registrar.mutate(
              { evidencia, validade_dias: dias ? Number(dias) : undefined },
              {
                onSuccess: () => {
                  setAberto(false)
                  setEvidencia("")
                },
              },
            )
          }}
        >
          <DialogHeader>
            <DialogTitle>Execução de {requisito.id}</DialogTitle>
          </DialogHeader>
          <p className="text-muted-foreground text-sm">
            Registre só o que foi verificado agora e está conforme: &quot;{requisito.titulo}&quot;.
          </p>
          <div className="space-y-1.5">
            <Label htmlFor="evidencia">Evidência: o que foi verificado e como</Label>
            <Textarea id="evidencia" rows={5} value={evidencia} onChange={(e) => setEvidencia(e.target.value)} required />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="validade">Validade em dias (vazio: não expira)</Label>
            <Input id="validade" type="number" min={1} value={dias} onChange={(e) => setDias(e.target.value)} />
          </div>
          {registrar.error && <p className="text-destructive text-sm">{registrar.error.message}</p>}
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={() => setAberto(false)}>
              Cancelar
            </Button>
            <Button type="submit" disabled={registrar.isPending}>
              Registrar
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

// Triagem num clique: a prioridade muda na hora.
function Triagem({ requisito }: { requisito: TipoRequisito }) {
  const alterar = useAlterarRequisito(requisito.id)
  return (
    <Select value={requisito.prioridade ?? "nao_triado"} onValueChange={(v) => alterar.mutate({ prioridade: v === "nao_triado" ? "" : v })}>
      <SelectTrigger className="h-7 w-auto gap-1 text-xs">
        <span className="text-muted-foreground">Prioridade:</span>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {Object.entries(nomesPrioridade).map(([valor, nome]) => (
          <SelectItem key={valor} value={valor}>
            {nome}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}
