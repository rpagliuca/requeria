---
name: requerial
description: "Ciclo de trabalho de um agente de IA no Requerial pelas ferramentas MCP: toda sessão de trabalho é registrada antes da primeira ação que muda estado; abrir ou adotar a Sessão de Trabalho, achar ou criar a Tarefa, escrever o journal, mudar o status, encerrar com resumo e atender ao pedido de encerramento do lançador. Use SEMPRE no início de qualquer trabalho num chat em que as ferramentas mcp__requerial__* estejam disponíveis — investigar, alterar, escrever código, documentar, operar sistemas —, qualquer que seja a origem do chat. Use também ao retomar ou encerrar trabalho, quando o prompt citar uma tarefa T-0001 ou sessão S-0001, quando REQUERIAL_TAREFA ou REQUERIAL_SESSAO estiver no ambiente, ou quando pedirem para este chat virar (se promover a) uma tarefa. Triggers: começar a trabalhar, continue a partir do CLAUDE.md, retomar, Requerial, tarefa T-, sessão S-, iniciar_sessao, sessao_atual, encerrar_sessao, repassar_sessao, resumo para retomar, journal da sessão, a tarefa foi movida, a tarefa já tem a sessão ativa, se promover a tarefa."
user-invocable: true
---

# Requerial: o ciclo de um agente

O Requerial é a fonte única de **Tarefas** (o que fazer, com status) e **Sessões de Trabalho**
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

**Toda sessão de trabalho é registrada, antes da primeira ação que muda estado**, qualquer que seja
a origem do chat. Só fica de fora a pergunta rápida que não muda nada; na dúvida, registre.
Primeiro, `sessao_atual`: se o lançador já abriu a sua sessão, siga o primeiro caminho abaixo.

Há dois caminhos.

**Você foi lançado para uma tarefa** (o prompt cita a tarefa; `REQUERIAL_SESSAO` está no ambiente):

1. A sua sessão de trabalho **já está aberta**: o lançador a abriu com o seu modelo e esforço.
   `sessao_atual` mostra qual é e o `journal` (`<máquina>:<caminho>/S-0001/journal.md`), já criado.
   Escreva nele. (`iniciar_sessao`, se chamado, devolve essa mesma sessão.)
2. `obter_tarefa(id)`: leia o **resumo** primeiro; é o que a sessão anterior deixou para você.
3. `historico_da_tarefa(id)`: as sessões anteriores, com o journal de cada uma. Leia os journals que
   o resumo não cobrir.
4. Se o prompt trouxer uma **instrução adicional de reabertura**, ela é o ponto de partida; o resto
   do contexto vem da tarefa e das sessões anteriores.

Nada que mude estado (comandos, PRs, consultas que alterem algo) antes de conferir a sessão.

**O usuário pediu para este chat virar uma tarefa** (ou começou a trabalhar numa sem o lançador):

1. `iniciar_sessao` com `modelo` e sem tarefas.
2. Se a tarefa já existe, o usuário diz o id; senão, `criar_tarefa` (título, organização, descrição,
   `status: "em_andamento"`, e já os `requisitos` e `componentes`; ver "Ligar ao porquê" abaixo).
   Uma tarefa criada por uma sessão não aciona o lançador.
3. `vincular_tarefa` com o id. Avise o usuário em uma linha qual tarefa foi criada ou vinculada.

Uma sessão por chat. Para mais uma tarefa no mesmo trabalho, `vincular_tarefa`, não outra sessão.

Se a tarefa tem `requisitos`, leia-os com `obter_requisito`: são o **porquê** do trabalho, e as
`trilhas` mostram a cadeia até o zero em uma linha cada (de onde vem cada exigência).

## Ligar ao porquê (barato)

Toda tarefa tenta atender algum requisito e mexe em algum componente. Para achá-los gastando pouco:

1. `buscar_requisitos` com 2–4 palavras do assunto (e `organizacao`): uma chamada, só id e título.
   Tente outras palavras antes de concluir que não existe.
2. `obter_requisito` no candidato, se precisar confirmar: as `trilhas` dão o contexto.
3. `listar_componentes` para reaproveitar um componente existente.
4. Só para criar um requisito novo, `arvore_de_requisitos` (da organização, ou do componente) para
   escolher o pai. Requisito é afirmação testável; no máximo 5 derivados por pai.

Na dúvida, não ligue: tarefa sem requisito aparece para o Rafael na triagem. Ligar errado é pior.

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
- Não mantenha status de tarefa em outro lugar (pastas, listas, arquivos): o status é o do Requerial.
- Não mostre nem procure o token da sessão: ele fica no processo MCP.
- Nunca feche o tmux de outro agente; só o seu, e só depois de encerrar a sua sessão.
- Para criar tarefas para outros agentes, pausar ou mover tarefas que não são suas: skill
  `requerial-coordenacao`.
