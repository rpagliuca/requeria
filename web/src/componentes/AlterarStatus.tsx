import { useState } from "react"

import {
  type StatusTarefa,
  type Tarefa,
  useAlterarTarefa,
  useLancarAgente,
  useOpcoesDoLancador,
} from "@/api/cliente"
import { StatusDaTarefa, nomesStatusTarefa } from "@/componentes/rotulos"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import { Label } from "@/components/ui/label"
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Textarea } from "@/components/ui/textarea"

const todos = Object.keys(nomesStatusTarefa) as StatusTarefa[]
const PADRAO = "padrao"

// Status atual e o botão que abre a tela de mudança. Entrar em andamento sobe um agente (com
// modelo, esforço e instrução de reabertura); sair de em andamento faz o agente vivo encerrar.
export function AlterarStatus({ tarefa }: { tarefa: Tarefa }) {
  const [aberto, setAberto] = useState(false)
  return (
    <div className="flex flex-wrap items-center gap-2">
      <StatusDaTarefa status={tarefa.status} />
      <Dialog open={aberto} onOpenChange={setAberto}>
        <DialogTrigger asChild>
          <Button size="sm" variant="outline">
            Alterar status
          </Button>
        </DialogTrigger>
        <DialogContent className="max-h-[90vh] overflow-y-auto">
          {aberto && <TelaDeStatus tarefa={tarefa} fechar={() => setAberto(false)} />}
        </DialogContent>
      </Dialog>
    </div>
  )
}

function TelaDeStatus({ tarefa, fechar }: { tarefa: Tarefa; fechar: () => void }) {
  const alterar = useAlterarTarefa(tarefa.id)
  const lancar = useLancarAgente(tarefa.id)
  const opcoes = useOpcoesDoLancador().data
  const [destino, setDestino] = useState<StatusTarefa>(tarefa.status)
  const [instrucao, setInstrucao] = useState("")
  const [modelo, setModelo] = useState(opcoes?.modelo_padrao ?? "")
  const [esforco, setEsforco] = useState(opcoes?.esforco_padrao ?? PADRAO)

  const comAgente = !!tarefa.sessao_ativa
  // Sobe agente: indo para em andamento (ou já nela, sem agente) e com lançador.
  const iniciar = destino === "em_andamento" && !comAgente && !!opcoes
  const parar = tarefa.status === "em_andamento" && destino !== "em_andamento" && comAgente
  const nada = destino === tarefa.status && !iniciar
  const erro = alterar.error ?? lancar.error

  const confirmar = () => {
    if (iniciar) {
      lancar.mutate(
        { instrucao: instrucao.trim() || undefined, modelo: modelo || undefined, esforco: esforco === PADRAO ? undefined : esforco },
        { onSuccess: fechar },
      )
    } else {
      alterar.mutate({ status: destino }, { onSuccess: fechar })
    }
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>Alterar status</DialogTitle>
        <DialogDescription>{tarefa.titulo}</DialogDescription>
      </DialogHeader>

      <RadioGroup value={destino} onValueChange={(v) => setDestino(v as StatusTarefa)} className="gap-1">
        {todos.map((s) => (
          <Label key={s} className="flex cursor-pointer items-center gap-3 rounded-md px-2 py-2 font-normal hover:bg-muted">
            <RadioGroupItem value={s} />
            <span className="flex-1">{nomesStatusTarefa[s]}</span>
            {s === tarefa.status && <span className="text-muted-foreground text-xs">atual</span>}
          </Label>
        ))}
      </RadioGroup>

      {iniciar && opcoes && (
        <div className="space-y-3 rounded-md border p-3">
          <p className="text-sm">Um agente sobe para trabalhar nesta tarefa.</p>
          <div className="space-y-1.5">
            <Label htmlFor="instrucao">Instrução adicional de reabertura (opcional)</Label>
            <Textarea
              id="instrucao"
              rows={4}
              placeholder="O que mudou ou o que fazer agora. O resto o agente lê da tarefa e das sessões anteriores."
              value={instrucao}
              onChange={(e) => setInstrucao(e.target.value)}
            />
          </div>
          <div className="flex flex-wrap gap-3">
            <div className="space-y-1.5">
              <Label>Modelo</Label>
              <Select value={modelo} onValueChange={setModelo}>
                <SelectTrigger className="w-52">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {opcoes.modelos.map((m) => (
                    <SelectItem key={m} value={m}>
                      {m}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label>Esforço</Label>
              <Select value={esforco} onValueChange={setEsforco}>
                <SelectTrigger className="w-36">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {!opcoes.esforco_padrao && <SelectItem value={PADRAO}>Padrão do agente</SelectItem>}
                  {(opcoes.esforcos ?? []).map((e) => (
                    <SelectItem key={e} value={e}>
                      {e}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
        </div>
      )}
      {parar && (
        <p className="rounded-md border border-destructive/40 p-3 text-sm">
          O agente da sessão {tarefa.sessao_ativa} vai parar o que está fazendo, encerrar a sessão com um resumo e fechar o
          próprio tmux.
        </p>
      )}
      {erro && <p className="text-destructive text-sm">{erro.message}</p>}

      <DialogFooter>
        <Button variant="ghost" onClick={fechar}>
          Cancelar
        </Button>
        <Button
          variant={parar ? "destructive" : "default"}
          disabled={nada || alterar.isPending || lancar.isPending}
          onClick={confirmar}
        >
          {iniciar ? "Iniciar agente" : parar ? "Parar agente e alterar" : "Alterar"}
        </Button>
      </DialogFooter>
    </>
  )
}
