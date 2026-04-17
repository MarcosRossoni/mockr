package engine

// Package engine: renderiza templates {{...}} dentro de valores JSON.
// Permite que respostas mock gerem dados dinâmicos a cada requisição.

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	mrand "math/rand/v2"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
)

// templateRe encontra todos os tokens {{nome}} dentro de uma string.
var templateRe = regexp.MustCompile(`\{\{([^}]+)\}\}`)

// seq é um contador global atômico para {{seq}}.
// atomic garante que incrementos de goroutines concorrentes não colidem.
var seq int64

// Render percorre recursivamente um valor JSON desserializado (any) e
// substitui strings que contenham templates pelos valores gerados.
// Retorna a estrutura reconstruída — o original não é modificado.
func Render(data any) any {
	return walk(data)
}

// RenderJSON é um atalho que renderiza e serializa para []byte.
func RenderJSON(data any) ([]byte, error) {
	return json.MarshalIndent(Render(data), "", "  ")
}

// walk percorre a árvore de dados recursivamente.
// Em Go, type switch permite inspecionar o tipo concreto de um interface{}.
func walk(v any) any {
	switch val := v.(type) {
	case map[string]any:
		// Reconstrói o mapa com cada valor renderizado
		out := make(map[string]any, len(val))
		for k, v2 := range val {
			out[k] = walk(v2)
		}
		return out

	case []any:
		// Reconstrói o slice com cada elemento renderizado
		out := make([]any, len(val))
		for i, v2 := range val {
			out[i] = walk(v2)
		}
		return out

	case string:
		return renderString(val)

	default:
		// Números, booleans, nil — devolvemos sem modificar
		return v
	}
}

// renderString resolve os templates dentro de uma string.
// Se a string inteira for um único template (ex: "{{seq}}"), devolve o
// tipo nativo (int, bool, float64) em vez de string — permitindo que
// campos JSON recebam o tipo correto.
func renderString(s string) any {
	matches := templateRe.FindAllString(s, -1)
	if len(matches) == 0 {
		return s
	}

	// String é exatamente um template — pode devolver tipo nativo
	if len(matches) == 1 && strings.TrimSpace(s) == matches[0] {
		name := extractName(matches[0])
		return resolve(name)
	}

	// Múltiplos templates ou template embutido em texto — substituição textual
	result := templateRe.ReplaceAllStringFunc(s, func(match string) string {
		return fmt.Sprintf("%v", resolve(extractName(match)))
	})
	return result
}

func extractName(token string) string {
	// token tem formato "{{nome}}" — remove as chaves e espaços
	return strings.TrimSpace(token[2 : len(token)-2])
}

// resolve converte um nome de template no valor correspondente.
func resolve(name string) any {
	switch name {
	case "uuid":
		return newUUID()

	case "seq":
		// atomic.AddInt64 incrementa e retorna o novo valor atomicamente
		return int(atomic.AddInt64(&seq, 1))

	case "now":
		return time.Now().UTC().Format(time.RFC3339)

	case "now.unix":
		return time.Now().Unix()

	case "rand.int":
		return mrand.IntN(10000)

	case "rand.bool":
		return mrand.IntN(2) == 1

	case "rand.float":
		// Arredonda para 2 casas decimais para ficar legível no JSON
		f := mrand.Float64()
		return float64(int(f*100)) / 100

	case "faker.name":
		return pickFirst() + " " + pickLast()

	case "faker.first_name":
		return pickFirst()

	case "faker.last_name":
		return pickLast()

	case "faker.email":
		first := strings.ToLower(removeDiacritics(pickFirst()))
		last := strings.ToLower(removeDiacritics(pickLast()))
		domains := []string{"gmail.com", "outlook.com", "yahoo.com", "exemplo.com"}
		return fmt.Sprintf("%s.%s@%s", first, last, pick(domains))

	case "faker.username":
		first := strings.ToLower(removeDiacritics(pickFirst()))
		return fmt.Sprintf("%s%d", first, mrand.IntN(999)+1)

	case "faker.phone":
		ddd := []string{"11", "21", "31", "41", "51", "61", "71", "81", "85", "91"}
		return fmt.Sprintf("+55 %s 9%04d-%04d", pick(ddd), mrand.IntN(10000), mrand.IntN(10000))

	case "faker.word":
		return pick(words)

	case "faker.sentence":
		n := mrand.IntN(5) + 4
		ws := make([]string, n)
		for i := range ws {
			ws[i] = pick(words)
		}
		ws[0] = strings.Title(ws[0]) //nolint:staticcheck
		return strings.Join(ws, " ") + "."

	default:
		// Template desconhecido — devolve o nome entre colchetes para facilitar depuração
		return "[?" + name + "]"
	}
}

// --- UUID v4 ---

func newUUID() string {
	b := make([]byte, 16)
	rand.Read(b) //nolint:errcheck
	b[6] = (b[6] & 0x0f) | 0x40 // versão 4
	b[8] = (b[8] & 0x3f) | 0x80 // variante RFC 4122
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// --- HELPERS ---

func pick(list []string) string {
	if len(list) == 0 {
		return ""
	}
	return list[mrand.IntN(len(list))]
}

func pickFirst() string { return pick(firstNames) }
func pickLast() string  { return pick(lastNames) }

// removeDiacritics faz substituição simples de caracteres acentuados.
func removeDiacritics(s string) string {
	replacer := strings.NewReplacer(
		"á", "a", "à", "a", "ã", "a", "â", "a",
		"é", "e", "ê", "e",
		"í", "i",
		"ó", "o", "õ", "o", "ô", "o",
		"ú", "u", "ü", "u",
		"ç", "c",
		"Á", "A", "À", "A", "Ã", "A", "Â", "A",
		"É", "E", "Ê", "E",
		"Í", "I",
		"Ó", "O", "Õ", "O", "Ô", "O",
		"Ú", "U",
		"Ç", "C",
	)
	return replacer.Replace(s)
}

// randInt64 usa crypto/rand para gerar um int64 — usado apenas internamente
// onde queremos aleatoriedade criptograficamente segura (UUID).
func randInt64(max int64) int64 {
	n, _ := rand.Int(rand.Reader, big.NewInt(max))
	return n.Int64()
}

// --- DADOS FAKER ---

var firstNames = []string{
	"Ana", "Bruno", "Carlos", "Daniela", "Eduardo", "Fernanda",
	"Gabriel", "Helena", "Igor", "Juliana", "Lucas", "Mariana",
	"Nicolas", "Olivia", "Pedro", "Rafaela", "Samuel", "Tatiana",
	"Victor", "Yasmin", "Mateus", "Larissa", "Felipe", "Camila",
	"Thiago", "Beatriz", "Rodrigo", "Amanda", "Gustavo", "Letícia",
}

var lastNames = []string{
	"Silva", "Santos", "Oliveira", "Souza", "Rodrigues", "Ferreira",
	"Alves", "Pereira", "Lima", "Gomes", "Costa", "Ribeiro",
	"Martins", "Carvalho", "Almeida", "Lopes", "Sousa", "Fernandes",
	"Vieira", "Barbosa", "Rocha", "Dias", "Nascimento", "Andrade",
	"Moreira", "Nunes", "Marques", "Machado", "Mendes", "Freitas",
}

var words = []string{
	"lorem", "ipsum", "dolor", "amet", "consectetur", "adipiscing",
	"elit", "sed", "eiusmod", "tempor", "incididunt", "labore",
	"magna", "aliqua", "enim", "veniam", "quis", "nostrud",
	"exercitation", "ullamco", "laboris", "nisi", "aliquip",
	"commodo", "consequat", "duis", "aute", "irure", "reprehenderit",
}
