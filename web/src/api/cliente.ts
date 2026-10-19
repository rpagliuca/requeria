import { useQuery } from "@tanstack/react-query"

import type { components } from "./esquema"

export type Tarefa = components["schemas"]["Tarefa"]
export type Sessao = components["schemas"]["Sessao"]
export type Evento = components["schemas"]["Evento"]
export type StatusTarefa = components["schemas"]["StatusTarefa"]
export type StatusSessao = components["schemas"]["StatusSessao"]

// A API fica na mesma origem do frontend; em desenvolvimento, o Vite encaminha.
async function obter<T>(caminho: string): Promise<T> {
  const resposta = await fetch(caminho)
  if (!resposta.ok) {
    const corpo = await resposta.json().catch(() => null)
    throw new Error(corpo?.mensagem ?? `a API respondeu ${resposta.status}`)
  }
  return resposta.json()
}

export const useTarefas = () => useQuery({ queryKey: ["tarefas"], queryFn: () => obter<Tarefa[]>("/tarefas") })

export const useTarefa = (id: string) =>
  useQuery({ queryKey: ["tarefa", id], queryFn: () => obter<Tarefa>(`/tarefas/${id}`) })

export const useSessoesDaTarefa = (id: string) =>
  useQuery({ queryKey: ["tarefa", id, "sessoes"], queryFn: () => obter<Sessao[]>(`/tarefas/${id}/sessoes`) })

export const useSessoes = () => useQuery({ queryKey: ["sessoes"], queryFn: () => obter<Sessao[]>("/sessoes") })

export const useSessao = (id: string) =>
  useQuery({ queryKey: ["sessao", id], queryFn: () => obter<Sessao>(`/sessoes/${id}`) })

export const useEventos = (entidade: string) =>
  useQuery({ queryKey: ["eventos", entidade], queryFn: () => obter<Evento[]>(`/eventos?entidade=${entidade}`) })
