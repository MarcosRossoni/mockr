package engine

// Package engine: renderiza templates {{...}} dentro de valores JSON.
// Permite que respostas mock gerem dados dinâmicos a cada requisição.

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// templateRe encontra todos os tokens {{nome}} dentro de uma string.
var templateRe = regexp.MustCompile(`\{\{([^}]+)\}\}`)

// Render percorre recursivamente um valor JSON desserializado (any) e
// substitui strings que contenham templates pelos valores gerados.
func Render(data any) any {
	return walkCtx(data, nil)
}

// RenderWithContext é igual a Render mas recebe o body da request como contexto.
// Permite que templates {{body.campo}} sejam resolvidos com valores da request.
func RenderWithContext(data any, bodyCtx map[string]any) any {
	return walkCtx(data, bodyCtx)
}

// RenderJSON é um atalho que renderiza e serializa para []byte.
func RenderJSON(data any) ([]byte, error) {
	return json.MarshalIndent(Render(data), "", "  ")
}

// RenderJSONWithContext é um atalho que renderiza com contexto e serializa para []byte.
func RenderJSONWithContext(data any, bodyCtx map[string]any) ([]byte, error) {
	return json.MarshalIndent(RenderWithContext(data, bodyCtx), "", "  ")
}

func walkCtx(v any, bodyCtx map[string]any) any {
	switch val := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, v2 := range val {
			out[k] = walkCtx(v2, bodyCtx)
		}
		return out

	case []any:
		out := make([]any, len(val))
		for i, v2 := range val {
			out[i] = walkCtx(v2, bodyCtx)
		}
		return out

	case string:
		return renderStringCtx(val, bodyCtx)

	default:
		return v
	}
}

// renderStringCtx resolve os templates dentro de uma string.
// Se a string inteira for um único template (ex: "{{seq}}"), devolve o
// tipo nativo (int, bool, float64) em vez de string.
func renderStringCtx(s string, bodyCtx map[string]any) any {
	matches := templateRe.FindAllString(s, -1)
	if len(matches) == 0 {
		return s
	}

	if len(matches) == 1 && strings.TrimSpace(s) == matches[0] {
		return resolveCtx(extractName(matches[0]), bodyCtx)
	}

	return templateRe.ReplaceAllStringFunc(s, func(match string) string {
		return fmt.Sprintf("%v", resolveCtx(extractName(match), bodyCtx))
	})
}

func extractName(token string) string {
	return strings.TrimSpace(token[2 : len(token)-2])
}

// resolveCtx despacha para lookupBody ({{body.*}}) ou resolve (todos os demais).
func resolveCtx(name string, bodyCtx map[string]any) any {
	if strings.HasPrefix(name, "body.") {
		if bodyCtx == nil {
			return ""
		}
		return lookupBody(bodyCtx, name[5:])
	}
	return resolve(name)
}

// lookupBody navega no mapa usando dot notation (ex: "address.city").
func lookupBody(m map[string]any, key string) any {
	dot := strings.Index(key, ".")
	if dot == -1 {
		v, ok := m[key]
		if !ok {
			return ""
		}
		return v
	}
	head, tail := key[:dot], key[dot+1:]
	nested, ok := m[head]
	if !ok {
		return ""
	}
	nestedMap, ok := nested.(map[string]any)
	if !ok {
		return ""
	}
	return lookupBody(nestedMap, tail)
}
