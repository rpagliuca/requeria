import { Navigate, NavLink, Route, Routes } from "react-router-dom"

import { NovaTarefa } from "@/paginas/NovaTarefa"
import { NovoRequisito } from "@/paginas/NovoRequisito"
import { Origens } from "@/paginas/Origens"
import { Requisito } from "@/paginas/Requisito"
import { Requisitos } from "@/paginas/Requisitos"
import { Sessao } from "@/paginas/Sessao"
import { Sessoes } from "@/paginas/Sessoes"
import { Template } from "@/paginas/Template"
import { Templates } from "@/paginas/Templates"
import { useTemplates } from "@/api/cliente"
import { Tarefa } from "@/paginas/Tarefa"
import { Tarefas } from "@/paginas/Tarefas"
import { cn } from "@/lib/utils"

const aba = ({ isActive }: { isActive: boolean }) =>
  cn("rounded-md px-3 py-1.5 text-sm", isActive ? "bg-muted font-medium" : "text-muted-foreground hover:text-foreground")

export default function App() {
  const templates = useTemplates()
  const devidos = (templates.data ?? []).filter((t) => t.devido).length
  return (
    <div className="mx-auto max-w-4xl px-4 py-4">
      <nav className="mb-6 flex flex-wrap items-center gap-1 border-b pb-3">
        <span className="mr-3 font-semibold">Requerial</span>
        <NavLink to="/requisitos" className={aba}>
          Requisitos
        </NavLink>
        <NavLink to="/tarefas" end className={aba}>
          Tarefas
        </NavLink>
        <NavLink to="/templates" className={aba}>
          Templates
          {devidos > 0 && (
            <span className="ml-1.5 rounded-full bg-amber-500 px-1.5 text-white text-xs" title={`${devidos} devidos agora`}>
              {devidos}
            </span>
          )}
        </NavLink>
        <NavLink to="/sessoes" className={aba}>
          Sessões
        </NavLink>
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
        <Route path="/sessoes" element={<Sessoes />} />
        <Route path="/sessoes/:id" element={<Sessao />} />
      </Routes>
    </div>
  )
}
