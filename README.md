# requeria

Coordenação de humanos e agentes de IA orientada por requisitos vivos, com evidência contínua.

Desenho completo em [`docs/arquitetura.md`](docs/arquitetura.md).

## Estado atual

Prova de conceito de **Tarefas** e **Sessões de Trabalho**: API HTTP sobre SQLite, com o contrato em
[`api/openapi.yaml`](api/openapi.yaml) e as invariantes impostas pela API e pelo banco, mais um
servidor MCP para agentes de IA.

## Rodar

```bash
go build -o requeria ./cmd/requeria
./requeria servir -banco requeria.db -endereco 127.0.0.1:8787
```

## Usar com um agente (MCP)

O servidor MCP fala com a API e guarda, em memória, o token da sessão aberta no chat; o agente nunca
o vê. No Claude Code:

```bash
claude mcp add requeria -- /caminho/para/requeria mcp -api http://127.0.0.1:8787
```

Para os agentes saberem usar as ferramentas, instale a skill [`skills/requeria`](skills/requeria/SKILL.md):

```bash
ln -s "$PWD/skills/requeria" ~/.claude/skills/requeria
```

Ferramentas: `obter_tarefa`, `criar_tarefa`, `alterar_tarefa`,
`historico_da_tarefa`, `iniciar_sessao`, `sessao_atual`, `vincular_tarefa`, `atualizar_journal`,
`encerrar_sessao`, `repassar_sessao`, `listar_eventos`.

## Desenvolver

```bash
go generate ./api   # regenera api/api.gen.go a partir do contrato OpenAPI
go test ./...
```

Frontend (React + Vite + TypeScript + Tailwind + shadcn/ui), embutido no binário a partir de `web/dist`:

```bash
cd web
npm ci
npm run gerar   # tipos TypeScript a partir do contrato OpenAPI
npm run build   # compila para web/dist; depois recompile o binário Go
REQUERIA_API=http://127.0.0.1:8787 npm run dev   # desenvolvimento, com a API encaminhada
```

O frontend fica em `/` e navega pelo hash da URL (`/#/tarefas/T-0001`); os caminhos `/tarefas`,
`/sessoes`, `/eventos` e `/importacao` são da API.
