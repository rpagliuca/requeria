import { useState } from "react"
import { Link } from "react-router-dom"

import { type Tarefa, useDesvincularRequisito, useRequisitos, useVincularRequisito } from "@/api/cliente"
import { Button } from "@/components/ui/button"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"

// Os requisitos que a tarefa tenta atender: o porquê do trabalho.
export function RequisitosDaTarefa({ tarefa }: { tarefa: Tarefa }) {
  const requisitos = useRequisitos()
  const vincular = useVincularRequisito(tarefa.id)
  const desvincular = useDesvincularRequisito(tarefa.id)
  const [escolhido, setEscolhido] = useState("")
  const ligados = tarefa.requisitos ?? []
  const porId = new Map((requisitos.data ?? []).map((r) => [r.id, r]))
  const candidatos = (requisitos.data ?? []).filter((r) => !ligados.includes(r.id) && !r.obsoleto)
  const erro = vincular.error ?? desvincular.error

  return (
    <div className="space-y-2">
      {ligados.length === 0 ? (
        <p className="text-muted-foreground text-sm">Nenhum requisito vinculado.</p>
      ) : (
        <ul className="divide-y rounded-md border">
          {ligados.map((rid) => (
            <li key={rid} className="flex items-center justify-between gap-2 px-3 py-2">
              {porId.has(rid) || !requisitos.data ? (
                <Link to={`/requisitos/${rid}`} className="hover:underline">
                  <span className="font-mono text-muted-foreground text-xs">{rid}</span> {porId.get(rid)?.titulo}
                  {porId.get(rid)?.obsoleto && <span className="ml-2 text-tom-atencao-fg text-xs">obsoleto</span>}
                </Link>
              ) : (
                <span className="text-muted-foreground text-sm">um requisito de outra organização</span>
              )}
              <Button size="sm" variant="ghost" className="h-7 text-xs" onClick={() => desvincular.mutate(rid)}>
                desvincular
              </Button>
            </li>
          ))}
        </ul>
      )}
      <div className="flex flex-wrap items-center gap-2">
        <Select value={escolhido} onValueChange={setEscolhido}>
          <SelectTrigger className="sm:w-80">
            <SelectValue placeholder="Vincular requisito…" />
          </SelectTrigger>
          <SelectContent>
            {candidatos.map((r) => (
              <SelectItem key={r.id} value={r.id}>
                {r.id} {r.titulo}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Button
          size="sm"
          variant="outline"
          disabled={!escolhido || vincular.isPending}
          onClick={() => vincular.mutate(escolhido, { onSuccess: () => setEscolhido("") })}
        >
          Vincular
        </Button>
      </div>
      {erro && <p className="text-destructive text-sm">{erro.message}</p>}
    </div>
  )
}
