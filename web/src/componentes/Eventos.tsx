import { useEventos } from "@/api/cliente"
import { Data } from "@/componentes/rotulos"
import { Estado } from "@/componentes/Estado"

// Log de eventos da entidade, do mais recente para o mais antigo.
export function Eventos({ entidade }: { entidade: string }) {
  const { data, isLoading, error } = useEventos(entidade)
  if (!data) return <Estado carregando={isLoading} erro={error} />
  if (data.length === 0) return <p className="text-muted-foreground text-sm">Nenhum evento.</p>
  return (
    <ol className="space-y-2 text-sm">
      {[...data].reverse().map((e) => (
        <li key={e.id} className="border-l-2 pl-3">
          <div className="flex flex-wrap gap-x-2 text-muted-foreground text-xs">
            <Data valor={e.momento} />
            <span>{e.autor}</span>
          </div>
          <div className="font-medium">{e.tipo.replace(/_/g, " ")}</div>
          {Object.keys(e.dados).length > 0 && (
            <pre className="text-muted-foreground text-xs whitespace-pre-wrap break-all">
              {JSON.stringify(e.dados)}
            </pre>
          )}
        </li>
      ))}
    </ol>
  )
}
