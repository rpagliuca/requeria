import { useState } from "react"

import { type StatusTarefa, type Tarefa, useAlterarTarefa, useOpcoesDoLancador } from "@/api/cliente"
import { LancarAgente } from "@/componentes/LancarAgente"
import { nomesStatusTarefa } from "@/componentes/rotulos"
import { Button } from "@/components/ui/button"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"

// Pôr em andamento sobe um agente (com instrução, modelo e esforço escolhidos aqui); sair de em
// andamento pede ao agente vivo que encerre. Os dois são feitos pelo lançador no servidor.
export function AlterarStatus({ tarefa }: { tarefa: Tarefa }) {
  const alterar = useAlterarTarefa(tarefa.id)
  const lancador = useOpcoesDoLancador()
  const [lancando, setLancando] = useState(false)
  const podeLancar = !!lancador.data && !tarefa.sessao_ativa

  return (
    <div className="space-y-2">
      <div className="flex flex-wrap items-center gap-2">
        <Select
          value={tarefa.status}
          disabled={alterar.isPending}
          onValueChange={(status) => {
            if (status === "em_andamento" && podeLancar) setLancando(true)
            else alterar.mutate({ status: status as StatusTarefa })
          }}
        >
          <SelectTrigger className="w-48">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {(Object.keys(nomesStatusTarefa) as StatusTarefa[]).map((s) => (
              <SelectItem key={s} value={s}>
                {nomesStatusTarefa[s]}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {tarefa.status === "em_andamento" && podeLancar && !lancando && (
          <Button size="sm" variant="outline" onClick={() => setLancando(true)}>
            Subir agente
          </Button>
        )}
      </div>
      {lancando && lancador.data && (
        <LancarAgente tarefa={tarefa} opcoes={lancador.data} aoTerminar={() => setLancando(false)} />
      )}
      {alterar.error && <p className="text-destructive text-sm">{alterar.error.message}</p>}
    </div>
  )
}
