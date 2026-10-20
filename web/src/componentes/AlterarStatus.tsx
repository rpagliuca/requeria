import { useState } from "react"

import { type StatusTarefa, type Tarefa, useAlterarTarefa, useOpcoesDoLancador } from "@/api/cliente"
import { LancarAgente } from "@/componentes/LancarAgente"
import { nomesStatusTarefa } from "@/componentes/rotulos"
import { Button } from "@/components/ui/button"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"

const todos = Object.keys(nomesStatusTarefa) as StatusTarefa[]

// Um só combo de status. Entrar em andamento sobe um agente e sair derruba o agente vivo: nesses
// dois casos a escolha abre uma confirmação logo abaixo, em vez de agir na hora.
export function AlterarStatus({ tarefa }: { tarefa: Tarefa }) {
  const alterar = useAlterarTarefa(tarefa.id)
  const lancador = useOpcoesDoLancador()
  const [pendente, setPendente] = useState<StatusTarefa | null>(null)
  const comAgente = !!tarefa.sessao_ativa
  const semAgenteEmAndamento = tarefa.status === "em_andamento" && !comAgente && !!lancador.data

  const escolher = (status: StatusTarefa) => {
    const iniciar = status === "em_andamento" && !comAgente && !!lancador.data
    const parar = tarefa.status === "em_andamento" && status !== "em_andamento" && comAgente
    if (iniciar || parar) setPendente(status)
    else alterar.mutate({ status })
  }

  return (
    <div className="space-y-2">
      <div className="flex flex-wrap items-center gap-2">
        <Select value={pendente ?? tarefa.status} disabled={alterar.isPending} onValueChange={(s) => escolher(s as StatusTarefa)}>
          <SelectTrigger className="w-52">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {todos.map((s) => (
              <SelectItem key={s} value={s}>
                {nomesStatusTarefa[s]}
                {s === "em_andamento" && !comAgente && lancador.data ? " (sobe agente)" : ""}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {semAgenteEmAndamento && pendente === null && (
          <Button size="sm" variant="outline" onClick={() => setPendente("em_andamento")}>
            Subir agente…
          </Button>
        )}
      </div>
      {pendente === "em_andamento" && lancador.data && (
        <LancarAgente tarefa={tarefa} opcoes={lancador.data} aoTerminar={() => setPendente(null)} />
      )}
      {pendente && pendente !== "em_andamento" && (
        <PararAgente tarefa={tarefa} destino={pendente} aoTerminar={() => setPendente(null)} />
      )}
      {alterar.error && <p className="text-destructive text-sm">{alterar.error.message}</p>}
    </div>
  )
}

function PararAgente({ tarefa, destino, aoTerminar }: { tarefa: Tarefa; destino: StatusTarefa; aoTerminar: () => void }) {
  const alterar = useAlterarTarefa(tarefa.id)
  return (
    <div className="space-y-3 rounded-md border p-3">
      <p className="text-sm">
        A tarefa vai para <strong>{nomesStatusTarefa[destino].toLowerCase()}</strong>. O agente da sessão{" "}
        {tarefa.sessao_ativa} vai parar o que está fazendo, encerrar a sessão com um resumo e fechar o próprio tmux.
      </p>
      <div className="flex gap-2">
        <Button
          size="sm"
          variant="destructive"
          disabled={alterar.isPending}
          onClick={() => alterar.mutate({ status: destino }, { onSuccess: aoTerminar })}
        >
          Parar agente
        </Button>
        <Button size="sm" variant="ghost" onClick={aoTerminar}>
          Cancelar
        </Button>
      </div>
      {alterar.error && <p className="text-destructive text-sm">{alterar.error.message}</p>}
    </div>
  )
}
