import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { StrictMode } from "react"
import { createRoot } from "react-dom/client"
import { HashRouter } from "react-router-dom"

import App from "./App.tsx"
import { OrganizacaoAtual } from "./hooks/useOrganizacaoAtual"
import "./index.css"

// Tema do sistema operacional.
const escuro = window.matchMedia("(prefers-color-scheme: dark)")
const aplicarTema = () => document.documentElement.classList.toggle("dark", escuro.matches)
aplicarTema()
escuro.addEventListener("change", aplicarTema)

// Rotas no hash: os caminhos /tarefas e /sessoes do servidor são da API.
const cliente = new QueryClient({ defaultOptions: { queries: { staleTime: 30_000 } } })

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <QueryClientProvider client={cliente}>
      <HashRouter>
        <OrganizacaoAtual>
          <App />
        </OrganizacaoAtual>
      </HashRouter>
    </QueryClientProvider>
  </StrictMode>,
)
