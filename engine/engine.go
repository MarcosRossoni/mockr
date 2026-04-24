package engine

// Package engine: renderiza templates {{...}} dentro de valores JSON.
// Permite que respostas mock gerem dados dinâmicos a cada requisição.

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// templateRe encontra todos os tokens {{nome}} dentro de uma string.
var templateRe = regexp.MustCompile(`\{\{([^}]+)\}\}`)

// RequestContext agrupa os contextos disponíveis durante a renderização de um template.
type RequestContext struct {
	Body  map[string]any // campos do body da request (POST/PUT/PATCH)
	Query url.Values     // query params da URL (?foo=bar)
}

// Render percorre recursivamente um valor JSON desserializado (any) e
// substitui strings que contenham templates pelos valores gerados.
func Render(data any) any {
	return walkCtx(data, nil)
}

// RenderWithContext é igual a Render mas recebe o body da request como contexto.
// Mantido para compatibilidade; prefira RenderWithRequestCtx quando houver query params.
func RenderWithContext(data any, bodyCtx map[string]any) any {
	return walkCtx(data, &RequestContext{Body: bodyCtx})
}

// RenderWithRequestCtx renderiza com body e query params disponíveis como templates.
func RenderWithRequestCtx(data any, ctx *RequestContext) any {
	return walkCtx(data, ctx)
}

// RenderJSON é um atalho que renderiza e serializa para []byte.
func RenderJSON(data any) ([]byte, error) {
	return json.MarshalIndent(Render(data), "", "  ")
}

// RenderJSONWithContext é um atalho que renderiza com body e serializa para []byte.
func RenderJSONWithContext(data any, bodyCtx map[string]any) ([]byte, error) {
	return json.MarshalIndent(RenderWithContext(data, bodyCtx), "", "  ")
}

// RenderJSONWithRequestCtx é o atalho completo: renderiza com body + query params.
func RenderJSONWithRequestCtx(data any, ctx *RequestContext) ([]byte, error) {
	return json.MarshalIndent(RenderWithRequestCtx(data, ctx), "", "  ")
}

func walkCtx(v any, ctx *RequestContext) any {
	switch val := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, v2 := range val {
			out[k] = walkCtx(v2, ctx)
		}
		return out

	case []any:
		out := make([]any, len(val))
		for i, v2 := range val {
			out[i] = walkCtx(v2, ctx)
		}
		return out

	case string:
		return renderStringCtx(val, ctx)

	default:
		return v
	}
}

// renderStringCtx resolve os templates dentro de uma string.
// Se a string inteira for um único template (ex: "{{seq}}"), devolve o
// tipo nativo (int, bool, float64) em vez de string.
func renderStringCtx(s string, ctx *RequestContext) any {
	matches := templateRe.FindAllString(s, -1)
	if len(matches) == 0 {
		return s
	}

	if len(matches) == 1 && strings.TrimSpace(s) == matches[0] {
		return resolveCtx(extractName(matches[0]), ctx)
	}

	return templateRe.ReplaceAllStringFunc(s, func(match string) string {
		return fmt.Sprintf("%v", resolveCtx(extractName(match), ctx))
	})
}

func extractName(token string) string {
	return strings.TrimSpace(token[2 : len(token)-2])
}

// resolveCtx despacha para lookupBody ({{body.*}}), lookupQuery ({{query.*}})
// ou resolve (todos os demais).
func resolveCtx(name string, ctx *RequestContext) any {
	switch {
	case strings.HasPrefix(name, "body."):
		if ctx == nil || ctx.Body == nil {
			return ""
		}
		return lookupBody(ctx.Body, name[5:])
	case strings.HasPrefix(name, "query."):
		if ctx == nil || ctx.Query == nil {
			return ""
		}
		return ctx.Query.Get(name[6:])
	default:
		return resolve(name)
	}
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
