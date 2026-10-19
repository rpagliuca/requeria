---
name: requeria
description: "Como um agente de IA trabalha numa Tarefa do requeria pelas ferramentas MCP: abrir a Sessão de Trabalho, registrar o journal, encerrar com resumo para o próximo agente e lidar com recusas. Use ao começar, retomar ou encerrar trabalho numa tarefa do requeria, quando o usuário citar um identificador T-0001 ou S-0001, ou quando as ferramentas mcp__requeria__* estiverem disponíveis. Triggers: requeria, tarefa T-, sessão S-, iniciar_sessao, encerrar_sessao, repassar_sessao, resumo para retomar, journal da sessão, a tarefa já tem a sessão ativa."
user-invocable: true
---

# requeria: trabalhar numa Tarefa como agente

O requeria guarda **Tarefas** (o que fazer) e **Sessões de Trabalho** (quem trabalhou, quando e com
qual agente). Você, agente, só escreve nele pelas ferramentas MCP `mcp__requeria__*`. O servidor
impõe as regras; se ele recusar, a recusa é a resposta certa, não um obstáculo a contornar.

Se as ferramentas `mcp__requeria__*` não estiverem disponíveis, diga isso ao usuário e pare. Não
tente falar com a API por `curl` nem editar o banco.

## Conceitos em uma linha

- **Tarefa** (`T-0001`): tem status (`a_fazer`, `em_andamento`, `pausada`, `bloqueada`,
  `em_validacao`, `concluida`) e um **resumo para retomar**, escrito pela última sessão.
- **Sessão de Trabalho** (`S-0001`): este chat, com um **agente imutável**. Cada tarefa tem no
  máximo uma sessão ativa.
- **Journal**: o seu registro livre do trabalho, num arquivo do git. O requeria guarda só onde ele
  está.

## Ao começar

1. **Leia a tarefa**: `obter_tarefa(id)`. O campo `resumo` é o que a sessão anterior deixou para
   você; comece por ele. Se precisar de mais contexto, `historico_da_tarefa(id)` lista as sessões
   anteriores com o journal de cada uma.
2. **Abra a sessão**: `iniciar_sessao` com:
   - `modelo`: provider e modelo, exatamente como você é, sem o esforço: `Claude Opus 5.5`,
     `Claude Fable 5.1`. O modelo está no seu prompt de sistema.
   - `tarefas`: as tarefas em que vai trabalhar.
   - `journal`: o localizador do arquivo, `<maquina>:<caminho-absoluto>`, se já souber onde vai
     escrever.

   O esforço e o identificador do chat vêm do cliente do agente; não os informe. O nome registrado
   fica como `Claude Opus 5.5 Medium`. Só se a ferramenta disser que o esforço não foi fornecido,
   pergunte ao usuário e passe em `esforco`.
3. **Marque o andamento**, se fizer sentido: `alterar_tarefa(id, status: "em_andamento")`.

Uma sessão por chat. Se precisar de mais uma tarefa no meio do trabalho, use `vincular_tarefa`, não
uma sessão nova.

## Durante

- Escreva o journal no arquivo, como sempre: o que fez, o que decidiu e por quê, o que falhou.
- Ao criar ou mover o journal ou artefatos, atualize os localizadores com `atualizar_journal`.
- Toda alteração que você fizer pelas ferramentas fica atribuída à sua sessão no log de eventos.

## Ao terminar

Chame `encerrar_sessao` com o **resumo**. Ele é gravado em cada tarefa da sessão e é a primeira
coisa que o próximo agente lê. Escreva para quem não viu nada desta conversa:

- em que estado a tarefa ficou;
- o que falta, começando pelo próximo passo concreto;
- decisões e descobertas que o próximo não pode perder;
- onde está o detalhe (journal, PR, arquivo).

Curto e factual; o detalhe fica no journal. **Antes** de encerrar, ajuste o status da tarefa se mudou
(`em_validacao`, `concluida`, `bloqueada`...): depois do encerramento, a alteração não é mais
atribuída à sua sessão.

Persista o journal no git antes de encerrar e passe o localizador definitivo no próprio
`encerrar_sessao`: `github.com/<dono>/<repo>@<commit>:<caminho>`.

## Recusas e o que fazer

| Mensagem | Significa | Faça |
|---|---|---|
| "a tarefa T-… já tem a sessão ativa S-…" | Outro chat está trabalhando nela | Pare e avise o usuário. Nunca encerre a sessão de outro chat |
| "este chat já tem a sessão S-… aberta" | Você já iniciou | Use `vincular_tarefa` ou continue na mesma |
| "não há sessão aberta neste chat" | Faltou `iniciar_sessao` | Inicie antes de operar a sessão |
| "o resumo é obrigatório" | Encerramento sem resumo | Escreva o resumo |

## Troca de agente

Se o usuário trocar o modelo ou o esforço no meio do chat, você deixou de ser o agente registrado
na sessão. Chame `repassar_sessao` com o `modelo` e o `esforco` novos (aqui o esforço é obrigatório): a sessão atual termina como substituída e
outra começa com as mesmas tarefas. Não faça isso por iniciativa própria; só quando a troca
aconteceu.

## O que não fazer

- Não altere o resumo de uma tarefa com `alterar_tarefa` para "registrar progresso": o resumo é do
  encerramento.
- Não abra sessão só para ler; ler não exige sessão.
- Não mostre nem procure o token da sessão: ele fica no processo MCP e você não precisa dele.
