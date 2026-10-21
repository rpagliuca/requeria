import { useEffect, useState } from "react"

import { type Requisito, useDefinirPontos } from "@/api/cliente"
import { percentual } from "@/componentes/rotulos"
import { cn } from "@/lib/utils"

// Pontos de importância do requisito sob um pai, editáveis no lugar, com a fatia entre os irmãos.
// Os pontos só se comparam entre irmãos; vazio é "não triado" e conta como 1.
export function PontosSobPai({ requisito, pai, className }: { requisito: Requisito; pai: string; className?: string }) {
  const ligacao = requisito.importancia.find((i) => i.pai === pai)
  const definir = useDefinirPontos(requisito.id)
  const atual = ligacao?.pontos?.toString() ?? ""
  const [valor, setValor] = useState(atual)
  useEffect(() => setValor(atual), [atual])
  if (!ligacao) return null
  const gravar = () => {
    const texto = valor.trim()
    const pontos = texto === "" ? null : Number(texto)
    if (texto === atual || (pontos !== null && (!Number.isInteger(pontos) || pontos < 1))) {
      setValor(atual)
      return
    }
    definir.mutate({ pai, pontos })
  }
  return (
    <span className={cn("inline-flex items-center gap-1 text-muted-foreground text-xs", className)} title={definir.error?.message ?? "Pontos entre os irmãos; vazio: não triado (conta como 1)"}>
      <input
        type="number"
        min={1}
        inputMode="numeric"
        aria-label={`Pontos de ${requisito.id} sob ${pai}`}
        placeholder="?"
        className={cn(
          "h-6 w-11 rounded border bg-background px-1 text-right text-foreground tabular-nums",
          definir.error && "border-destructive",
          !ligacao.pontos && "border-dashed",
        )}
        value={valor}
        onChange={(e) => setValor(e.target.value)}
        onBlur={gravar}
        onKeyDown={(e) => e.key === "Enter" && e.currentTarget.blur()}
        onClick={(e) => e.stopPropagation()}
      />
      <span className="w-10 tabular-nums">{percentual(ligacao.fatia)}</span>
    </span>
  )
}
