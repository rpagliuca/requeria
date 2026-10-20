import { X } from "lucide-react"
import { useState } from "react"
import { Link } from "react-router-dom"

import { useComponentes, useCriarComponente, useDesvincularComponente, useVincularComponente } from "@/api/cliente"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"

// Componentes de um requisito ou de uma tarefa: ligar um existente, criar um novo já ligado, desligar.
export function ComponentesLigados({
  alvo,
  id,
  ligados,
  organizacao,
}: {
  alvo: "requisitos" | "tarefas"
  id: string
  ligados: string[]
  organizacao: string
}) {
  const componentes = useComponentes()
  const vincular = useVincularComponente(alvo, id)
  const desvincular = useDesvincularComponente(alvo, id)
  const criar = useCriarComponente()
  const [escolhido, setEscolhido] = useState("")
  const [novo, setNovo] = useState("")
  const porId = new Map((componentes.data ?? []).map((c) => [c.id, c]))
  const candidatos = (componentes.data ?? []).filter((c) => !ligados.includes(c.id))
  const erro = vincular.error ?? desvincular.error ?? criar.error

  return (
    <div className="space-y-2">
      <div className="flex flex-wrap gap-1.5">
        {ligados.length === 0 && <span className="text-muted-foreground text-sm">Nenhum.</span>}
        {ligados.map((cid) => (
          <span key={cid} className="inline-flex items-center gap-1 rounded-full border px-2.5 py-0.5 text-sm">
            {porId.has(cid) || !componentes.data ? (
              <Link to={`/componentes/${cid}`} className="hover:underline">
                {porId.get(cid)?.nome ?? cid}
              </Link>
            ) : (
              <span className="text-muted-foreground">de outra organização</span>
            )}
            <button type="button" aria-label="desligar" className="text-muted-foreground hover:text-foreground" onClick={() => desvincular.mutate(cid)}>
              <X className="size-3.5" />
            </button>
          </span>
        ))}
      </div>
      <div className="flex flex-wrap items-center gap-2">
        <Select value={escolhido} onValueChange={(v) => vincular.mutate(v, { onSuccess: () => setEscolhido("") })}>
          <SelectTrigger className="h-8 sm:w-64">
            <SelectValue placeholder="Ligar componente…" />
          </SelectTrigger>
          <SelectContent>
            {candidatos.map((c) => (
              <SelectItem key={c.id} value={c.id}>
                {c.nome}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <form
          className="flex gap-2"
          onSubmit={(e) => {
            e.preventDefault()
            if (!novo.trim()) return
            criar.mutate({ nome: novo.trim(), organizacoes: [organizacao] }, { onSuccess: (c) => vincular.mutate(c.id, { onSuccess: () => setNovo("") }) })
          }}
        >
          <Input className="h-8 sm:w-48" placeholder="ou criar novo…" value={novo} onChange={(e) => setNovo(e.target.value)} />
          <Button type="submit" size="sm" variant="outline" className="h-8" disabled={!novo.trim() || criar.isPending}>
            Criar e ligar
          </Button>
        </form>
      </div>
      {erro && <p className="text-destructive text-sm">{erro.message}</p>}
    </div>
  )
}
