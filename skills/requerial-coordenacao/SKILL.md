---
name: requerial-coordenacao
description: "Coordenar o trabalho de agentes de IA pelo requerial: criar tarefas, pôr um agente para trabalhar numa tarefa, pausar, bloquear, mandar para validação, concluir ou retomar tarefas de outros agentes. Use quando o usuário pedir para criar uma tarefa ou um agente, iniciar, pausar, bloquear, retomar ou concluir uma tarefa que não é a deste chat, ou revisar o que está em andamento. Triggers: criar tarefa, criar agente, novo agente para, subir um agente, pausar a tarefa, bloquear a tarefa, retomar a tarefa, concluir a tarefa, mover para validação, o que está em andamento, matar o agente de, encerrar o agente de."
user-invocable: true
---

# requerial: coordenar agentes

O requerial é a fonte única do status das tarefas. Coordenar é **mudar o status**; o resto é
consequência:

| Você muda | O requerial faz |
|---|---|
| tarefa nova ou existente → `em_andamento`, sem sessão ativa | o lançador sobe um agente para ela, numa sessão tmux |
| `em_andamento` → qualquer outro status | o lançador pede ao agente vivo que encerre a sessão com resumo; o agente fecha o próprio tmux |
| entre outros status (`em_validacao` → `concluida`...) | nada além do status |

Você não cria sessões tmux, não escolhe pastas e não fecha agentes à mão.

Se as ferramentas `mcp__requerial__*` não estiverem disponíveis, diga isso ao usuário e pare.

## Status

`a_fazer` (registrada, sem agente) · `em_andamento` (com agente trabalhando) · `pausada` (o usuário
decidiu parar) · `bloqueada` (dependência externa) · `em_validacao` (pronta, falta o usuário validar)
· `concluida`.

`pausada` e `bloqueada` param do mesmo jeito; muda a causa. Para `bloqueada`, diga ao usuário o que
bloqueia e o que precisa acontecer para destravar, e registre isso na descrição da tarefa
(`alterar_tarefa`, acrescentando uma seção "Bloqueio").

## Criar uma tarefa

`criar_tarefa` com:

- `titulo`: o trabalho, curto e específico;
- `organizacao`: a do pedido (pergunte se não estiver claro);
- `descricao`: objetivo, contexto e restrições que o agente precisa para começar sem perguntar. Não
  pesquise antes: o agente da tarefa faz a pesquisa dele;
- `status`: `em_andamento` para começar já (sobe o agente) ou `a_fazer` para só registrar.

Se este chat tem uma sessão de trabalho aberta, uma tarefa que ele cria em andamento **não** sobe
agente (o requerial entende que a sessão vai trabalhar nela). Para delegar a outro agente, crie a
tarefa sem sessão aberta neste chat, ou crie como `a_fazer` e depois mude para `em_andamento`.

Depois de pôr em andamento, confira com `obter_tarefa` em cerca de um minuto que `sessao_ativa`
apareceu. Se não aparecer, o agente não subiu: avise o usuário.

## Pausar, bloquear, validar, concluir ou retomar

- **Parar** (`pausada`, `bloqueada`, `em_validacao`, `concluida`): `alterar_tarefa(id, status)`. Se
  havia agente, ele encerra com resumo sozinho. Não mande mensagens a ele nem feche o tmux dele.
- **Retomar**: `alterar_tarefa(id, status: "em_andamento")`. Sobe um agente novo, que começa pelo
  resumo da última sessão.
- **Tarefa com sessão ativa mas sem agente vivo**: encerre pelo frontend (encerramento manual, com
  resumo). Não use a API por fora.

## Achar uma tarefa

O MCP não lista todas as tarefas, de propósito: custa muitos tokens e quase nunca é preciso. Use o id
que o usuário citar; se ele não souber, peça para procurar no frontend. Com o id, `obter_tarefa` e
`historico_da_tarefa` dão o estado e as sessões.

## Requisitos e a cadeia

Um **Requisito** (`R-0001`) nunca fecha: é o que precisa ser (e continuar) verdade. Tarefas tentam
atendê-lo. Cada requisito tem uma **Origem** (`O-0001`): uma pessoa que pediu ou um documento (lei,
norma, contrato). Requisitos derivam de outros, formando um grafo: "backup em dois destinos" deriva
de "cópia fora do ambiente principal", que deriva de "cumprir a lei de dados".

- **Um só requisito zero**: o primeiro criado, o único sem pais. Todo outro requisito deriva de
  algum (sem `deriva_de`, deriva do zero), então todo requisito tem uma trilha até o zero: é ela que
  dá o contexto inteiro dele.
- **Requisito é uma afirmação testável (falseável)**, no presente, sobre o estado do mundo — não uma
  ação. "O backup do cartório tem cópia em dois destinos independentes, verificada toda semana" ✅;
  "Fazer backup" ❌ (isso é tarefa). Quanto mais abaixo na cadeia, mais concreto e verificável.
- **Boa prática: no máximo 5 requisitos derivados de cada um.** Passou disso, agrupe-os sob
  requisitos intermediários (cada um também uma afirmação testável). Não é bloqueado; o requisito
  só passa a pedir atenção.
- O campo `atencao` de cada requisito diz o que falta para ele estar bem cuidado (muitos filhos,
  sem origem, sem descrição, sem tarefa, trabalho parado). Ao mexer num requisito, resolva o que
  estiver ao alcance.
- Antes de criar uma origem, `listar_origens`: reaproveite a existente.
- `criar_requisito` com `origem` e `deriva_de`; sem `organizacao`, herda a do primeiro pai.
- `derivar_requisito` acrescenta um pai (um requisito pode derivar de vários); ciclos são recusados.
- `vincular_requisito` liga uma tarefa ao requisito que ela tenta atender.
- Registre o pedido verbatim na descrição do requisito: é a evidência de onde ele veio.

## Responder ao usuário

Diga o id da tarefa, o status novo e o que vai acontecer ("o agente vai encerrar com resumo", "um
agente está subindo").
