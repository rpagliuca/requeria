import { NavLink, Route, Routes } from "react-router-dom"

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
        <NavLink to="/" end className={aba}>
          Tarefas
        </NavLink>
        <NavLink to="/sessoes" className={aba}>
          Sessões
        </NavLink>
      </nav>
      <Routes>
        <Route path="/" element={<Tarefas />} />
        <Route path="/tarefas/:id" element={<Tarefa />} />
        <Route path="/sessoes" element={<Sessoes />} />
        <Route path="/sessoes/:id" element={<Sessao />} />
      </Routes>
    </div>
  )
}
