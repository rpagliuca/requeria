import { Navigate, NavLink, Route, Routes } from "react-router-dom"

import { NovaTarefa } from "@/paginas/NovaTarefa"
import { NovoRequisito } from "@/paginas/NovoRequisito"
import { Origens } from "@/paginas/Origens"
import { Requisito } from "@/paginas/Requisito"
import { Requisitos } from "@/paginas/Requisitos"
import { Sessao } from "@/paginas/Sessao"
import { Sessoes } from "@/paginas/Sessoes"
import { Componente } from "@/paginas/Componente"
import { Componentes } from "@/paginas/Componentes"
import { Logotipo } from "@/componentes/Logotipo"
import { Lembretes } from "@/paginas/Lembretes"
import { Template } from "@/paginas/Template"
import { Templates } from "@/paginas/Templates"
import { useLembretes, useOrganizacoes, useTemplates } from "@/api/cliente"
import { useOrganizacaoAtual } from "@/hooks/useOrganizacaoAtual"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Tarefa } from "@/paginas/Tarefa"
import { Tarefas } from "@/paginas/Tarefas"
import { cn } from "@/lib/utils"

const aba = ({ isActive }: { isActive: boolean }) =>
  cn("rounded-md px-3 py-1.5 text-sm", isActive ? "bg-muted font-medium" : "text-muted-foreground hover:text-foreground")

export default function App() {
  const templates = useTemplates()
  const devidos = (templates.data ?? []).filter((t) => t.devido).length
  const tocando = (useLembretes().data ?? []).filter((l) => l.status === "disparado").length
  return (
    <div className="mx-auto max-w-4xl px-4 py-4">
      <nav className="mb-6 flex flex-wrap items-center gap-1 border-b pb-3">
        <NavLink to="/" className="mr-3 flex items-center gap-2 font-semibold tracking-tight" aria-label="Requerial">
          <Logotipo className="size-5" />
          Requerial
        </NavLink>
        <NavLink to="/requisitos" className={aba}>
          Requisitos
        </NavLink>
        <NavLink to="/tarefas" end className={aba}>
          Tarefas
        </NavLink>
        <NavLink to="/templates" className={aba}>
          Tarefas Pré-moldadas
          {devidos > 0 && (
            <span className="ml-1.5 rounded-full bg-tom-atencao px-1.5 text-tom-atencao-fg text-xs font-medium" title={`${devidos} devidas agora`}>
              {devidos}
            </span>
          )}
        </NavLink>
        <NavLink to="/lembretes" className={aba}>
          Lembretes
          {tocando > 0 && (
            <span className="ml-1.5 rounded-full bg-tom-perigo px-1.5 text-tom-perigo-fg text-xs font-medium" title={`${tocando} tocando agora`}>
              {tocando}
            </span>
          )}
        </NavLink>
        <NavLink to="/componentes" className={aba}>
          Componentes
        </NavLink>
        <NavLink to="/sessoes" className={aba}>
          Sessões
        </NavLink>
        <EscolherOrganizacao />
      </nav>
      <Routes>
        <Route path="/" element={<Navigate to="/requisitos" replace />} />
        <Route path="/tarefas" element={<Tarefas />} />
        <Route path="/tarefas/nova" element={<NovaTarefa />} />
        <Route path="/tarefas/:id" element={<Tarefa />} />
        <Route path="/requisitos" element={<Requisitos />} />
        <Route path="/requisitos/novo" element={<NovoRequisito />} />
        <Route path="/requisitos/:id" element={<Requisito />} />
        <Route path="/origens" element={<Origens />} />
        <Route path="/templates" element={<Templates />} />
        <Route path="/templates/:id" element={<Template />} />
        <Route path="/lembretes" element={<Lembretes />} />
        <Route path="/lembretes/:id" element={<Lembretes />} />
        <Route path="/componentes" element={<Componentes />} />
        <Route path="/componentes/:id" element={<Componente />} />
        <Route path="/sessoes" element={<Sessoes />} />
        <Route path="/sessoes/:id" element={<Sessao />} />
      </Routes>
    </div>
  )
}

// Escolha global: com uma organização, todas as telas mostram só o que é dela (bom para compartilhar a tela).
function EscolherOrganizacao() {
  const { organizacao, escolher } = useOrganizacaoAtual()
  const organizacoes = useOrganizacoes()
  const comZero = (organizacoes.data ?? []).filter((o) => o.zero || o.nome === organizacao)
  return (
    <Select value={organizacao || "_todas"} onValueChange={(v) => escolher(v === "_todas" ? "" : v)}>
      <SelectTrigger className={cn("ml-auto h-8 w-auto gap-2", organizacao && "border-primary font-medium")}>
        <SelectValue />
      </SelectTrigger>
      <SelectContent align="end">
        <SelectItem value="_todas">Todas as organizações</SelectItem>
        {(comZero.length > 0 ? comZero : organizacoes.data ?? []).map((o) => (
          <SelectItem key={o.nome} value={o.nome}>
            {o.nome}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}
