import { useState } from "react"

import { type OpcoesDeLancamento, type Tarefa, useLancarAgente } from "@/api/cliente"
import { Button } from "@/components/ui/button"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Textarea } from "@/components/ui/textarea"

const PADRAO = "padrao"

// Reabrir com agente: o contexto ele lê da tarefa e das sessões anteriores; aqui vai só o que é novo.
export function LancarAgente({
  tarefa,
  opcoes,
  aoTerminar,
}: {
  tarefa: Tarefa
  opcoes: OpcoesDeLancamento
  aoTerminar: () => void
}) {
  const lancar = useLancarAgente(tarefa.id)
  const [instrucao, setInstrucao] = useState("")
  const [modelo, setModelo] = useState(opcoes.modelo_padrao)
  const [esforco, setEsforco] = useState(opcoes.esforco_padrao ?? PADRAO)

  return (
    <form
      className="space-y-3 rounded-md border p-3"
      onSubmit={(e) => {
        e.preventDefault()
        lancar.mutate(
          { instrucao: instrucao.trim() || undefined, modelo, esforco: esforco === PADRAO ? undefined : esforco },
          { onSuccess: aoTerminar },
        )
      }}
    >
      <p className="text-sm">A tarefa vai para <strong>em andamento</strong> e um agente sobe para trabalhar nela.</p>
      <div className="space-y-1.5">
        <Label htmlFor="instrucao">Instrução adicional de reabertura (opcional)</Label>
        <Textarea
          id="instrucao"
          rows={4}
          placeholder="O que mudou ou o que fazer agora. O resto o agente lê da tarefa e das sessões anteriores."
          value={instrucao}
          onChange={(e) => setInstrucao(e.target.value)}
        />
      </div>
      <div className="flex flex-wrap gap-3">
        <div className="space-y-1.5">
          <Label>Modelo</Label>
          <Select value={modelo} onValueChange={setModelo}>
            <SelectTrigger className="w-52">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {opcoes.modelos.map((m) => (
                <SelectItem key={m} value={m}>
                  {m}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-1.5">
          <Label>Esforço</Label>
          <Select value={esforco} onValueChange={setEsforco}>
            <SelectTrigger className="w-40">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {!opcoes.esforco_padrao && <SelectItem value={PADRAO}>Padrão do agente</SelectItem>}
              {(opcoes.esforcos ?? []).map((e) => (
                <SelectItem key={e} value={e}>
                  {e}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>
      <div className="flex gap-2">
        <Button size="sm" type="submit" disabled={lancar.isPending}>
          Subir agente
        </Button>
        <Button size="sm" type="button" variant="ghost" onClick={aoTerminar}>
          Cancelar
        </Button>
      </div>
      {lancar.error && <p className="text-destructive text-sm">{lancar.error.message}</p>}
    </form>
  )
}
