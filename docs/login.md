# Login pelo Google

O frontend e a API ficam no mesmo endereço. Para abrir o rex no navegador por uma rede (a tailnet,
por exemplo) sem deixá-lo aberto a quem estiver nela, o `rex servir` separa os endereços:

- `-endereco`: abertos, sem login. Use só loopback (`127.0.0.1`): é por eles que falam o MCP dos
  agentes, o lançador e as integrações (a ponte da captura, os ganchos).
- `-endereco-com-login` + `-login login.json`: tudo exige login pelo Google, inclusive a API.
  Sem cookie válido, a navegação vai para o Google e o `fetch` recebe `401` com
  `X-Rex-Login: necessario` (o frontend então refaz o login).

Um endereço que esteja nas duas listas fica com login.

## HTTPS

O Google só aceita redirect URI em `https://` com domínio (ou `http://localhost`). Exponha o
endereço com login atrás de um proxy com TLS; na tailnet, o `tailscale serve`:

```bash
rex servir -endereco 127.0.0.1:8790 -endereco-com-login 127.0.0.1:8791 -login login.json ...
tailscale serve --bg --https=443 http://127.0.0.1:8791
```

## Cliente OAuth

No Google Cloud Console, em *APIs e serviços → Credenciais → Criar credenciais → ID do cliente
OAuth*, tipo **Aplicativo da Web**, com o URI de redirecionamento autorizado
`<url_base>/login/retorno`. A tela de consentimento pode ficar em modo de teste, com o seu e-mail como
usuário de teste: o escopo é só `openid email`.

## Configuração

```json
{
  "url_base": "https://maquina.tailnet.ts.net",
  "client_id": "123-abc.apps.googleusercontent.com",
  "client_secret_ambiente": "REX_GOOGLE_CLIENT_SECRET",
  "chave_ambiente": "REX_CHAVE_LOGIN",
  "emails": ["voce@exemplo.com"],
  "duracao_horas": 720
}
```

- `client_secret_ambiente` e `chave_ambiente` nomeiam variáveis de ambiente: os segredos nunca ficam
  no JSON. A chave (≥ 32 caracteres; `openssl rand -base64 32`) assina os cookies; trocá-la desfaz
  todos os logins.
- `emails`: só esses entram, e só com o e-mail verificado no Google. Tirar um e-mail da lista desfaz
  o login dele no próximo pedido.
- `duracao_horas`: quanto vale um login. Padrão: 720 (30 dias).

`/login/sair` apaga o cookie.

## Como funciona

OAuth 2.0 com código e PKCE. State e verificador vão num cookie curto e assinado, restrito a
`/login/retorno`; nada fica no servidor. O id_token vem direto do token endpoint do Google, por TLS,
e por isso as afirmações valem sem conferir a assinatura (OpenID Connect Core, 3.1.3.7); o rex
confere emissor, audiência, validade, `email_verified` e a lista. O login é um cookie `HttpOnly`,
`Secure` (com `url_base` em https) e `SameSite=Lax`, com e-mail e validade assinados por HMAC-SHA256.
