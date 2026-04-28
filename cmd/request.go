package cmd

import (
	"bytes"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	mrand "math/rand/v2"
	"net/http"
	"strings"
	"sync"
	"time"

	"mockr/config"
	"mockr/cryptoutil"
	"mockr/engine"
	"mockr/server"
)

type encryptOpts struct {
	pubKey    *rsa.PublicKey
	wrapField string // vazio = base64 puro (text/plain); não vazio = {"<campo>": "<base64>"} (application/json)
}

func runRequest(args []string) error {
	configPath, err := parseFlag(args, "--config")
	if err != nil {
		return fmt.Errorf("request requer --config: %w", err)
	}

	path, _ := parseFlag(args, "--path")
	_, urlProvided := parseFlag(args, "--url")

	if path == "" && urlProvided != nil {
		return fmt.Errorf("request requer --path ou --url")
	}

	method := "GET"
	if m, err := parseFlag(args, "--method"); err == nil {
		method = strings.ToUpper(m)
	}

	repeat := 1
	if v, err := parseFlag(args, "--repeat"); err == nil {
		if _, err := fmt.Sscanf(v, "%d", &repeat); err != nil || repeat < 1 {
			return fmt.Errorf("--repeat inválido: deve ser um inteiro >= 1")
		}
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("config inválida: %w", err)
	}

	var bodyTemplate any
	baseHeaders := make(map[string]string)
	var enc *encryptOpts

	var authTokens []string
	var authLabels []string
	var authHeader string

	if route := findRoute(cfg, method, path); route != nil {
		if hasBody(method) && route.BodyFile != "" {
			rb, err := config.LoadResponseBody(route.BodyFile)
			if err != nil {
				return fmt.Errorf("erro ao ler body da rota: %w", err)
			}
			bodyTemplate = rb.Raw
		}
		if route.EncryptBody {
			if cfg.Crypto == nil || cfg.Crypto.PublicKey == "" {
				return fmt.Errorf("rota usa encrypt_body: true mas nenhuma crypto.public_key está definida no config")
			}
			pubKey, err := cryptoutil.LoadPublicKey(cfg.Crypto.PublicKey)
			if err != nil {
				return fmt.Errorf("erro ao carregar chave pública para encrypt_body: %w", err)
			}
			enc = &encryptOpts{pubKey: pubKey, wrapField: route.EncryptBodyWrap}
		}
		for k, v := range route.Headers {
			baseHeaders[k] = v
		}
		if route.UseAuth {
			if cfg.Auth == nil {
				return fmt.Errorf("rota usa auth: true mas nenhum bloco auth está definido no config")
			}
			if cfg.Auth.Users != "" {
				authTokens, authLabels, err = resolveAuthTokens(cfg.Auth)
			} else {
				var h, v string
				h, v, err = resolveAuthToken(cfg.Auth)
				authTokens = []string{v}
				authLabels = []string{"user"}
				authHeader = h
			}
			if err != nil {
				return fmt.Errorf("falha no fluxo de autenticação: %w", err)
			}
			if authHeader == "" {
				authHeader = cfg.Auth.Header
			}
		}
	}

	var url string
	if externalURL, err := parseFlag(args, "--url"); err == nil {
		url = externalURL
	} else {
		srv, err := server.New(cfg, 0)
		if err != nil {
			return fmt.Errorf("erro ao criar servidor: %w", err)
		}
		go srv.Start() //nolint:errcheck

		base := fmt.Sprintf("http://localhost:%d", srv.Port())
		if err := waitReady(base, 3*time.Second); err != nil {
			return fmt.Errorf("servidor não respondeu: %w", err)
		}
		url = base + path
	}

	for i := 0; i < repeat; i++ {
		iterHeaders := copyMap(baseHeaders)
		label := ""

		if len(authTokens) > 0 {
			idx := mrand.IntN(len(authTokens))
			iterHeaders[authHeader] = authTokens[idx]
			if len(authTokens) > 1 {
				label = authLabels[idx]
			}
		}

		if repeat > 1 {
			if label != "" {
				fmt.Printf("── requisição %d/%d  [%s]\n", i+1, repeat, label)
			} else {
				fmt.Printf("── requisição %d/%d\n", i+1, repeat)
			}
		}

		if err := fireRequest(method, url, bodyTemplate, iterHeaders, enc); err != nil {
			fmt.Printf("  erro: %v\n", err)
		}

		if repeat > 1 && i < repeat-1 {
			fmt.Println()
		}
	}

	return nil
}

func fireRequest(method, url string, bodyTemplate any, headers map[string]string, enc *encryptOpts) error {
	var reqBody io.Reader
	var contentType string

	if bodyTemplate != nil {
		rendered, err := engine.RenderJSON(bodyTemplate)
		if err != nil {
			return fmt.Errorf("erro ao renderizar body: %w", err)
		}

		if enc != nil {
			encrypted, err := cryptoutil.Encrypt(string(rendered), enc.pubKey)
			if err != nil {
				return fmt.Errorf("erro ao criptografar body: %w", err)
			}
			if enc.wrapField != "" {
				wrapped, _ := json.Marshal(map[string]string{enc.wrapField: encrypted})
				reqBody = bytes.NewReader(wrapped)
				contentType = "application/json"
			} else {
				reqBody = bytes.NewReader([]byte(encrypted))
				contentType = "text/plain; charset=utf-8"
			}
		} else {
			reqBody = bytes.NewReader(rendered)
			contentType = "application/json"
		}
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return err
	}
	if bodyTemplate != nil {
		req.Header.Set("Content-Type", contentType)
	}

	resolvedHeaders := make(map[string]string, len(headers))
	for k, v := range headers {
		resolved := fmt.Sprintf("%v", engine.Render(v))
		req.Header.Set(k, resolved)
		resolvedHeaders[k] = resolved
	}

	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	elapsed := time.Since(start)

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	printResponse(method, url, resp.StatusCode, elapsed, raw, resolvedHeaders)
	return nil
}

// resolveAuthToken autentica um único usuário (campo body do config).
func resolveAuthToken(auth *config.AuthConfig) (string, string, error) {
	var credBody io.Reader
	if auth.Body != "" {
		rb, err := config.LoadResponseBody(auth.Body)
		if err != nil {
			return "", "", fmt.Errorf("erro ao ler body de auth: %w", err)
		}
		rendered, err := engine.RenderJSON(rb.Raw)
		if err != nil {
			return "", "", fmt.Errorf("erro ao renderizar body de auth: %w", err)
		}
		credBody = bytes.NewReader(rendered)
	}

	fmt.Printf("  auth → %s %s", strings.ToUpper(auth.Method), auth.URL)
	token, err := doAuthRequest(auth, credBody)
	if err != nil {
		fmt.Println("  ✗")
		return "", "", err
	}
	fmt.Printf("  ✓  (%s extraído)\n", auth.Extract)
	return auth.Header, auth.Prefix + token, nil
}

// resolveAuthTokens autentica múltiplos usuários em paralelo (campo users do config).
// Retorna os tokens, os labels de identificação e qualquer erro de autenticação.
func resolveAuthTokens(auth *config.AuthConfig) ([]string, []string, error) {
	rb, err := config.LoadResponseBody(auth.Users)
	if err != nil {
		return nil, nil, fmt.Errorf("erro ao ler users: %w", err)
	}
	users, ok := rb.Raw.([]any)
	if !ok {
		return nil, nil, fmt.Errorf("auth.users deve ser um array JSON")
	}

	fmt.Printf("  auth → autenticando %d usuário(s)...\n", len(users))

	type result struct {
		token string
		label string
		err   error
	}

	results := make([]result, len(users))
	var wg sync.WaitGroup

	for i, u := range users {
		wg.Add(1)
		go func(idx int, cred any) {
			defer wg.Done()
			credMap, _ := cred.(map[string]any)
			label := extractLabel(credMap, idx)

			rendered, err := engine.RenderJSON(cred)
			if err != nil {
				results[idx] = result{err: err, label: label}
				return
			}
			token, err := doAuthRequest(auth, bytes.NewReader(rendered))
			results[idx] = result{token: token, label: label, err: err}
		}(i, u)
	}

	wg.Wait()

	var tokens, labels, failed []string
	for _, r := range results {
		if r.err != nil {
			failed = append(failed, fmt.Sprintf("%s (%v)", r.label, r.err))
			continue
		}
		tokens = append(tokens, auth.Prefix+r.token)
		labels = append(labels, r.label)
	}

	if len(failed) > 0 {
		return nil, nil, fmt.Errorf("falha ao autenticar: %s", strings.Join(failed, ", "))
	}

	fmt.Printf("  auth ✓  %s  (%d tokens prontos)\n", strings.Join(labels, " | "), len(tokens))
	return tokens, labels, nil
}

// doAuthRequest executa a requisição de auth e extrai o token da resposta.
// É a função central compartilhada pelo fluxo de usuário único e múltiplo.
func doAuthRequest(auth *config.AuthConfig, credBody io.Reader) (string, error) {
	method := strings.ToUpper(auth.Method)
	if method == "" {
		method = http.MethodPost
	}

	req, err := http.NewRequest(method, auth.URL, credBody)
	if err != nil {
		return "", err
	}
	if credBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, string(raw))
	}

	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return "", fmt.Errorf("response não é JSON válido: %w", err)
	}

	token, ok := extractFromJSON(data, auth.Extract)
	if !ok {
		return "", fmt.Errorf("campo %q não encontrado na resposta", auth.Extract)
	}
	return fmt.Sprintf("%v", token), nil
}

// extractLabel tenta extrair um identificador legível do objeto de credenciais.
func extractLabel(cred map[string]any, idx int) string {
	for _, field := range []string{"username", "email", "user", "login", "name"} {
		if v, ok := cred[field]; ok {
			return fmt.Sprintf("%v", v)
		}
	}
	return fmt.Sprintf("user%d", idx+1)
}

func printResponse(method, url string, status int, elapsed time.Duration, body []byte, sentHeaders map[string]string) {
	statusColor := colorForStatus(status)
	fmt.Printf("%s %s  →  %s%d%s  (%s)\n",
		method, url,
		statusColor, status, colorReset,
		elapsed.Round(time.Millisecond),
	)

	if len(sentHeaders) > 0 {
		fmt.Println("  headers enviados:")
		for k, v := range sentHeaders {
			fmt.Printf("    %s: %s\n", k, v)
		}
	}

	if len(body) == 0 {
		return
	}

	var pretty bytes.Buffer
	if json.Indent(&pretty, body, "", "  ") == nil {
		fmt.Println()
		fmt.Println(pretty.String())
	} else {
		fmt.Println()
		fmt.Println(string(body))
	}
}

func copyMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func extractFromJSON(data map[string]any, path string) (any, bool) {
	dot := strings.Index(path, ".")
	if dot == -1 {
		v, ok := data[path]
		return v, ok
	}
	head, tail := path[:dot], path[dot+1:]
	nested, ok := data[head]
	if !ok {
		return nil, false
	}
	nestedMap, ok := nested.(map[string]any)
	if !ok {
		return nil, false
	}
	return extractFromJSON(nestedMap, tail)
}

func hasBody(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		return true
	}
	return false
}

func findRoute(cfg *config.Config, method, path string) *config.Route {
	for i := range cfg.Routes {
		r := &cfg.Routes[i]
		if strings.EqualFold(r.Method, method) && matchSegments(r.Path, path) {
			return r
		}
	}
	return nil
}

func matchSegments(pattern, actual string) bool {
	pp := strings.Split(pattern, "/")
	ap := strings.Split(actual, "/")
	if len(pp) != len(ap) {
		return false
	}
	for i := range pp {
		if strings.HasPrefix(pp[i], ":") {
			continue
		}
		if pp[i] != ap[i] {
			return false
		}
	}
	return true
}

func colorForStatus(status int) string {
	switch {
	case status < 300:
		return "\033[32m"
	case status < 400:
		return "\033[33m"
	default:
		return "\033[31m"
	}
}

const colorReset = "\033[0m"
