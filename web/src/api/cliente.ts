import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { useOrganizacaoAtual } from "@/hooks/useOrganizacaoAtual"

import type { components } from "./esquema"

export type Tarefa = components["schemas"]["Tarefa"]
export type Sessao = components["schemas"]["Sessao"]
export type Evento = components["schemas"]["Evento"]
export type StatusTarefa = components["schemas"]["StatusTarefa"]
export type StatusSessao = components["schemas"]["StatusSessao"]
export type NovaTarefa = components["schemas"]["NovaTarefa"]
export type AlteracaoTarefa = components["schemas"]["AlteracaoTarefa"]
export type Requisito = components["schemas"]["Requisito"]
export type Origem = components["schemas"]["Origem"]
export type NovoRequisito = components["schemas"]["NovoRequisito"]
export type AlteracaoRequisito = components["schemas"]["AlteracaoRequisito"]
export type EstadoDoRequisito = components["schemas"]["EstadoDoRequisito"]
export type Execucao = components["schemas"]["Execucao"]
export type NovaExecucao = components["schemas"]["NovaExecucao"]
export type Template = components["schemas"]["Template"]
export type NovoTemplate = components["schemas"]["NovoTemplate"]
export type AlteracaoTemplate = components["schemas"]["AlteracaoTemplate"]
export type Organizacao = components["schemas"]["Organizacao"]
export type NovaOrigem = components["schemas"]["NovaOrigem"]
export type PedidoDeLancamento = components["schemas"]["PedidoDeLancamento"]
export type OpcoesDeLancamento = components["schemas"]["OpcoesDeLancamento"]

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

// Com uma organização escolhida na barra superior, as listas só trazem o que é dela.
const daOrganizacao =
  <T extends { organizacao: string }>(org: string) =>
  (itens: T[]) =>
    org ? itens.filter((x) => x.organizacao === org) : itens

export const useTarefas = () => {
  const { organizacao } = useOrganizacaoAtual()
  return useQuery({ queryKey: ["tarefas"], queryFn: () => obter<Tarefa[]>("/tarefas"), select: daOrganizacao<Tarefa>(organizacao) })
}

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

// Uma sessão é da organização quando todas as tarefas dela são; sem tarefa, só na visão geral.
export const useSessoes = () => {
  const { organizacao } = useOrganizacaoAtual()
  const tarefas = useTarefas()
  const sessoes = useQuery({ queryKey: ["sessoes"], queryFn: () => obter<Sessao[]>("/sessoes"), refetchInterval: atualizacao })
  if (!organizacao) return sessoes
  const minhas = new Set((tarefas.data ?? []).map((t) => t.id))
  const data = sessoes.data?.filter((x) => x.tarefas.length > 0 && x.tarefas.every((t) => minhas.has(t)))
  return { ...sessoes, data: tarefas.data ? data : undefined }
}

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

// Sem lançador configurado, a API responde 404 e o frontend só muda o status.
export const useOpcoesDoLancador = () =>
  useQuery({
    queryKey: ["lancador"],
    queryFn: () => obter<OpcoesDeLancamento>("/lancador").catch(() => null),
    staleTime: Infinity,
  })

export const useLancarAgente = (id: string) =>
  useEscrita((pedido: PedidoDeLancamento) => obter<Tarefa>(`/tarefas/${id}/lancamento`, "POST", pedido))

export const useRequisitos = () => {
  const { organizacao } = useOrganizacaoAtual()
  return useQuery({
    queryKey: ["requisitos"],
    queryFn: () => obter<Requisito[]>("/requisitos"),
    refetchInterval: atualizacao,
    select: daOrganizacao<Requisito>(organizacao),
  })
}

export const useOrganizacoes = () =>
  useQuery({ queryKey: ["organizacoes"], queryFn: () => obter<Organizacao[]>("/organizacoes") })

export const useDefinirZero = (nome: string) =>
  useEscrita((zero: string) => obter<Organizacao>(`/organizacoes/${encodeURIComponent(nome)}`, "PUT", { zero }))

// O zero da visão atual: o da organização escolhida ou, na visão geral, o único sem pais.
export const useZero = () => {
  const { organizacao } = useOrganizacaoAtual()
  const organizacoes = useOrganizacoes()
  const requisitos = useRequisitos()
  if (organizacao) return organizacoes.data?.find((o) => o.nome === organizacao)?.zero
  return requisitos.data?.find((r) => r.deriva_de.length === 0)?.id
}

export const useOrigens = () => useQuery({ queryKey: ["origens"], queryFn: () => obter<Origem[]>("/origens") })

export const useCriarRequisito = () =>
  useEscrita((novo: NovoRequisito) => obter<Requisito>("/requisitos", "POST", novo))

export const useAlterarRequisito = (id: string) =>
  useEscrita((alteracao: AlteracaoRequisito) => obter<Requisito>(`/requisitos/${id}`, "PATCH", alteracao))

export const useDerivar = (id: string) =>
  useEscrita((pai: string) => obter<Requisito>(`/requisitos/${id}/derivacoes`, "POST", { pai }))

export const useDesfazerDerivacao = (id: string) =>
  useEscrita((pai: string) => obter<Requisito>(`/requisitos/${id}/derivacoes/${pai}`, "DELETE"))

export const useExecucoes = (id: string) =>
  useQuery({ queryKey: ["execucoes", id], queryFn: () => obter<Execucao[]>(`/requisitos/${id}/execucoes`) })

export const useRegistrarExecucao = (id: string) =>
  useEscrita((nova: NovaExecucao) => obter<Execucao>(`/requisitos/${id}/execucoes`, "POST", nova))

export const useCriarOrigem = () => useEscrita((nova: NovaOrigem) => obter<Origem>("/origens", "POST", nova))

export const useVincularRequisito = (tarefa: string) =>
  useEscrita((requisito: string) => obter<Tarefa>(`/tarefas/${tarefa}/requisitos`, "POST", { requisito }))

export const useDesvincularRequisito = (tarefa: string) =>
  useEscrita((requisito: string) => obter<Tarefa>(`/tarefas/${tarefa}/requisitos/${requisito}`, "DELETE"))

export const useTemplates = () => {
  const { organizacao } = useOrganizacaoAtual()
  return useQuery({
    queryKey: ["templates"],
    queryFn: () => obter<Template[]>("/templates"),
    refetchInterval: atualizacao,
    select: daOrganizacao<Template>(organizacao),
  })
}

export const useCriarTemplate = () => useEscrita((novo: NovoTemplate) => obter<Template>("/templates", "POST", novo))

export const useAlterarTemplate = (id: string) =>
  useEscrita((alteracao: AlteracaoTemplate) => obter<Template>(`/templates/${id}`, "PATCH", alteracao))

export const useInstanciarTemplate = (id: string) =>
  useEscrita((titulo?: string) => obter<Tarefa>(`/templates/${id}/instancias`, "POST", { titulo }))
