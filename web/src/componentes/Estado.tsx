// Mensagens de carregamento e erro das consultas.
export function Estado({ carregando, erro }: { carregando: boolean; erro: Error | null }) {
  if (carregando) return <p className="text-muted-foreground text-sm">Carregando…</p>
  if (erro) return <p className="text-destructive text-sm">Erro: {erro.message}</p>
  return null
}
