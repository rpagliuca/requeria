---
name: requerial
description: "Ciclo de trabalho de um agente de IA no requerial pelas ferramentas MCP: abrir a Sessão de Trabalho, escrever o journal na pasta da sessão, mudar o status da Tarefa, encerrar com resumo para o próximo agente e atender ao pedido de encerramento do lançador. Use ao começar, retomar ou encerrar trabalho numa tarefa do requerial, quando o prompt citar uma tarefa T-0001 ou sessão S-0001, quando REQUERIAL_TAREFA estiver no ambiente, quando o usuário pedir para este chat virar (se promover a) uma tarefa, ou quando as ferramentas mcp__requerial__* estiverem disponíveis. Triggers: requerial, tarefa T-, sessão S-, iniciar_sessao, encerrar_sessao, repassar_sessao, resumo para retomar, journal da sessão, a tarefa foi movida, a tarefa já tem a sessão ativa, se promover a tarefa."
user-invocable: true
---

# requerial: o ciclo de um agente

O requerial é a fonte única de **Tarefas** (o que fazer, com status) e **Sessões de Trabalho**
(quem trabalhou, quando, com qual agente e onde está o journal). Você só escreve nele pelas
ferramentas MCP `mcp__requerial__*`. O servidor impõe as regras; uma recusa é a resposta certa, não
um obstáculo a contornar.

Se as ferramentas não estiverem disponíveis, diga isso ao usuário e pare. Não fale com a API por
`curl` nem edite o banco.

## Conceitos em uma linha

- **Tarefa** (`T-0001`): status `a_fazer`, `em_andamento`, `pausada`, `bloqueada`, `em_validacao` ou
  `concluida`, e um **resumo para retomar**, escrito no encerramento da última sessão.
- **Sessão de Trabalho** (`S-0001`): este chat, com um **agente imutável**. Uma tarefa tem no máximo
  uma sessão ativa; várias sessões, em máquinas diferentes, trabalham nela ao longo do tempo.
- **Journal**: o seu registro do trabalho, num arquivo da **pasta da sessão**, criada pelo
  `iniciar_sessao`. Artefatos da sessão ficam na mesma pasta.
- **Lançador**: quando alguém põe uma tarefa em andamento, ele sobe um agente para ela (talvez você)
  com `REQUERIAL_TAREFA` no ambiente; quando alguém a tira de andamento, ele pede ao agente que
  encerre.

## Ao começar

Há dois caminhos.

**Você foi lançado para uma tarefa** (o prompt cita a tarefa, `REQUERIAL_TAREFA` está no ambiente):

1. `obter_tarefa(id)`: leia o **resumo** primeiro; é o que a sessão anterior deixou para você.
2. `historico_da_tarefa(id)`: as sessões anteriores, com o journal de cada uma. Leia os journals que
   o resumo não cobrir.
3. `iniciar_sessao` só com `modelo` (provider e modelo, sem o esforço: `Claude Opus 5.5`). A tarefa
   vem do ambiente; o esforço e o chat vêm do cliente do agente. Se a ferramenta disser que o esforço
   não foi fornecido, pergunte ao usuário e passe em `esforco`.
4. A resposta traz o `journal`: `<máquina>:<caminho>/S-0001/journal.md`. Escreva nele.

**O usuário pediu para este chat virar uma tarefa** (ou começou a trabalhar numa sem o lançador):

1. `iniciar_sessao` com `modelo` e sem tarefas.
2. Se a tarefa já existe, o usuário diz o id; senão, `criar_tarefa` (título, organização, descrição,
   `status: "em_andamento"`). Uma tarefa criada por uma sessão não aciona o lançador.
3. `vincular_tarefa` com o id. Avise o usuário em uma linha qual tarefa foi criada ou vinculada.

Uma sessão por chat. Para mais uma tarefa no mesmo trabalho, `vincular_tarefa`, não outra sessão.

## Durante

- Escreva o journal: o que fez, o que decidiu e por quê, o que falhou, onde estão os artefatos.
- Ao criar artefatos fora da pasta da sessão, registre-os com `atualizar_journal`.
- Toda alteração que você fizer pelas ferramentas fica atribuída à sua sessão no log de eventos.

## Ao terminar o trabalho

1. **Primeiro** ajuste o status da tarefa: `em_validacao` (pronto, falta o usuário validar),
   `concluida`, `bloqueada` (dependência externa; diga qual no resumo) ou `pausada`.
2. **Persista o journal**, se a pasta da sessão estiver num repositório git: commit só da pasta da
   sessão e push (cada sessão tem a sua pasta, então não há conflito; se o push for recusado,
   `git pull --rebase` e push de novo). Commits sem assinatura, se a máquina não tiver como assinar.
3. **Depois** `encerrar_sessao` com o **resumo**. Ele vai para cada tarefa da sessão e é a primeira
   coisa que o próximo agente lê. Escreva para quem não viu nada desta conversa:
   - em que estado a tarefa ficou;
   - o que falta, começando pelo próximo passo concreto;
   - decisões e descobertas que o próximo não pode perder;
   - onde está o detalhe (journal, PR, arquivo).

4. Escreva a mensagem final ao usuário (o que ficou feito, o que falta) e, **como última ação**, se
   você roda numa sessão tmux (`$TMUX` preenchido), feche-a:
   `tmux kill-session -t "$(tmux display-message -p '#S')"`. Nada depois disso. Fora do tmux (um chat
   comum), não feche nada.

## Quando o lançador pede para encerrar

A mensagem diz que a tarefa foi movida para outro status por outra pessoa. **Não altere o status**:
quem moveu já decidiu. Pare o que estiver fazendo, registre no journal onde parou, chame
`encerrar_sessao` com o resumo para retomar e persista o journal e feche o seu tmux como nos passos 2 e 4 acima.

## Recusas e o que fazer

| Mensagem | Significa | Faça |
|---|---|---|
| "a tarefa T-… já tem a sessão ativa S-…" | Outro chat está trabalhando nela | Pare e avise o usuário. Nunca encerre a sessão de outro chat |
| "este chat já tem a sessão S-… aberta" | Você já iniciou | Use `vincular_tarefa` ou continue na mesma |
| "não há sessão aberta neste chat" | Faltou `iniciar_sessao` | Inicie antes de operar a sessão |
| "o resumo é obrigatório" | Encerramento sem resumo | Escreva o resumo |

## Troca de agente

Se o usuário trocar o modelo ou o esforço no meio do chat, você deixou de ser o agente registrado.
Chame `repassar_sessao` com o `modelo` e o `esforco` novos (aqui o esforço é obrigatório): a sessão
atual termina como substituída e outra começa com as mesmas tarefas. Só quando a troca aconteceu.

## O que não fazer

- Não altere o resumo com `alterar_tarefa` para registrar progresso: o resumo é do encerramento.
- Não abra sessão só para ler; ler não exige sessão.
- Não mantenha status de tarefa em outro lugar (pastas, listas, arquivos): o status é o do requerial.
- Não mostre nem procure o token da sessão: ele fica no processo MCP.
- Nunca feche o tmux de outro agente; só o seu, e só depois de encerrar a sua sessão.
- Para criar tarefas para outros agentes, pausar ou mover tarefas que não são suas: skill
  `requerial-coordenacao`.
