package api

import (
	"regexp"
	"strings"
)

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

// TamanhoDoNome é o limite do nome curto de um agente, em caracteres: o mesmo do /rename do Claude
// Code.
const TamanhoDoNome = 200

var foraDoNome = regexp.MustCompile(`[^a-z0-9]+`)

var semAcento = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "é", "e", "ê", "e", "í", "i", "ó", "o", "ô", "o", "õ", "o",
	"ú", "u", "ü", "u", "ç", "c",
)

// PalavrasDoNome reduz um texto às palavras que podem entrar num nome curto: minúsculas, sem
// acento, só letras e números.
func PalavrasDoNome(texto string) []string {
	return strings.Fields(foraDoNome.ReplaceAllString(semAcento.Replace(strings.ToLower(texto)), " "))
}

// JuntarNome liga as palavras com hífen até o limite de caracteres, sem cortar palavra ao meio
// (uma primeira palavra longa demais é truncada).
func JuntarNome(palavras []string, limite int) string {
	nome := ""
	for _, p := range palavras {
		proximo := p
		if nome != "" {
			proximo = nome + "-" + p
		}
		if len(proximo) > limite {
			if nome == "" {
				nome = p[:limite]
			}
			break
		}
		nome = proximo
	}
	return nome
}

// NomeCurto normaliza o nome curto pedido para um agente (tmux e remote-control): minúsculas, sem
// acento, palavras ligadas por hífen, até TamanhoDoNome. Vazio quando não sobra nada.
func NomeCurto(texto string) string {
	return JuntarNome(PalavrasDoNome(texto), TamanhoDoNome)
}
