import { useEffect, useState } from "react"
import { Link, useParams } from "react-router-dom"

import {
  type AlteracaoLembrete,
  type Lembrete,
  useAdiarLembrete,
  useAlterarLembrete,
  useCancelarLembrete,
  useConfirmarLembrete,
  useCriarLembrete,
  useLembretes,
  useLimparLembretes,
  useOrganizacoes,
  useReabrirLembrete,
} from "@/api/cliente"
import { Estado } from "@/componentes/Estado"
import { StatusDoLembrete } from "@/componentes/rotulos"
import { Secao } from "@/componentes/Secao"
import { useOrganizacaoAtual } from "@/hooks/useOrganizacaoAtual"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { cn } from "@/lib/utils"

const formatoDaHora = new Intl.DateTimeFormat("pt-BR", { weekday: "short", day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit" })

// Aviso do que a última ação fez, com o desfazer quando ele existe: reabrir o lembrete ou voltar a
// alteração. Quem desfaz é a página, que não some quando o lembrete muda de seção.
type Desfazer = { id: string; alteracao?: AlteracaoLembrete }
type Aviso = { texto: string; desfazer?: Desfazer; erro?: boolean }
type Avisar = (aviso: Aviso) => void

const duracaoDoAviso = 10_000

// Lembretes com hora marcada: o que está tocando primeiro, depois os próximos; quem avisa no celular
// e na barra são as integrações da instalação. Feitos e excluídos ficam em Encerrados até reabrir ou
// apagar de vez.
export function Lembretes() {
  const { id } = useParams()
  const pendentes = useLembretes()
  const encerrados = useLembretes(["confirmado", "cancelado"])
  const lista = pendentes.data ?? []
  const tocando = lista.filter((l) => l.status === "disparado")
  const proximos = lista.filter((l) => l.status !== "disparado")
  const fechados = [...(encerrados.data ?? [])].reverse()
  const [aviso, setAviso] = useState<Aviso>()
  const reabrir = useReabrirLembrete()
  const alterar = useAlterarLembrete()
  const desfazer = ({ id, alteracao }: Desfazer) => {
    const retorno = {
      onSuccess: () => setAviso({ texto: `${id}: desfeito.` }),
      onError: (e: Error) => setAviso({ texto: `${id}: não deu para desfazer (${e.message}).`, erro: true }),
    }
    if (alteracao) alterar.mutate({ id, ...alteracao }, retorno)
    else reabrir.mutate(id, retorno)
  }

  useEffect(() => {
    if (!aviso) return
    const t = setTimeout(() => setAviso(undefined), duracaoDoAviso)
    return () => clearTimeout(t)
  }, [aviso])

  return (
    <div className="space-y-6">
      <h1 className="font-semibold text-xl">Lembretes</h1>
      <NovoLembrete />
      {aviso && <BarraDeAviso aviso={aviso} desfazer={desfazer} ocupado={reabrir.isPending || alterar.isPending} fechar={() => setAviso(undefined)} />}
      <Estado carregando={pendentes.isLoading} erro={pendentes.error} />
      {tocando.length > 0 && (
        <Secao titulo="Tocando">
          <ListaDeLembretes lembretes={tocando} destaque={id} avisar={setAviso} />
        </Secao>
      )}
      <Secao titulo="Próximos">
        {proximos.length === 0 ? (
          <p className="text-muted-foreground text-sm">Nenhum lembrete agendado.</p>
        ) : (
          <ListaDeLembretes lembretes={proximos} destaque={id} avisar={setAviso} />
        )}
      </Secao>
      <Secao titulo={`Encerrados (${fechados.length})`} recolhida>
        {fechados.length === 0 ? (
          <p className="text-muted-foreground text-sm">Nenhum lembrete feito ou excluído.</p>
        ) : (
          <>
            <LimparEncerrados lembretes={fechados} avisar={setAviso} />
            <ListaDeLembretes lembretes={fechados} destaque={id} avisar={setAviso} />
          </>
        )}
      </Secao>
    </div>
  )
}

function BarraDeAviso({ aviso, desfazer, ocupado, fechar }: { aviso: Aviso; desfazer: (d: Desfazer) => void; ocupado: boolean; fechar: () => void }) {
  const alvo = aviso.desfazer
  return (
    <div
      role="status"
      className={cn("flex flex-wrap items-center justify-between gap-2 rounded-md border bg-muted/60 px-3 py-2 text-sm", aviso.erro && "text-destructive")}
    >
      <span>{aviso.texto}</span>
      <span className="flex gap-2">
        {alvo && (
          <Button size="sm" variant="outline" disabled={ocupado} onClick={() => desfazer(alvo)}>
            Desfazer
          </Button>
        )}
        <Button size="sm" variant="ghost" onClick={fechar}>
          Fechar
        </Button>
      </span>
    </div>
  )
}

// Apagar de vez não tem desfazer: o texto sai do Requerial. Por isso pede confirmação.
function LimparEncerrados({ lembretes, avisar }: { lembretes: Lembrete[]; avisar: Avisar }) {
  const limpar = useLimparLembretes()
  return (
    <div className="flex flex-wrap items-center gap-2">
      <Button
        size="sm"
        variant="outline"
        disabled={limpar.isPending}
        onClick={() => {
          if (!window.confirm(`Apagar de vez ${lembretes.length} lembrete(s) encerrado(s)? Não dá para desfazer.`)) return
          limpar.mutate(
            { ids: lembretes.map((l) => l.id) },
            { onSuccess: (r) => avisar({ texto: `${r.apagados.length} lembrete(s) apagado(s) de vez.` }) },
          )
        }}
      >
        Apagar todos de vez
      </Button>
      {limpar.error && <p className="text-destructive text-sm">{limpar.error.message}</p>}
    </div>
  )
}

function ListaDeLembretes({ lembretes, destaque, avisar }: { lembretes: Lembrete[]; destaque?: string; avisar: Avisar }) {
  return (
    <ul className="divide-y rounded-md border">
      {lembretes.map((l) => (
        <ItemDeLembrete key={l.id} lembrete={l} destacado={l.id === destaque} avisar={avisar} />
      ))}
    </ul>
  )
}

// Os botões dependem do status: agendado se edita ou exclui; tocando se marca como feito ou adia;
// encerrado se reabre ou apaga de vez.
function ItemDeLembrete({ lembrete: l, destacado, avisar }: { lembrete: Lembrete; destacado: boolean; avisar: Avisar }) {
  const confirmar = useConfirmarLembrete()
  const adiar = useAdiarLembrete()
  const cancelar = useCancelarLembrete()
  const reabrir = useReabrirLembrete()
  const limpar = useLimparLembretes()
  const desfazer = { id: l.id }
  const [editando, setEditando] = useState(false)
  const encerrado = l.status === "confirmado" || l.status === "cancelado"
  const tocando = l.status === "disparado"
  const erro = confirmar.error ?? adiar.error ?? cancelar.error ?? reabrir.error ?? limpar.error
  const ocupado = confirmar.isPending || adiar.isPending || cancelar.isPending || reabrir.isPending || limpar.isPending
  const adiarPor = (minutos: number) =>
    adiar.mutate(
      { id: l.id, minutos },
      { onSuccess: (x) => avisar({ texto: `${l.id} adiado para ${formatoDaHora.format(new Date(x.quando))}.` }) },
    )
  return (
    <li className={cn("space-y-2 px-3 py-2.5", destacado && "bg-muted/60")}>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span>
          <span className="font-mono text-muted-foreground text-xs">{l.id}</span> {l.texto}
        </span>
        <StatusDoLembrete status={l.status} />
      </div>
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-muted-foreground text-xs">
        <span>{formatoDaHora.format(new Date(l.quando))}</span>
        {l.atrasado && tocando && <span>tocou atrasado</span>}
        {l.adiamentos > 0 && <span>adiado {l.adiamentos}×</span>}
        <span>{l.organizacao}</span>
        {l.tarefa && (
          <Link to={`/tarefas/${l.tarefa}`} className="hover:underline">
            {l.tarefa}
          </Link>
        )}
        {l.requisito && (
          <Link to={`/requisitos/${l.requisito}`} className="hover:underline">
            {l.requisito}
          </Link>
        )}
        <span>por {l.autor}</span>
      </div>
      {editando ? (
        <EdicaoDeLembrete lembrete={l} fechar={() => setEditando(false)} avisar={avisar} />
      ) : (
        <div className="flex flex-wrap gap-2">
          {tocando && (
            <>
              <Button
                size="sm"
                disabled={ocupado}
                onClick={() =>
                  confirmar.mutate(l.id, {
                    onSuccess: () => avisar({ texto: `${l.id} marcado como feito; está em Encerrados.`, desfazer }),
                  })
                }
              >
                Feito
              </Button>
              <Button size="sm" variant="outline" disabled={ocupado} onClick={() => adiarPor(10)}>
                +10 min
              </Button>
              <Button size="sm" variant="outline" disabled={ocupado} onClick={() => adiarPor(60)}>
                +1 h
              </Button>
            </>
          )}
          {!encerrado && (
            <>
              <Button size="sm" variant="outline" disabled={ocupado} onClick={() => setEditando(true)}>
                Editar
              </Button>
              <Button
                size="sm"
                variant="ghost"
                disabled={ocupado}
                onClick={() =>
                  cancelar.mutate(l.id, {
                    onSuccess: () => avisar({ texto: `${l.id} excluído; está em Encerrados.`, desfazer }),
                  })
                }
              >
                Excluir
              </Button>
            </>
          )}
          {encerrado && (
            <>
              <Button
                size="sm"
                variant="outline"
                disabled={ocupado}
                onClick={() => reabrir.mutate(l.id, { onSuccess: (x) => avisar({ texto: `${l.id} reaberto (${x.status === "disparado" ? "tocando" : "agendado"}).` }) })}
              >
                Reabrir
              </Button>
              <Button
                size="sm"
                variant="ghost"
                disabled={ocupado}
                onClick={() => {
                  if (!window.confirm(`Apagar ${l.id} de vez? Não dá para desfazer.`)) return
                  limpar.mutate({ ids: [l.id] }, { onSuccess: () => avisar({ texto: `${l.id} apagado de vez.` }) })
                }}
              >
                Apagar de vez
              </Button>
            </>
          )}
        </div>
      )}
      {erro && <p className="text-destructive text-sm">{erro.message}</p>}
    </li>
  )
}

// A hora do lembrete no formato do datetime-local, no fuso do navegador.
function paraLocal(iso: string): string {
  const d = new Date(iso)
  const dois = (n: number) => String(n).padStart(2, "0")
  return `${d.getFullYear()}-${dois(d.getMonth() + 1)}-${dois(d.getDate())}T${dois(d.getHours())}:${dois(d.getMinutes())}`
}

function EdicaoDeLembrete({ lembrete: l, fechar, avisar }: { lembrete: Lembrete; fechar: () => void; avisar: Avisar }) {
  const alterar = useAlterarLembrete()
  const [texto, setTexto] = useState(l.texto)
  const [quando, setQuando] = useState(paraLocal(l.quando))
  return (
    <form
      className="flex flex-wrap items-end gap-2"
      onSubmit={(e) => {
        e.preventDefault()
        const novaHora = quando !== paraLocal(l.quando)
        const alteracao = { texto: texto !== l.texto ? texto : undefined, quando: novaHora ? comFuso(quando) : undefined }
        if (!alteracao.texto && !alteracao.quando) return fechar()
        alterar.mutate(
          { id: l.id, ...alteracao },
          {
            onSuccess: () => {
              fechar()
              // Volta ao que era; a hora antiga só se ainda estiver no futuro (a API recusa o passado).
              const antes = { texto: alteracao.texto ? l.texto : undefined, quando: novaHora && new Date(l.quando) > new Date() ? l.quando : undefined }
              avisar({ texto: `${l.id} alterado.`, desfazer: antes.texto || antes.quando ? { id: l.id, alteracao: antes } : undefined })
            },
          },
        )
      }}
    >
      <Input aria-label="Lembrar de" className="min-w-48 flex-1" value={texto} onChange={(e) => setTexto(e.target.value)} required />
      <Input aria-label="Quando" type="datetime-local" className="w-auto" value={quando} onChange={(e) => setQuando(e.target.value)} required />
      <Button size="sm" type="submit" disabled={alterar.isPending}>
        Salvar
      </Button>
      <Button size="sm" type="button" variant="ghost" onClick={fechar}>
        Desistir
      </Button>
      {alterar.error && <p className="w-full text-destructive text-sm">{alterar.error.message}</p>}
    </form>
  )
}

// datetime-local não tem fuso: a hora vale no fuso do navegador e vai com o deslocamento dele.
function comFuso(local: string): string {
  const d = new Date(local)
  const minutos = -d.getTimezoneOffset()
  const sinal = minutos >= 0 ? "+" : "-"
  const dois = (n: number) => String(Math.floor(Math.abs(n))).padStart(2, "0")
  return `${local}:00${sinal}${dois(minutos / 60)}:${dois(minutos % 60)}`
}

function NovoLembrete() {
  const criar = useCriarLembrete()
  const organizacoes = useOrganizacoes()
  const atual = useOrganizacaoAtual().organizacao
  const [texto, setTexto] = useState("")
  const [quando, setQuando] = useState("")
  const [organizacao, setOrganizacao] = useState(atual)
  return (
    <form
      className="flex flex-wrap items-end gap-3 rounded-md border p-3"
      onSubmit={(e) => {
        e.preventDefault()
        criar.mutate(
          { texto, quando: comFuso(quando), organizacao },
          {
            onSuccess: () => {
              setTexto("")
              setQuando("")
            },
          },
        )
      }}
    >
      <div className="min-w-48 flex-1 space-y-1.5">
        <Label htmlFor="texto">Lembrar de</Label>
        <Input id="texto" value={texto} onChange={(e) => setTexto(e.target.value)} required />
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="quando">Quando</Label>
        <Input id="quando" type="datetime-local" value={quando} onChange={(e) => setQuando(e.target.value)} required />
      </div>
      <div className="w-36 space-y-1.5">
        <Label htmlFor="organizacao-lembrete">Organização</Label>
        <Input
          id="organizacao-lembrete"
          list="organizacoes-lembrete"
          value={organizacao}
          onChange={(e) => setOrganizacao(e.target.value)}
          required
        />
        <datalist id="organizacoes-lembrete">
          {(organizacoes.data ?? []).map((o) => (
            <option key={o.nome} value={o.nome} />
          ))}
        </datalist>
      </div>
      <Button type="submit" disabled={criar.isPending}>
        Agendar
      </Button>
      {criar.error && <p className="w-full text-destructive text-sm">{criar.error.message}</p>}
    </form>
  )
}
