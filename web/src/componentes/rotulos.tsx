import { Badge } from "@/components/ui/badge"
import { cn } from "@/lib/utils"
import { contornoDoTom, seloDoTom, type Tom } from "@/componentes/tons"

import type { EstadoDoRequisito, StatusSessao, StatusTarefa } from "@/api/cliente"

export const nomesStatusTarefa: Record<StatusTarefa, string> = {
  a_fazer: "A fazer",
  em_andamento: "Em andamento",
  pausada: "Pausada",
  bloqueada: "Bloqueada",
  em_validacao: "Em validação",
  concluida: "Concluída",
}

const tomDoStatusTarefa: Record<StatusTarefa, Tom> = {
  a_fazer: "neutro",
  em_andamento: "andamento",
  pausada: "atencao",
  bloqueada: "perigo",
  em_validacao: "validacao",
  concluida: "sucesso",
}

export function StatusDaTarefa({ status }: { status: StatusTarefa }) {
  return (
    <Badge variant="outline" className={cn("border-transparent", seloDoTom[tomDoStatusTarefa[status]])}>
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

const tomDoEstadoDoRequisito: Record<EstadoDoRequisito, Tom> = {
  em_trabalho: "andamento",
  nao_atendido: "perigo",
  nao_validado: "neutro",
  validado: "sucesso",
  validacao_expirada: "atencao",
}

// Estado calculado pelo servidor: o menos maduro entre o próprio (tarefas e última execução) e os de baixo.
export function EstadoDoRequisito({ estado, className }: { estado: EstadoDoRequisito; className?: string }) {
  return (
    <Badge variant="outline" className={cn("border-transparent", seloDoTom[tomDoEstadoDoRequisito[estado]], className)}>
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

const tomDaPrioridade: Record<string, Tom> = {
  alta: "perigo",
  media: "atencao",
  baixa: "neutro",
}

// Prioridade da triagem; sem ela, nada aparece.
export function PrioridadeDoRequisito({ prioridade, className }: { prioridade?: string; className?: string }) {
  if (!prioridade) return null
  return (
    <Badge variant="outline" className={cn(contornoDoTom[tomDaPrioridade[prioridade] ?? "neutro"], className)}>
      ↑ {nomesPrioridade[prioridade]}
    </Badge>
  )
}
