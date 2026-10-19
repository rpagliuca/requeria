import { useEffect } from "react"
import { useSearchParams } from "react-router-dom"

// Os filtros vivem na URL; esta lista também os lembra, para que voltar a ela (pelo menu, pelo
// "← Tarefas" ou por outro link sem parâmetros) reabra a mesma visão.
export function useFiltrosLembrados(chave: string) {
  const [params, setParams] = useSearchParams()
  const atual = params.toString()

  useEffect(() => {
    if (atual) {
      gravar(chave, atual)
      return
    }
    const lembrado = ler(chave)
    if (lembrado) setParams(new URLSearchParams(lembrado), { replace: true })
  }, [chave, atual, setParams])

  // Limpar tudo explicitamente também precisa ser lembrado.
  const alterar = (novos: URLSearchParams, opcoes?: { replace?: boolean }) => {
    if (!novos.toString()) gravar(chave, "")
    setParams(novos, opcoes)
  }
  return [params, alterar] as const
}

function ler(chave: string): string {
  try {
    return localStorage.getItem(`filtros:${chave}`) ?? ""
  } catch {
    return ""
  }
}

function gravar(chave: string, valor: string) {
  try {
    localStorage.setItem(`filtros:${chave}`, valor)
  } catch {
    // sem armazenamento, os filtros só não são lembrados
  }
}
