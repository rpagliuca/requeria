import { createContext, type ReactNode, useContext, useState } from "react"

// A organização escolhida na barra superior filtra todas as telas ("" = todas, visão geral). Fica
// lembrada neste navegador: ao compartilhar a tela, nada de outra organização aparece.
const Contexto = createContext<{ organizacao: string; escolher: (o: string) => void }>({ organizacao: "", escolher: () => {} })

export function OrganizacaoAtual({ children }: { children: ReactNode }) {
  const [organizacao, setOrganizacao] = useState(() => {
    try {
      return localStorage.getItem("organizacao") ?? ""
    } catch {
      return ""
    }
  })
  const escolher = (o: string) => {
    setOrganizacao(o)
    try {
      localStorage.setItem("organizacao", o)
    } catch {
      // sem armazenamento, a escolha vale só até recarregar
    }
  }
  return <Contexto.Provider value={{ organizacao, escolher }}>{children}</Contexto.Provider>
}

export const useOrganizacaoAtual = () => useContext(Contexto)
