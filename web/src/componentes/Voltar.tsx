import { Link, useLocation, useNavigate } from "react-router-dom"

// Volta para a tela de onde se veio (o grafo, outra tarefa…); aberta direto por link, vai para a lista.
export function Voltar({ para, rotulo }: { para: string; rotulo: string }) {
  const navegar = useNavigate()
  const location = useLocation()
  if (location.key === "default")
    return (
      <Link to={para} className="text-muted-foreground text-sm hover:underline">
        ← {rotulo}
      </Link>
    )
  return (
    <button type="button" onClick={() => navegar(-1)} className="text-muted-foreground text-sm hover:underline">
      ← Voltar
    </button>
  )
}
