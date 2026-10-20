import { useOrganizacaoAtual } from "@/hooks/useOrganizacaoAtual"

// Página de um item que não é da organização escolhida: não mostra nada dele.
export function ForaDaOrganizacao() {
  const { organizacao, escolher } = useOrganizacaoAtual()
  return (
    <p className="text-muted-foreground text-sm">
      Não encontrado em {organizacao || "nenhuma organização"}.{" "}
      {organizacao && (
        <button type="button" className="underline" onClick={() => escolher("")}>
          Ver todas as organizações
        </button>
      )}
    </p>
  )
}
