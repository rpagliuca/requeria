# requerial

Coordenação de humanos e agentes de IA orientada por requisitos vivos, com evidência contínua.

Desenho completo em [`docs/arquitetura.md`](docs/arquitetura.md).

## Estado atual

Prova de conceito de **Tarefas** e **Sessões de Trabalho**: API HTTP sobre SQLite, com o contrato em
[`api/openapi.yaml`](api/openapi.yaml) e as invariantes impostas pela API e pelo banco, mais um
servidor MCP para agentes de IA.

## Rodar

```bash
go build -o requerial ./cmd/requerial
./requerial servir -banco requerial.db -endereco 127.0.0.1:8787
```

## Usar com um agente (MCP)

O servidor MCP fala com a API e guarda, em memória, o token da sessão aberta no chat; o agente nunca
o vê. No Claude Code:

```bash
claude mcp add requerial -- /caminho/para/requerial mcp -api http://127.0.0.1:8787
```

Para os agentes saberem usar as ferramentas, instale as skills de [`skills/`](skills):
[`requerial`](skills/requerial/SKILL.md) (o agente trabalhando numa tarefa) e
[`requerial-coordenacao`](skills/requerial-coordenacao/SKILL.md) (criar tarefas, subir, pausar e
retomar agentes):

```bash
for skill in skills/*/; do ln -sfn "$PWD/$skill" ~/.claude/skills/"$(basename "$skill")"; done
```

Ferramentas: `obter_tarefa`, `criar_tarefa`, `alterar_tarefa`,
`historico_da_tarefa`, `iniciar_sessao`, `sessao_atual`, `vincular_tarefa`, `atualizar_journal`,
`encerrar_sessao`, `repassar_sessao`, `listar_eventos`.

## Lançador de agentes (opcional)

Com `requerial servir -lancador lancador.json`, uma tarefa que entra em `em_andamento` (pelo frontend
ou por quem não é uma sessão) ganha um agente numa sessão tmux; uma tarefa que sai de `em_andamento`
por mão de outro faz o agente vivo encerrar a sessão com resumo. Depois de encerrar, o próprio
agente fecha o seu tmux (skill `requerial`).

```json
{
  "diretorio": "/caminho/dos/journals",
  "comando": ["claude", "--remote-control", "{nome}"],
  "modelos": [
    {"nome": "Modelo A", "argumentos": ["--model", "modelo-a"]},
    {"nome": "Modelo B", "argumentos": ["--model", "modelo-b"]}
  ],
  "esforcos": ["low", "medium", "high"],
  "argumentos_esforco": ["--effort", "{esforco}"],
  "esforco_padrao": "high"
}
```

No frontend, pôr uma tarefa em andamento abre a escolha de modelo, esforço e uma instrução adicional
de reabertura (`POST /tarefas/{id}/lancamento`); o primeiro modelo é o padrão.

`prompt_iniciar` e `prompt_encerrar` são opcionais e aceitam `{tarefa}`, `{titulo}` e `{status}`.
No systemd, use `KillMode=process` para reiniciar o servidor sem derrubar o tmux dos agentes.

O MCP, com `requerial mcp -journals /caminho/dos/journals`, cria a pasta de cada sessão
(`S-0001/journal.md`) ao iniciá-la e registra o localizador.

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
REQUERIAL_API=http://127.0.0.1:8787 npm run dev   # desenvolvimento, com a API encaminhada
```

O frontend fica em `/` e navega pelo hash da URL (`/#/tarefas/T-0001`); os caminhos `/tarefas`,
`/sessoes`, `/eventos`, `/importacao` e `/lancador` são da API.
