import { Navigate, NavLink, Route, Routes } from "react-router-dom"

import { NovaTarefa } from "@/paginas/NovaTarefa"
import { NovoRequisito } from "@/paginas/NovoRequisito"
import { Origens } from "@/paginas/Origens"
import { Requisito } from "@/paginas/Requisito"
import { Requisitos } from "@/paginas/Requisitos"
import { Sessao } from "@/paginas/Sessao"
import { Sessoes } from "@/paginas/Sessoes"
import { Tarefa } from "@/paginas/Tarefa"
import { Tarefas } from "@/paginas/Tarefas"
import { cn } from "@/lib/utils"

const aba = ({ isActive }: { isActive: boolean }) =>
  cn("rounded-md px-3 py-1.5 text-sm", isActive ? "bg-muted font-medium" : "text-muted-foreground hover:text-foreground")

export default function App() {
  return (
    <div className="mx-auto max-w-4xl px-4 py-4">
      <nav className="mb-6 flex items-center gap-1 border-b pb-3">
        <span className="mr-3 font-semibold">requerial</span>
        <NavLink to="/requisitos" className={aba}>
          Requisitos
        </NavLink>
        <NavLink to="/tarefas" end className={aba}>
          Tarefas
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
        <Route path="/sessoes" element={<Sessoes />} />
        <Route path="/sessoes/:id" element={<Sessao />} />
      </Routes>
    </div>
  )
}
