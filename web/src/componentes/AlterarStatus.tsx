import { useState } from "react"

import { type StatusTarefa, type Tarefa, useAlterarTarefa, useOpcoesDoLancador } from "@/api/cliente"
import { LancarAgente } from "@/componentes/LancarAgente"
import { nomesStatusTarefa } from "@/componentes/rotulos"
import { Button } from "@/components/ui/button"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"

const paradas = (Object.keys(nomesStatusTarefa) as StatusTarefa[]).filter((s) => s !== "em_andamento")

// Entrar em andamento sobe um agente e sair derruba o agente vivo: os dois são ações explícitas,
// com confirmação. O select só troca entre os status parados.
export function AlterarStatus({ tarefa }: { tarefa: Tarefa }) {
  const alterar = useAlterarTarefa(tarefa.id)
  const lancador = useOpcoesDoLancador()
  const [acao, setAcao] = useState<"iniciar" | "parar" | null>(null)
  const emAndamento = tarefa.status === "em_andamento"

  return (
    <div className="space-y-2">
      <div className="flex flex-wrap items-center gap-2">
        {emAndamento ? (
          <span className="rounded-md bg-blue-100 px-3 py-1.5 text-blue-800 text-sm dark:bg-blue-950 dark:text-blue-200">
            Em andamento{tarefa.sessao_ativa ? ` · sessão ${tarefa.sessao_ativa}` : " · sem agente"}
          </span>
        ) : (
          <Select
            value={tarefa.status}
            disabled={alterar.isPending}
            onValueChange={(status) => alterar.mutate({ status: status as StatusTarefa })}
          >
            <SelectTrigger className="w-48">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {paradas.map((s) => (
                <SelectItem key={s} value={s}>
                  {nomesStatusTarefa[s]}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}
        {!tarefa.sessao_ativa && lancador.data && acao === null && (
          <Button size="sm" onClick={() => setAcao("iniciar")}>
            Iniciar agente…
          </Button>
        )}
        {emAndamento && acao === null && (
          <Button size="sm" variant="outline" onClick={() => setAcao("parar")}>
            {tarefa.sessao_ativa ? "Parar agente…" : "Tirar de andamento…"}
          </Button>
        )}
      </div>
      {acao === "iniciar" && lancador.data && (
        <LancarAgente tarefa={tarefa} opcoes={lancador.data} aoTerminar={() => setAcao(null)} />
      )}
      {acao === "parar" && <PararAgente tarefa={tarefa} aoTerminar={() => setAcao(null)} />}
      {alterar.error && <p className="text-destructive text-sm">{alterar.error.message}</p>}
    </div>
  )
}

function PararAgente({ tarefa, aoTerminar }: { tarefa: Tarefa; aoTerminar: () => void }) {
  const alterar = useAlterarTarefa(tarefa.id)
  const [destino, setDestino] = useState<StatusTarefa>("pausada")
  return (
    <div className="space-y-3 rounded-md border p-3">
      <p className="text-sm">
        {tarefa.sessao_ativa
          ? `O agente da sessão ${tarefa.sessao_ativa} vai parar o que está fazendo, encerrar a sessão com um resumo e fechar o próprio tmux.`
          : "Não há agente nesta tarefa; só o status muda."}
      </p>
      <div className="flex flex-wrap items-center gap-2">
        <span className="text-sm">Mover para</span>
        <Select value={destino} onValueChange={(v) => setDestino(v as StatusTarefa)}>
          <SelectTrigger className="w-44">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {paradas.map((s) => (
              <SelectItem key={s} value={s}>
                {nomesStatusTarefa[s]}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <div className="flex gap-2">
        <Button
          size="sm"
          variant="destructive"
          disabled={alterar.isPending}
          onClick={() => alterar.mutate({ status: destino }, { onSuccess: aoTerminar })}
        >
          {tarefa.sessao_ativa ? "Parar agente" : "Mover"}
        </Button>
        <Button size="sm" variant="ghost" onClick={aoTerminar}>
          Cancelar
        </Button>
      </div>
      {alterar.error && <p className="text-destructive text-sm">{alterar.error.message}</p>}
    </div>
  )
}
