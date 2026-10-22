package login

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

const redirectDeTeste = "http://localhost:9999/callback"

// servidorDoConector sobe o rex com login e conector num endereço local, com um MCP de uma
// ferramenta, e devolve também o cookie de login do e-mail liberado.
func servidorDoConector(t *testing.T) (*Login, *httptest.Server, *http.Cookie) {
	t.Helper()
	var h http.Handler
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { h.ServeHTTP(w, r) }))
	t.Cleanup(srv.Close)
	ambiente := map[string]string{"SEGREDO": "s", "CHAVE": strings.Repeat("k", 32)}
	l, err := Novo(Config{
		URLBase: srv.URL, ClientID: clienteDeTeste, ClientSecretAmbiente: "SEGREDO", ChaveAmbiente: "CHAVE",
		Emails: []string{"dono@exemplo.com"},
	}, func(k string) string { return ambiente[k] })
	if err != nil {
		t.Fatal(err)
	}
	m := mcp.NewServer(&mcp.Implementation{Name: "rex"}, nil)
	mcp.AddTool(m, &mcp.Tool{Name: "voz_panorama"}, func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, map[string]string, error) {
		return nil, map[string]string{"sessoes": "nenhuma"}, nil
	})
	l.ComConector(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return m }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true, DisableLocalhostProtection: true}))
	h = l.Proteger(protegido)
	w := httptest.NewRecorder()
	l.gravarCookie(w, cookieSessao, "dono@exemplo.com", "/", time.Hour)
	return l, srv, w.Result().Cookies()[0]
}

var semSeguirRedirect = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

// navegar abre a URL de autorização como o navegador logado e devolve para onde ela redireciona.
func navegar(t *testing.T, endereco string, cookie *http.Cookie) *url.URL {
	t.Helper()
	req, _ := http.NewRequest("GET", endereco, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	resp, err := semSeguirRedirect.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	destino, err := url.Parse(resp.Header.Get("Location"))
	if resp.StatusCode != http.StatusFound || err != nil {
		t.Fatalf("autorizar devolveu %d, Location %q", resp.StatusCode, resp.Header.Get("Location"))
	}
	return destino
}

// O fluxo inteiro com o cliente OAuth do próprio go-sdk: descoberta pelo 401, registro dinâmico,
// autorização com PKCE, troca do código e a chamada da ferramenta.
func TestConectorFluxoCompletoComClienteDoSDK(t *testing.T) {
	_, srv, cookie := servidorDoConector(t)
	oauth, err := auth.NewAuthorizationCodeHandler(&auth.AuthorizationCodeHandlerConfig{
		DynamicClientRegistrationConfig: &auth.DynamicClientRegistrationConfig{Metadata: &oauthex.ClientRegistrationMetadata{
			RedirectURIs: []string{redirectDeTeste}, ClientName: "teste", GrantTypes: []string{"authorization_code", "refresh_token"},
		}},
		AuthorizationCodeFetcher: func(_ context.Context, args *auth.AuthorizationArgs) (*auth.AuthorizationResult, error) {
			volta := navegar(t, args.URL, cookie)
			if volta.Query().Get("code") == "" {
				return nil, errors.New("sem código: " + volta.String())
			}
			return &auth.AuthorizationResult{Code: volta.Query().Get("code"), State: volta.Query().Get("state"), Iss: volta.Query().Get("iss")}, nil
		},
		RequestRefreshToken: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	cliente := mcp.NewClient(&mcp.Implementation{Name: "teste"}, nil)
	sessao, err := cliente.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: srv.URL + CaminhoMCP, OAuthHandler: oauth}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer sessao.Close()
	res, err := sessao.CallTool(context.Background(), &mcp.CallToolParams{Name: "voz_panorama"})
	if err != nil || res.IsError {
		t.Fatalf("voz_panorama: %v %+v", err, res)
	}
	if texto, _ := json.Marshal(res.StructuredContent); !strings.Contains(string(texto), "nenhuma") {
		t.Fatalf("resposta inesperada: %s", texto)
	}
}

func TestConectorSemTokenRecebe401ComMetadados(t *testing.T) {
	_, srv, _ := servidorDoConector(t)
	resp, err := http.Post(srv.URL+CaminhoMCP, "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	esperado := `Bearer resource_metadata="` + srv.URL + `/.well-known/oauth-protected-resource/mcp"`
	if resp.StatusCode != http.StatusUnauthorized || resp.Header.Get("WWW-Authenticate") != esperado {
		t.Fatalf("status %d, WWW-Authenticate %q", resp.StatusCode, resp.Header.Get("WWW-Authenticate"))
	}
	var meta map[string]any
	resp, _ = http.Get(srv.URL + "/.well-known/oauth-protected-resource/mcp")
	json.NewDecoder(resp.Body).Decode(&meta)
	resp.Body.Close()
	if meta["resource"] != srv.URL+CaminhoMCP {
		t.Fatalf("resource %v", meta["resource"])
	}
}

func registrarCliente(t *testing.T, srv *httptest.Server, redirect string) (int, string) {
	t.Helper()
	resp, err := http.Post(srv.URL+caminhoRegistrar, "application/json", strings.NewReader(`{"redirect_uris":["`+redirect+`"]}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var corpo struct {
		ClientID string `json:"client_id"`
	}
	json.NewDecoder(resp.Body).Decode(&corpo)
	return resp.StatusCode, corpo.ClientID
}

func TestConectorSoRegistraRedirectDoClaudeOuLoopback(t *testing.T) {
	_, srv, _ := servidorDoConector(t)
	for redirect, status := range map[string]int{
		"https://claude.ai/api/mcp/auth_callback": http.StatusCreated,
		redirectDeTeste:                          http.StatusCreated,
		"https://atacante.exemplo/callback":      http.StatusBadRequest,
		"http://claude.ai/api/mcp/auth_callback": http.StatusBadRequest,
	} {
		if s, _ := registrarCliente(t, srv, redirect); s != status {
			t.Errorf("%s: status %d, esperado %d", redirect, s, status)
		}
	}
}

// pedirCodigo registra um cliente e percorre a autorização logado; devolve client_id e código.
func pedirCodigo(t *testing.T, srv *httptest.Server, cookie *http.Cookie, verificador string) (string, string) {
	t.Helper()
	_, clientID := registrarCliente(t, srv, redirectDeTeste)
	q := url.Values{
		"response_type": {"code"}, "client_id": {clientID}, "redirect_uri": {redirectDeTeste}, "state": {"st"},
		"code_challenge": {desafioS256(verificador)}, "code_challenge_method": {"S256"},
	}
	volta := navegar(t, srv.URL+caminhoAutorizar+"?"+q.Encode(), cookie)
	if volta.Query().Get("state") != "st" || volta.Query().Get("iss") != srv.URL {
		t.Fatalf("volta sem state ou iss: %s", volta)
	}
	return clientID, volta.Query().Get("code")
}

func trocar(t *testing.T, srv *httptest.Server, form url.Values) (int, map[string]any) {
	t.Helper()
	resp, err := http.PostForm(srv.URL+caminhoToken, form)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var corpo map[string]any
	json.NewDecoder(resp.Body).Decode(&corpo)
	return resp.StatusCode, corpo
}

func TestConectorCodigoValeUmaVezEComOVerificadorCerto(t *testing.T) {
	_, srv, cookie := servidorDoConector(t)
	clientID, codigo := pedirCodigo(t, srv, cookie, "verificador-com-tamanho-suficiente-para-o-pkce")
	form := url.Values{"grant_type": {"authorization_code"}, "code": {codigo}, "client_id": {clientID}, "redirect_uri": {redirectDeTeste}}
	form.Set("code_verifier", "outro")
	if s, corpo := trocar(t, srv, form); s != http.StatusBadRequest || corpo["error"] != "invalid_grant" {
		t.Fatalf("verificador errado: %d %v", s, corpo)
	}
	form.Set("code_verifier", "verificador-com-tamanho-suficiente-para-o-pkce")
	if s, corpo := trocar(t, srv, form); s != http.StatusOK || corpo["access_token"] == nil || corpo["refresh_token"] == nil {
		t.Fatalf("troca: %d %v", s, corpo)
	}
	if s, _ := trocar(t, srv, form); s != http.StatusBadRequest {
		t.Fatalf("o código valeu duas vezes: %d", s)
	}
}

func TestConectorRefreshRotativo(t *testing.T) {
	_, srv, cookie := servidorDoConector(t)
	clientID, codigo := pedirCodigo(t, srv, cookie, "v")
	_, corpo := trocar(t, srv, url.Values{"grant_type": {"authorization_code"}, "code": {codigo}, "client_id": {clientID}, "code_verifier": {"v"}})
	refresh := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {corpo["refresh_token"].(string)}, "client_id": {clientID}}
	s, novo := trocar(t, srv, refresh)
	if s != http.StatusOK || novo["refresh_token"] == corpo["refresh_token"] {
		t.Fatalf("refresh: %d %v", s, novo)
	}
	if s, _ := trocar(t, srv, refresh); s != http.StatusBadRequest {
		t.Fatalf("o refresh antigo valeu de novo: %d", s)
	}
	// O access token não serve como refresh, nem o contrário.
	refresh.Set("refresh_token", novo["access_token"].(string))
	if s, _ := trocar(t, srv, refresh); s != http.StatusBadRequest {
		t.Fatalf("access token aceito como refresh: %d", s)
	}
}

func TestConectorSemLoginVaiAoGoogleEVoltaParaAAutorizacao(t *testing.T) {
	l, srv, _ := servidorDoConector(t)
	_, clientID := registrarCliente(t, srv, redirectDeTeste)
	pedido := caminhoAutorizar + "?" + url.Values{
		"response_type": {"code"}, "client_id": {clientID}, "redirect_uri": {redirectDeTeste},
		"code_challenge": {desafioS256("v")}, "code_challenge_method": {"S256"},
	}.Encode()
	destino := navegar(t, srv.URL+pedido, nil)
	if destino.Path != CaminhoEntrar || destino.Query().Get("voltar") != pedido {
		t.Fatalf("sem login devia ir a %s?voltar=…, foi a %s", CaminhoEntrar, destino)
	}
	l.trocar = func(context.Context, string, string) (string, error) {
		return idToken(t, afirmacoes("dono@exemplo.com")), nil
	}
	h := l.Proteger(protegido)
	ida := httptest.NewRecorder()
	h.ServeHTTP(ida, httptest.NewRequest("GET", destino.String(), nil))
	google, _ := url.Parse(ida.Header().Get("Location"))
	volta := httptest.NewRequest("GET", CaminhoRetorno+"?code=c&state="+google.Query().Get("state"), nil)
	for _, c := range ida.Result().Cookies() {
		volta.AddCookie(c)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, volta)
	if w.Header().Get("Location") != pedido {
		t.Fatalf("depois do login devia voltar a %s, foi a %q", pedido, w.Header().Get("Location"))
	}
}

func TestVoltarSoParaCaminhoLocal(t *testing.T) {
	for v, esperado := range map[string]string{"/oauth/autorizar?x=1": "/oauth/autorizar?x=1", "//atacante.exemplo": "", "/\\atacante.exemplo": "", "https://atacante.exemplo": ""} {
		if voltarLocal(v) != esperado {
			t.Errorf("voltarLocal(%q) = %q", v, voltarLocal(v))
		}
	}
}

func TestConectorAceitaClientIDNoBasic(t *testing.T) {
	_, srv, cookie := servidorDoConector(t)
	clientID, codigo := pedirCodigo(t, srv, cookie, "v")
	req, _ := http.NewRequest("POST", srv.URL+caminhoToken, strings.NewReader(url.Values{
		"grant_type": {"authorization_code"}, "code": {codigo}, "redirect_uri": {redirectDeTeste}, "code_verifier": {"v"},
	}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, "")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("troca com client_id no Basic: %d", resp.StatusCode)
	}
}

// O túnel entrega em loopback com o Host público: o /mcp tem de aceitar.
func TestConectorAceitaHostPublicoEmLoopback(t *testing.T) {
	_, srv, cookie := servidorDoConector(t)
	clientID, codigo := pedirCodigo(t, srv, cookie, "v")
	_, corpo := trocar(t, srv, url.Values{"grant_type": {"authorization_code"}, "code": {codigo}, "client_id": {clientID}, "code_verifier": {"v"}})
	req, _ := http.NewRequest("POST", srv.URL+CaminhoMCP, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Host = "rex.exemplo.com.br"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Mcp-Protocol-Version", "2025-06-18")
	req.Header.Set("Authorization", "Bearer "+corpo["access_token"].(string))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var lista struct {
		Result struct{ Tools []struct{ Name string } } `json:"result"`
	}
	json.NewDecoder(resp.Body).Decode(&lista)
	if resp.StatusCode != http.StatusOK || len(lista.Result.Tools) != 1 {
		t.Fatalf("tools/list com Host público: %d %+v", resp.StatusCode, lista)
	}
}
