import { useState } from "react"
import { Link, useNavigate, useParams } from "react-router-dom"

import { useAlterarTemplate, useInstanciarTemplate, useRequisitos, useTarefas, useTemplates } from "@/api/cliente"
import { Estado } from "@/componentes/Estado"
import { ForaDaOrganizacao } from "@/componentes/ForaDaOrganizacao"
import { Voltar } from "@/componentes/Voltar"
import { Eventos } from "@/componentes/Eventos"
import { FormularioDeTemplate, Periodicidade } from "@/componentes/FormularioDeTemplate"
import { Data, StatusDaTarefa } from "@/componentes/rotulos"
import { Secao } from "@/componentes/Secao"
import { Texto } from "@/componentes/Texto"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"

export function Template() {
  const { id = "" } = useParams()
  const templates = useTemplates()
  const requisitos = useRequisitos()
  const tarefas = useTarefas()
  const t = templates.data?.find((x) => x.id === id)
  if (!t) return templates.data ? <ForaDaOrganizacao /> : <Estado carregando={templates.isLoading} erro={templates.error} />
  const req = new Map((requisitos.data ?? []).map((r) => [r.id, r]))
  const tar = new Map((tarefas.data ?? []).map((x) => [x.id, x]))

  return (
    <div className="space-y-6">
      <header className="space-y-2">
        <Voltar para="/templates" rotulo="Tarefas Pré-moldadas" />
        <h1 className="font-semibold text-xl">{t.titulo}</h1>
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-muted-foreground text-sm">
          {t.devido && (
            <Badge variant="outline" className="border-transparent bg-tom-atencao text-tom-atencao-fg">
              Devida agora
            </Badge>
          )}
          <span className="font-mono">{t.id}</span>
          <span>{t.organizacao}</span>
          <Periodicidade template={t} />
          {t.proxima_em && !t.devido && (
            <span>
              próxima em <Data valor={t.proxima_em} />
            </span>
          )}
          <Editar id={t.id} />
          <Instanciar id={t.id} />
        </div>
        {t.instancia_aberta && (
          <p className="text-sm">
            Há uma instância aberta:{" "}
            <Link to={`/tarefas/${t.instancia_aberta}`} className="underline">
              {t.instancia_aberta}
            </Link>
          </p>
        )}
      </header>

      <Secao titulo="Requisitos que cada instância atende">
        {t.requisitos.length === 0 ? (
          <p className="text-muted-foreground text-sm">Nenhum. Edite a pré-moldada para ligar.</p>
        ) : (
          <ul className="space-y-1 text-sm">
            {t.requisitos.map((rid) => (
              <li key={rid}>
                <Link to={`/requisitos/${rid}`} className="hover:underline">
                  <span className="font-mono text-muted-foreground text-xs">{rid}</span> {req.get(rid)?.titulo}
                </Link>
              </li>
            ))}
          </ul>
        )}
      </Secao>
      <Secao titulo="Instruções">
        <Texto>{t.descricao}</Texto>
      </Secao>
      <Secao titulo={`Instâncias (${t.instancias.length})`}>
        {t.instancias.length === 0 ? (
          <p className="text-muted-foreground text-sm">Nenhuma ainda.</p>
        ) : (
          <ul className="divide-y rounded-md border">
            {t.instancias.map((tid) => {
              const x = tar.get(tid)
              return (
                <li key={tid}>
                  <Link to={`/tarefas/${tid}`} className="flex items-start justify-between gap-2 px-3 py-2.5 hover:bg-muted/50">
                    <span>
                      <span className="font-mono text-muted-foreground text-xs">{tid}</span> {x?.titulo}
                      {x && (
                        <span className="ml-2 text-muted-foreground text-xs">
                          <Data valor={x.criada_em} />
                        </span>
                      )}
                    </span>
                    {x && <StatusDaTarefa status={x.status} />}
                  </Link>
                </li>
              )
            })}
          </ul>
        )}
      </Secao>
      <Secao titulo="Eventos" recolhida>
        <Eventos entidade={t.id} />
      </Secao>
    </div>
  )
}

function Editar({ id }: { id: string }) {
  const templates = useTemplates()
  const alterar = useAlterarTemplate(id)
  const [aberto, setAberto] = useState(false)
  const t = templates.data?.find((x) => x.id === id)
  return (
    <Dialog open={aberto} onOpenChange={setAberto}>
      <DialogTrigger asChild>
        <Button size="sm" variant="outline" className="h-7">
          Editar
        </Button>
      </DialogTrigger>
      <DialogContent className="max-h-[90vh] overflow-y-auto">
        {aberto && t && (
          <FormularioDeTemplate
            titulo={`Editar ${id}`}
            inicial={t}
            fechar={() => setAberto(false)}
            pendente={alterar.isPending}
            erro={alterar.error}
            salvar={(d) => alterar.mutate(d, { onSuccess: () => setAberto(false) })}
          />
        )}
      </DialogContent>
    </Dialog>
  )
}

// Cria a tarefa (a fazer) a partir do template, com o contexto desta vez; o agente dá o título.
// Para pôr um agente nela, "Alterar status" na tarefa.
function Instanciar({ id }: { id: string }) {
  const instanciar = useInstanciarTemplate(id)
  const navegar = useNavigate()
  const [aberto, setAberto] = useState(false)
  const [contexto, setContexto] = useState("")
  return (
    <Dialog open={aberto} onOpenChange={setAberto}>
      <DialogTrigger asChild>
        <Button size="sm" className="h-7">
          Instanciar
        </Button>
      </DialogTrigger>
      <DialogContent>
        <form
          className="space-y-4"
          onSubmit={(e) => {
            e.preventDefault()
            instanciar.mutate(contexto.trim() || undefined, { onSuccess: (t) => navegar(`/tarefas/${t.id}`) })
          }}
        >
          <DialogHeader>
            <DialogTitle>Nova tarefa a partir da pré-moldada {id}</DialogTitle>
          </DialogHeader>
          <div className="space-y-1.5">
            <Label htmlFor="inst-contexto">Contexto desta vez</Label>
            <Textarea
              id="inst-contexto"
              rows={5}
              placeholder="Ex.: link do PR, quem pediu, o que mudou desde a última vez…"
              value={contexto}
              onChange={(e) => setContexto(e.target.value)}
            />
          </div>
          <p className="text-muted-foreground text-sm">
            A tarefa nasce a fazer, com o contexto no topo, as instruções e os requisitos da pré-moldada; o agente dá o título ao começar. Para pôr um agente nela, use Alterar status.
          </p>
          {instanciar.error && <p className="text-destructive text-sm">{instanciar.error.message}</p>}
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={() => setAberto(false)}>
              Cancelar
            </Button>
            <Button type="submit" disabled={instanciar.isPending}>
              Criar tarefa
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
