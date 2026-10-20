import { Badge } from "@/components/ui/badge"
import { cn } from "@/lib/utils"

import type { EstadoDoRequisito, StatusSessao, StatusTarefa } from "@/api/cliente"

export const nomesStatusTarefa: Record<StatusTarefa, string> = {
  a_fazer: "A fazer",
  em_andamento: "Em andamento",
  pausada: "Pausada",
  bloqueada: "Bloqueada",
  em_validacao: "Em validação",
  concluida: "Concluída",
}

const coresStatusTarefa: Record<StatusTarefa, string> = {
  a_fazer: "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200",
  em_andamento: "bg-blue-100 text-blue-800 dark:bg-blue-950 dark:text-blue-200",
  pausada: "bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-200",
  bloqueada: "bg-red-100 text-red-800 dark:bg-red-950 dark:text-red-200",
  em_validacao: "bg-violet-100 text-violet-800 dark:bg-violet-950 dark:text-violet-200",
  concluida: "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-200",
}

export function StatusDaTarefa({ status }: { status: StatusTarefa }) {
  return (
    <Badge variant="outline" className={cn("border-transparent", coresStatusTarefa[status])}>
      {nomesStatusTarefa[status]}
    </Badge>
  )
}

export const nomesStatusSessao: Record<StatusSessao, string> = {
  ativa: "Ativa",
  encerrada: "Encerrada",
  substituida: "Substituída",
}

export function StatusDaSessao({ status }: { status: StatusSessao }) {
  return (
    <Badge variant={status === "ativa" ? "default" : "secondary"}>{nomesStatusSessao[status]}</Badge>
  )
}

const formatoData = new Intl.DateTimeFormat("pt-BR", { dateStyle: "short", timeStyle: "short" })

export function Data({ valor }: { valor?: string }) {
  if (!valor) return <span className="text-muted-foreground">—</span>
  return <time dateTime={valor}>{formatoData.format(new Date(valor))}</time>
}

export const nomesMotivoDeAtencao: Record<string, string> = {
  muitos_filhos: "Mais de 5 requisitos abaixo",
  sem_origem: "Sem origem",
  sem_descricao: "Sem descrição",
  sem_tarefa: "Ninguém trabalhando nele",
  trabalho_parado: "Trabalho parado",
}

export const nomesEstadoDoRequisito: Record<EstadoDoRequisito, string> = {
  nao_atendido: "Não atendido",
  em_trabalho: "Em trabalho",
  nao_validado: "Não validado",
  validacao_expirada: "Validação expirada",
  validado: "Validado",
}

// Do menos ao mais maduro.
export const ordemDosEstados = Object.keys(nomesEstadoDoRequisito) as EstadoDoRequisito[]

const coresEstadoDoRequisito: Record<EstadoDoRequisito, string> = {
  em_trabalho: "bg-blue-100 text-blue-800 dark:bg-blue-950 dark:text-blue-200",
  nao_atendido: "bg-red-100 text-red-800 dark:bg-red-950 dark:text-red-200",
  nao_validado: "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200",
  validado: "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-200",
  validacao_expirada: "bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-200",
}

// Estado calculado pelo servidor: o menos maduro entre o próprio (tarefas e última execução) e os de baixo.
export function EstadoDoRequisito({ estado, className }: { estado: EstadoDoRequisito; className?: string }) {
  return (
    <Badge variant="outline" className={cn("border-transparent", coresEstadoDoRequisito[estado], className)}>
      {nomesEstadoDoRequisito[estado]}
    </Badge>
  )
}

export const nomesPrioridade: Record<string, string> = {
  alta: "Alta",
  media: "Média",
  baixa: "Baixa",
  nao_triado: "Não triado",
}

export const ordemDaPrioridade: Record<string, number> = { alta: 0, media: 1, baixa: 2, nao_triado: 3 }

const coresPrioridade: Record<string, string> = {
  alta: "border-red-400 text-red-700 dark:border-red-700 dark:text-red-300",
  media: "border-amber-400 text-amber-700 dark:border-amber-700 dark:text-amber-300",
  baixa: "border-slate-300 text-slate-600 dark:border-slate-600 dark:text-slate-300",
}

// Prioridade da triagem; sem ela, nada aparece.
export function PrioridadeDoRequisito({ prioridade, className }: { prioridade?: string; className?: string }) {
  if (!prioridade) return null
  return (
    <Badge variant="outline" className={cn(coresPrioridade[prioridade], className)}>
      ↑ {nomesPrioridade[prioridade]}
    </Badge>
  )
}
