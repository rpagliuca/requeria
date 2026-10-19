import { ChevronDown } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"

export type Opcao = { valor: string; rotulo: string; contagem?: number }
export type Atalho = { rotulo: string; valores: string[] }

// Combobox de seleção múltipla: marca e desmarca opções; atalhos trocam a seleção inteira.
export function SelecaoMultipla({
  rotulo,
  opcoes,
  selecionados,
  alterar,
  atalhos = [],
}: {
  rotulo: string
  opcoes: Opcao[]
  selecionados: string[]
  alterar: (valores: string[]) => void
  atalhos?: Atalho[]
}) {
  const iguais = (a: string[]) => a.length === selecionados.length && a.every((v) => selecionados.includes(v))
  const resumo = (() => {
    const atalho = atalhos.find((a) => iguais(a.valores))
    if (atalho) return atalho.rotulo
    if (selecionados.length === opcoes.length) return "Todos"
    if (selecionados.length === 0) return "Nenhum"
    if (selecionados.length <= 2)
      return opcoes
        .filter((o) => selecionados.includes(o.valor))
        .map((o) => o.rotulo)
        .join(", ")
    return `${selecionados.length} selecionados`
  })()
  const alternar = (valor: string) =>
    alterar(selecionados.includes(valor) ? selecionados.filter((v) => v !== valor) : [...selecionados, valor])

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button variant="outline" className="justify-between font-normal sm:w-56">
          <span className="truncate">
            <span className="text-muted-foreground">{rotulo}:</span> {resumo}
          </span>
          <ChevronDown className="opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-64 p-1">
        {atalhos.length > 0 && (
          <div className="flex flex-wrap gap-1 border-b p-1 pb-2">
            {atalhos.map((a) => (
              <Button
                key={a.rotulo}
                size="sm"
                variant={iguais(a.valores) ? "default" : "ghost"}
                className="h-7"
                onClick={() => alterar(a.valores)}
              >
                {a.rotulo}
              </Button>
            ))}
          </div>
        )}
        <div className="max-h-72 overflow-y-auto py-1">
          {opcoes.map((o) => (
            <label
              key={o.valor}
              className="flex cursor-pointer items-center gap-2 rounded-sm px-2 py-1.5 text-sm hover:bg-muted"
            >
              <Checkbox checked={selecionados.includes(o.valor)} onCheckedChange={() => alternar(o.valor)} />
              <span className="flex-1">{o.rotulo}</span>
              {o.contagem !== undefined && <span className="text-muted-foreground text-xs">{o.contagem}</span>}
            </label>
          ))}
        </div>
      </PopoverContent>
    </Popover>
  )
}
