# Integração

O Requerial não fala com nenhum sistema de notificação, celular ou assistente de voz. Ele publica o
que acontece e o que está pendente por interfaces genéricas, e cada instalação liga a elas o que
quiser por configuração, sem código particular neste repositório.

Três interfaces de saída e uma de entrada, todas no contrato [`api/openapi.yaml`](../api/openapi.yaml):

| Interface | Para quem | Garantia |
|---|---|---|
| `GET /integracao/pendencias` | quem faz polling (um retrato do que pede atenção agora) | estado: quem perdeu uma chamada se recupera na seguinte |
| `GET /eventos?depois=<id>&tipo=…` | quem quer cada transição, sem perder nenhuma | cursor sobre o log de eventos, que nunca é apagado |
| ganchos (`-integracoes`) | quem quer ser avisado na hora, sem polling | pelo menos uma tentativa por evento, na ordem, sobrevivendo a reinícios; desistência registrada no log |
| `POST /captura` | atalhos de voz e outros canais sem sessão | idempotente pela `chave` |

## Por que pendências + ganchos, e não só ganchos

O consumidor que motivou o desenho (uma integração pessoal que desenha na Dynamic Island e na
barra do i3, com polling de 1 min no laptop e uma função na nuvem) precisa de **estado**: um lembrete
disparado continua pedindo atenção até alguém confirmar ou adiar, e "silêncio nunca é tudo bem". Um
gancho perdido (consumidor fora do ar, rede caída) num desenho só de eventos deixaria o lembrete
sumir em silêncio. O retrato de pendências se corrige sozinho na chamada seguinte, e a lógica de
apresentação (quanto antes aparecer, quando tocar alarme) fica toda no consumidor: o Requerial
guarda só a hora marcada, e `horizonte` deixa o consumidor ver o que vence nas próximas horas.

Os ganchos existem para a latência (avisar no instante em que o lembrete é criado ou dispara, para
o consumidor reagendar o alarme no celular antes de o laptop desligar) e para consumidores que não
alcançam o Requerial por polling. Não são a fonte de verdade.

## Pendências

`GET /integracao/pendencias?horizonte=120` devolve:

- `lembretes`: os disparados (até confirmar ou adiar) e os agendados/adiados que vencem até
  `horizonte_ate`;
- `tarefas_em_validacao`: o que espera validação do humano;
- `sessoes_aguardando`: sessões cujo agente entregou ou está bloqueado e espera resposta
  (`POST /sessoes/{id}/espera`, desfeito por `DELETE` ou pelo encerramento);
- `ultimo_evento`: cursor para continuar por `GET /eventos?depois=`.

## Eventos

Todo evento tem `id` crescente, `momento`, `autor`, `entidade`, `tipo` e `dados`. Os que interessam
a integrações:

| Tipo | Entidade | Dados |
|---|---|---|
| `lembrete_criado` | L-… | `quando` |
| `lembrete_alterado` | L-… | campos alterados, com o valor novo; `texto` vai só como `true` (o texto não entra no log) |
| `lembrete_disparado` | L-… | `quando`, `atrasado` |
| `lembrete_adiado` | L-… | `quando` (nova hora) |
| `lembrete_confirmado` | L-… | — |
| `lembrete_cancelado` | L-… | — |
| `lembrete_reaberto` | L-… | `status` (agendado, ou disparado se a hora já passou) |
| `lembrete_apagado` | L-… | — (a notificação vai sem `lembrete`: ele não existe mais) |
| `tarefa_alterada` | T-… | campos alterados (`status: em_validacao` = foi para validação) |
| `agente_pedido` | T-… | `modelo`, `esforco`, `instrucao` |
| `sessao_iniciada`, `sessao_encerrada`, `sessao_substituida` | S-… | — |
| `sessao_aguardando` | S-… | `mensagem` |
| `sessao_retomada` | S-… | — |
| `integracao_falhou` | nome do gancho | `evento`, `erro` (nunca gera gancho) |

## Ganchos

`requerial servir -integracoes integracoes.json`. Arquivo particular de cada instalação:

```json
{
  "fuso": "America/Sao_Paulo",
  "base_publica": "http://exemplo:8790",
  "captura": {
    "token_ambiente": "REQUERIAL_TOKEN_CAPTURA",
    "organizacao": "Pessoal"
  },
  "ganchos": [
    {
      "nome": "barra",
      "eventos": ["lembrete_*", "sessao_aguardando", "sessao_retomada"],
      "comando": ["/caminho/do/script", "{tipo}", "{entidade}"]
    },
    {
      "nome": "nuvem",
      "eventos": ["lembrete_*"],
      "webhook": {
        "url": "https://exemplo/requerial",
        "cabecalhos": { "Authorization": "env:TOKEN_DA_NUVEM" }
      }
    }
  ]
}
```

- `fuso`: em que fuso a captura interpreta "amanhã às 9" e a tela mostra as horas. Padrão: o da máquina.
- `base_publica`: base do campo `link` dos lembretes. Sem ela, `link` fica ausente.
- `captura`: liga `POST /captura`; o token vem da variável de ambiente nomeada (o segredo nunca fica
  no JSON). Sem `captura`, o endpoint responde 404.
- `eventos`: tipos exatos ou com `*` no fim.
- `comando`: argv, sem shell; aceita `{tipo}`, `{entidade}` e `{evento}` (id). Recebe no stdin o
  JSON da notificação. Saída diferente de 0 conta como falha.
- `webhook`: `POST` com o JSON da notificação no corpo; valores `env:NOME` nos cabeçalhos vêm do
  ambiente. Resposta diferente de 2xx conta como falha.

A notificação é `{"evento": Evento, "lembrete"|"tarefa"|"sessao": <estado atual da entidade>}`
(schema `Notificacao` no contrato), para o consumidor não precisar chamar a API de volta.

Cada gancho segue o log de eventos por um cursor gravado no banco: um evento por vez, na ordem do
log, fora das requisições (um gancho lento não atrasa a API). Depois de reiniciar, continua de onde
parou; um gancho novo começa no fim do log, sem receber o histórico. Cada falha é tentada de novo
3 vezes (1 s, 10 s, 60 s) e depois vira `integracao_falhou`, e a entrega segue para o próximo evento.
A notificação é montada na hora da entrega, com o estado mais recente da entidade.

### O lançador de agentes

O lançador (`-lancador`) é o primeiro caso deste padrão: reage a `tarefa_alterada` com status
entrando ou saindo de `em_andamento` rodando um comando configurado. Continua com a configuração
própria por enquanto; quando migrar, vira um gancho de um tipo especial (`"lancador": {…}`) na mesma
lista, sobre os mesmos eventos.

## Agendador de lembretes

Dentro do `requerial servir`, a cada 15 s: todo lembrete agendado ou adiado com `quando` no
passado vira `disparado` e gera `lembrete_disparado`. Ao subir, o servidor dispara de uma vez os que
venceram enquanto ele estava parado, com `atrasado: true`; nada some em silêncio.

## Captura

`POST /captura` com `Authorization: Bearer <token de captura>`:

```json
{"tipo": "lembrete", "texto": "ligar para o contador amanhã às 9", "chave": "id-da-mensagem", "dita_em": "2026-10-20T21:14:00-03:00"}
```

Resposta em `text/plain`, uma frase de até 200 caracteres para ser falada:

| Código | Significa | O canal deve |
|---|---|---|
| 201 | criado; a frase repete o que foi entendido; `X-Requerial-Id` traz o id | falar a frase |
| 200 | `chave` repetida (devolve o mesmo resultado) ou `simular: true` | falar a frase |
| 422 | precisa de esclarecimento (hora ausente, ambígua ou no passado em relação a `dita_em`) | falar a pergunta e pedir de novo |
| 400, 401, 404 | recusado (pedido malformado, token errado, captura desligada) | falar o motivo; não reenviar |
| 5xx | falha do servidor | reenviar depois com a mesma `chave` |

- **lembrete**: o texto traz o que e quando juntos. Horas relativas contam de `dita_em` (padrão: a
  chegada). Se a hora já passou quando o pedido chega (fila atrasada), o lembrete é criado e
  disparado na hora, marcado como atrasado, e a frase diz isso.
- **tarefa**: sempre `a_fazer`; nunca aciona o lançador.
- **requisito**: título verbatim, sob o zero da organização, sem prioridade (vai para a triagem).

O autor de tudo no log de eventos é `captura`.

### Datas e horas aceitas

"hoje/amanhã/depois de amanhã às 9", "às 14h30", "9 da manhã", "3 da tarde", "segunda às 10",
"dia 25 às 8", "25/10 às 8", "daqui a 20 minutos", "daqui a 2 horas", "meio-dia", "meia-noite".

Regras para não chutar:

- hora sem dia vale para hoje; se já passou (em relação a `dita_em`), é 422 — nunca vira amanhã
  sozinha;
- hora de 1 a 11 sem "da manhã/da tarde/da noite" vale como dita (às 3 = 03:00), o que, combinado
  com a regra anterior, faz "às 3" dito às 14:00 virar 422;
- texto sem nenhuma hora reconhecida é 422;
- a frase de resposta sempre repete dia da semana, data e hora ("terça 21/10 às 09:00").
