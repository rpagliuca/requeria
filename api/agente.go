package api

import "strings"

// nomesDeEsforco escreve os níveis de esforço como aparecem no nome do agente.
var nomesDeEsforco = map[string]string{
	"low": "Low", "medium": "Medium", "high": "High", "xhigh": "XHigh", "max": "Max",
}

// NomeDoAgente monta o nome imutável do agente: provider e modelo, mais o esforço, como
// "Claude Opus 5.5 Medium". Sem esforço, só o modelo.
func NomeDoAgente(modelo, esforco string) string {
	modelo, esforco = strings.TrimSpace(modelo), strings.TrimSpace(esforco)
	if esforco == "" {
		return modelo
	}
	nome, ok := nomesDeEsforco[strings.ToLower(esforco)]
	if !ok {
		nome = strings.ToUpper(esforco[:1]) + esforco[1:]
	}
	return modelo + " " + nome
}
