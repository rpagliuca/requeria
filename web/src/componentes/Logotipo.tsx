// Logotipo "Derivação" (docs/design.md): um requisito derivando dois abaixo, um atendido (cheio) e um não (vazado).
// Traço em currentColor; o mesmo desenho está em public/favicon.svg.
export function Logotipo({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 32 32" className={className} aria-hidden="true" fill="none" stroke="currentColor" strokeWidth="2.6" strokeLinecap="round">
      <path d="M14 11.5 9.2 20.5M18 11.5l4.8 9" />
      <circle cx="16" cy="8" r="4" fill="currentColor" stroke="none" />
      <circle cx="8" cy="24" r="3.6" />
      <circle cx="24" cy="24" r="4" fill="currentColor" stroke="none" />
    </svg>
  )
}
