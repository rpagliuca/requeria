import ReactMarkdown from "react-markdown"
import remarkGfm from "remark-gfm"

// Descrições e resumos vêm em Markdown (muitos importados de CLAUDE.md).
export function Texto({ children }: { children: string }) {
  if (!children.trim()) return <p className="text-muted-foreground text-sm">Vazio.</p>
  return (
    <div className="prose-requeria text-sm leading-relaxed break-words">
      <ReactMarkdown remarkPlugins={[remarkGfm]}>{children}</ReactMarkdown>
    </div>
  )
}
