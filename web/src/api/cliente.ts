import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import type { components } from "./esquema"

export type Tarefa = components["schemas"]["Tarefa"]
export type Sessao = components["schemas"]["Sessao"]
export type Evento = components["schemas"]["Evento"]
export type StatusTarefa = components["schemas"]["StatusTarefa"]
export type StatusSessao = components["schemas"]["StatusSessao"]
export type NovaTarefa = components["schemas"]["NovaTarefa"]
export type AlteracaoTarefa = components["schemas"]["AlteracaoTarefa"]

// A API fica na mesma origem do frontend; em desenvolvimento, o Vite encaminha.
async function obter<T>(caminho: string, metodo = "GET", corpo?: unknown): Promise<T> {
  const resposta = await fetch(caminho, {
    method: metodo,
    headers: corpo === undefined ? undefined : { "Content-Type": "application/json" },
    body: corpo === undefined ? undefined : JSON.stringify(corpo),
  })
  if (!resposta.ok) {
    const corpo = await resposta.json().catch(() => null)
    throw new Error(corpo?.mensagem ?? `a API respondeu ${resposta.status}`)
  }
  return resposta.json()
}

export const useTarefas = () => useQuery({ queryKey: ["tarefas"], queryFn: () => obter<Tarefa[]>("/tarefas") })

// Telas de detalhe se atualizam sozinhas: o lançador e os agentes mudam o estado por fora.
const atualizacao = 5_000

export const useTarefa = (id: string) =>
  useQuery({ queryKey: ["tarefa", id], queryFn: () => obter<Tarefa>(`/tarefas/${id}`), refetchInterval: atualizacao })

export const useSessoesDaTarefa = (id: string) =>
  useQuery({
    queryKey: ["tarefa", id, "sessoes"],
    queryFn: () => obter<Sessao[]>(`/tarefas/${id}/sessoes`),
    refetchInterval: atualizacao,
  })

export const useSessoes = () =>
  useQuery({ queryKey: ["sessoes"], queryFn: () => obter<Sessao[]>("/sessoes"), refetchInterval: atualizacao })

export const useSessao = (id: string) =>
  useQuery({ queryKey: ["sessao", id], queryFn: () => obter<Sessao>(`/sessoes/${id}`), refetchInterval: atualizacao })

// Depois de qualquer escrita, tudo é recarregado: são poucas consultas e o estado se espalha.
function useEscrita<E, R>(executar: (entrada: E) => Promise<R>) {
  const cliente = useQueryClient()
  return useMutation({ mutationFn: executar, onSuccess: () => cliente.invalidateQueries() })
}

export const useAlterarTarefa = (id: string) =>
  useEscrita((alteracao: AlteracaoTarefa) => obter<Tarefa>(`/tarefas/${id}`, "PATCH", alteracao))

export const useCriarTarefa = () => useEscrita((nova: NovaTarefa) => obter<Tarefa>("/tarefas", "POST", nova))

export const useEncerrarSessao = (id: string) =>
  useEscrita((resumo: string) => obter<Sessao>(`/sessoes/${id}/encerramento-manual`, "POST", { resumo }))

export const useEventos = (entidade: string) =>
  useQuery({ queryKey: ["eventos", entidade], queryFn: () => obter<Evento[]>(`/eventos?entidade=${entidade}`) })
