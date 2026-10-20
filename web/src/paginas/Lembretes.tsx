import { useState } from "react"
import { Link, useParams } from "react-router-dom"

import {
  type Lembrete,
  useAdiarLembrete,
  useCancelarLembrete,
  useConfirmarLembrete,
  useCriarLembrete,
  useLembretes,
  useOrganizacoes,
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

// Lembretes com hora marcada: o que está tocando primeiro, depois os próximos; quem avisa no celular
// e na barra são as integrações da instalação.
export function Lembretes() {
  const { id } = useParams()
  const pendentes = useLembretes()
  const encerrados = useLembretes(["confirmado", "cancelado"])
  const lista = pendentes.data ?? []
  const tocando = lista.filter((l) => l.status === "disparado")
  const proximos = lista.filter((l) => l.status !== "disparado")

  return (
    <div className="space-y-6">
      <h1 className="font-semibold text-xl">Lembretes</h1>
      <NovoLembrete />
      <Estado carregando={pendentes.isLoading} erro={pendentes.error} />
      {tocando.length > 0 && (
        <Secao titulo="Tocando">
          <ListaDeLembretes lembretes={tocando} destaque={id} />
        </Secao>
      )}
      <Secao titulo="Próximos">
        {proximos.length === 0 ? (
          <p className="text-muted-foreground text-sm">Nenhum lembrete agendado.</p>
        ) : (
          <ListaDeLembretes lembretes={proximos} destaque={id} />
        )}
      </Secao>
      <Secao titulo="Encerrados" recolhida>
        <ListaDeLembretes lembretes={[...(encerrados.data ?? [])].reverse()} destaque={id} />
      </Secao>
    </div>
  )
}

function ListaDeLembretes({ lembretes, destaque }: { lembretes: Lembrete[]; destaque?: string }) {
  return (
    <ul className="divide-y rounded-md border">
      {lembretes.map((l) => (
        <ItemDeLembrete key={l.id} lembrete={l} destacado={l.id === destaque} />
      ))}
    </ul>
  )
}

function ItemDeLembrete({ lembrete: l, destacado }: { lembrete: Lembrete; destacado: boolean }) {
  const confirmar = useConfirmarLembrete()
  const adiar = useAdiarLembrete()
  const cancelar = useCancelarLembrete()
  const aberto = l.status !== "confirmado" && l.status !== "cancelado"
  const erro = confirmar.error ?? adiar.error ?? cancelar.error
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
        {l.atrasado && l.status === "disparado" && <span>tocou atrasado</span>}
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
      {aberto && (
        <div className="flex flex-wrap gap-2">
          <Button size="sm" onClick={() => confirmar.mutate(l.id)} disabled={confirmar.isPending}>
            Confirmar
          </Button>
          <Button size="sm" variant="outline" onClick={() => adiar.mutate({ id: l.id, minutos: 10 })} disabled={adiar.isPending}>
            +10 min
          </Button>
          <Button size="sm" variant="outline" onClick={() => adiar.mutate({ id: l.id, minutos: 60 })} disabled={adiar.isPending}>
            +1 h
          </Button>
          <Button size="sm" variant="ghost" onClick={() => cancelar.mutate(l.id)} disabled={cancelar.isPending}>
            Cancelar
          </Button>
        </div>
      )}
      {erro && <p className="text-destructive text-sm">{erro.message}</p>}
    </li>
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
