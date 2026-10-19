import { type StatusTarefa, type Tarefa, useAlterarTarefa } from "@/api/cliente"
import { nomesStatusTarefa } from "@/componentes/rotulos"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"

// Entrar em andamento sobe um agente; sair pede ao agente vivo que encerre (lançador no servidor).
export function AlterarStatus({ tarefa }: { tarefa: Tarefa }) {
  const alterar = useAlterarTarefa(tarefa.id)
  return (
    <div className="space-y-1">
      <Select
        value={tarefa.status}
        disabled={alterar.isPending}
        onValueChange={(status) => alterar.mutate({ status: status as StatusTarefa })}
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
      {alterar.error && <p className="text-destructive text-sm">{alterar.error.message}</p>}
    </div>
  )
}
