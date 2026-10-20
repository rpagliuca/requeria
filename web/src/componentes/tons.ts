// Tons semânticos da identidade (docs/design.md): a interface é tinta sobre papel e só o estado tem cor.
// As classes ficam escritas por extenso para o Tailwind encontrá-las.
export type Tom = "neutro" | "andamento" | "atencao" | "perigo" | "validacao" | "sucesso"

// Selo cheio: fundo suave e texto do tom.
export const seloDoTom: Record<Tom, string> = {
  neutro: "bg-tom-neutro text-tom-neutro-fg",
  andamento: "bg-tom-andamento text-tom-andamento-fg",
  atencao: "bg-tom-atencao text-tom-atencao-fg",
  perigo: "bg-tom-perigo text-tom-perigo-fg",
  validacao: "bg-tom-validacao text-tom-validacao-fg",
  sucesso: "bg-tom-sucesso text-tom-sucesso-fg",
}

// Selo vazado: só borda e texto, para o que é secundário ao estado (prioridade).
export const contornoDoTom: Record<Tom, string> = {
  neutro: "border-tom-neutro-borda text-tom-neutro-fg",
  andamento: "border-tom-andamento-borda text-tom-andamento-fg",
  atencao: "border-tom-atencao-borda text-tom-atencao-fg",
  perigo: "border-tom-perigo-borda text-tom-perigo-fg",
  validacao: "border-tom-validacao-borda text-tom-validacao-fg",
  sucesso: "border-tom-sucesso-borda text-tom-sucesso-fg",
}

// Texto solto no tom (marcadores como "● S-0001" e avisos de atenção).
export const textoDoTom: Record<Tom, string> = {
  neutro: "text-tom-neutro-fg",
  andamento: "text-tom-andamento-fg",
  atencao: "text-tom-atencao-fg",
  perigo: "text-tom-perigo-fg",
  validacao: "text-tom-validacao-fg",
  sucesso: "text-tom-sucesso-fg",
}
