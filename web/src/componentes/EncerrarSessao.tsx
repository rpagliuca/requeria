import { useState } from "react"

import { useEncerrarSessao } from "@/api/cliente"
import { Button } from "@/components/ui/button"
import { Textarea } from "@/components/ui/textarea"

// Para quando o agente não está mais vivo para encerrar a própria sessão.
export function EncerrarSessao({ id }: { id: string }) {
  const [aberto, setAberto] = useState(false)
  const [resumo, setResumo] = useState("")
  const encerrar = useEncerrarSessao(id)
  if (!aberto) {
    return (
      <Button size="sm" variant="outline" onClick={() => setAberto(true)}>
        Encerrar sessão manualmente
      </Button>
    )
  }
  return (
    <form
      className="space-y-2"
      onSubmit={(e) => {
        e.preventDefault()
        encerrar.mutate(resumo, { onSuccess: () => setAberto(false) })
      }}
    >
      <p className="text-muted-foreground text-xs">
        Use só se o agente não estiver mais vivo; com ele vivo, mude o status da tarefa e ele encerra com o próprio resumo.
      </p>
      <Textarea
        placeholder="Resumo para retomar: onde parou e o próximo passo"
        value={resumo}
        onChange={(e) => setResumo(e.target.value)}
        required
      />
      <div className="flex gap-2">
        <Button size="sm" type="submit" disabled={encerrar.isPending || !resumo.trim()}>
          Encerrar
        </Button>
        <Button size="sm" type="button" variant="ghost" onClick={() => setAberto(false)}>
          Cancelar
        </Button>
      </div>
      {encerrar.error && <p className="text-destructive text-sm">{encerrar.error.message}</p>}
    </form>
  )
}
