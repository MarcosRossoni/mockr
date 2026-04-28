package cmd

// Este package "cmd" é responsável por interpretar os argumentos de terminal.
// Equivale ao seu "controller" de entrada da CLI.

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"

	"mockr/config"
	"mockr/cryptoutil"
	"mockr/server"
	"mockr/stress"
	"mockr/tui"
)

// Run é a função principal da CLI.
// Recebe os argumentos passados no terminal (ex: ["serve", "--config", "mock.yaml"])
//
// Em Go, funções podem retornar múltiplos valores.
// O padrão idiomático é retornar (resultado, error).
// Quando não há erro, retornamos nil (equivalente ao null do Java).
func Run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}

	// Em Go, switch não precisa de "break" — cada case para automaticamente.
	switch args[0] {
	case "serve":
		return runServe(args[1:])
	case "validate":
		return runValidate(args[1:])
	case "stress":
		return runStress(args[1:])
	case "request":
		return runRequest(args[1:])
	case "decrypt":
		return runDecrypt(args[1:])
	case "encrypt":
		return runEncrypt(args[1:])
	default:
		return errors.New("comando desconhecido: " + args[0])
	}
}

func printUsage() {
	fmt.Println(`
mockr — servidor de mocks para desenvolvimento

Uso:
  mockr serve    --config <arquivo.yaml> [--port <porta>]                                     Sobe o servidor mock
  mockr validate --config <arquivo.yaml>                                                      Valida o arquivo de config
  mockr stress   --config <arquivo.yaml> [-n <reqs>] [-c <conc>]                             Stress test (todas as rotas)
  mockr stress   --url <url> --method <METHOD> [-n <reqs>] [-c <conc>]                       Stress test (URL externa)
  mockr request  --config <arquivo.yaml> --path <path> [--method <METHOD>] [--repeat <n>]    Dispara contra o mock
  mockr request  --config <arquivo.yaml> --url <url>  [--method <METHOD>] [--path <path>]    Dispara contra URL externa
  mockr decrypt  --config <arquivo.yaml> --text <payload_base64>                             Descriptografa payload RSA
  mockr encrypt  --config <arquivo.yaml> --text <json_plaintext>                             Criptografa JSON com RSA

Exemplos:
  mockr serve --config ./mock.yaml
  mockr serve --config ./mock.yaml --port 8080
  mockr validate --config ./mock.yaml
  mockr stress --config ./mock.yaml -n 500 -c 20
  mockr stress --url http://localhost:9090/users --method GET -n 1000 -c 50
  mockr request --config ./mock.yaml --path /users
  mockr request --config ./mock.yaml --path /users --method POST --repeat 3
  mockr request --config ./mock.yaml --url http://minha-api.com/users --method POST --path /users
  mockr decrypt --config ./mock.yaml --text "SGVsbG8gV29ybGQ="
  mockr encrypt --config ./mock.yaml --text '{"id":1,"name":"Marco"}'
`)
}

func runServe(args []string) error {
	configPath, err := parseFlag(args, "--config")
	if err != nil {
		return fmt.Errorf("serve requer --config: %w", err)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("config inválida: %w", err)
	}

	// --port é opcional; 0 faz o servidor escolher uma porta aleatória
	port := 0
	if portStr, err := parseFlag(args, "--port"); err == nil {
		port, err = strconv.Atoi(portStr)
		if err != nil {
			return fmt.Errorf("--port inválida: %w", err)
		}
	}

	srv, err := server.New(cfg, port)
	if err != nil {
		return fmt.Errorf("erro ao criar servidor: %w", err)
	}

	addr := fmt.Sprintf("http://localhost:%d", srv.Port())

	// Sem TTY (pipe, CI, script): modo texto simples, sem TUI.
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Printf("mockr rodando em %s\n", addr)
		for _, r := range srv.Routes() {
			delay := ""
			if r.Delay > 0 {
				delay = fmt.Sprintf(" (delay %s)", r.Delay)
			}
			fmt.Printf("  %-6s %-30s → %d%s\n", r.Method, r.Path, r.Status, delay)
		}
		return srv.Start()
	}

	// Com TTY: sobe servidor em background, TUI em foreground.
	go func() {
		if err := srv.Start(); err != nil {
			fmt.Printf("servidor encerrado: %v\n", err)
		}
	}()

	return tui.Run(addr, srv)
}

func runStress(args []string) error {
	// Parâmetros comuns
	n := 100
	c := 10
	if v, err := parseFlag(args, "-n"); err == nil {
		if n, err = strconv.Atoi(v); err != nil {
			return fmt.Errorf("-n inválido: %w", err)
		}
	}
	if v, err := parseFlag(args, "-c"); err == nil {
		if c, err = strconv.Atoi(v); err != nil {
			return fmt.Errorf("-c inválido: %w", err)
		}
	}

	// Modo URL direta: --url + --method
	if url, err := parseFlag(args, "--url"); err == nil {
		method := "GET"
		if m, err := parseFlag(args, "--method"); err == nil {
			method = m
		}
		result, err := stress.Run(stress.Plan{
			URL: url, Method: method,
			Requests: n, Concurrency: c,
		})
		if err != nil {
			return err
		}
		stress.Print(result)
		return nil
	}

	// Modo config: sobe servidor interno e testa todas as rotas
	configPath, err := parseFlag(args, "--config")
	if err != nil {
		return fmt.Errorf("stress requer --config ou --url: %w", err)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("config inválida: %w", err)
	}

	srv, err := server.New(cfg, 0)
	if err != nil {
		return fmt.Errorf("erro ao criar servidor: %w", err)
	}

	// Servidor interno em background — sem TUI, saída limpa
	go srv.Start() //nolint:errcheck

	// Aguarda o servidor estar pronto
	addr := fmt.Sprintf("http://localhost:%d", srv.Port())
	if err := waitReady(addr, 3*time.Second); err != nil {
		return fmt.Errorf("servidor não respondeu: %w", err)
	}

	fmt.Printf("mockr stress  •  %s  •  %d req  •  %d concurrent\n\n", addr, n, c)

	for _, r := range srv.Routes() {
		// Substitui parâmetros :param por "1" para gerar uma URL válida
		path := replaceParts(r.Path)
		plan := stress.Plan{
			URL:         addr + path,
			Method:      r.Method,
			Requests:    n,
			Concurrency: c,
		}
		result, err := stress.Run(plan)
		if err != nil {
			fmt.Printf("erro em %s %s: %v\n", r.Method, r.Path, err)
			continue
		}
		stress.Print(result)
	}

	return nil
}

// waitReady tenta GET no addr até receber resposta ou timeout.
func waitReady(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 200 * time.Millisecond}
	for time.Now().Before(deadline) {
		resp, err := client.Get(addr)
		if err == nil {
			resp.Body.Close()
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("timeout após %s", timeout)
}

// replaceParts substitui segmentos :param por "1" para montar uma URL concreta.
func replaceParts(path string) string {
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if len(p) > 0 && p[0] == ':' {
			parts[i] = "1"
		}
	}
	return strings.Join(parts, "/")
}

func runValidate(args []string) error {
	configPath, err := parseFlag(args, "--config")
	if err != nil {
		return fmt.Errorf("validate requer --config: %w", err)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("config inválida: %w", err)
	}

	fmt.Println("✓ config válida")
	fmt.Print(cfg.Summary())
	return nil
}

// runDecrypt descriptografa um payload base64/RSA usando a chave privada definida no YAML.
// Útil para inspecionar requests e responses de APIs Java legadas em tempo de depuração.
//
// Uso: mockr decrypt --config ./mock.yaml --text "base64encodedpayload"
func runDecrypt(args []string) error {
	configPath, err := parseFlag(args, "--config")
	if err != nil {
		return fmt.Errorf("decrypt requer --config: %w", err)
	}
	cipherText, err := parseFlag(args, "--text")
	if err != nil {
		return fmt.Errorf("decrypt requer --text: %w", err)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("config inválida: %w", err)
	}
	if cfg.Crypto == nil || cfg.Crypto.PrivateKey == "" {
		return fmt.Errorf("decrypt requer o bloco crypto.private_key no YAML")
	}

	privKey, err := cryptoutil.LoadPrivateKey(cfg.Crypto.PrivateKey)
	if err != nil {
		return fmt.Errorf("erro ao carregar chave privada: %w", err)
	}

	plainText, err := cryptoutil.Decrypt(cipherText, privKey)
	if err != nil {
		return fmt.Errorf("erro ao descriptografar: %w", err)
	}

	fmt.Println(cryptoutil.PrettyJSON(plainText))
	return nil
}

// runEncrypt criptografa um JSON plaintext usando a chave pública definida no YAML.
// Útil para gerar payloads de teste compatíveis com APIs Java legadas.
//
// Uso: mockr encrypt --config ./mock.yaml --text '{"id":1,"name":"Marco"}'
func runEncrypt(args []string) error {
	configPath, err := parseFlag(args, "--config")
	if err != nil {
		return fmt.Errorf("encrypt requer --config: %w", err)
	}
	plainText, err := parseFlag(args, "--text")
	if err != nil {
		return fmt.Errorf("encrypt requer --text: %w", err)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("config inválida: %w", err)
	}
	if cfg.Crypto == nil || cfg.Crypto.PublicKey == "" {
		return fmt.Errorf("encrypt requer o bloco crypto.public_key no YAML")
	}

	pubKey, err := cryptoutil.LoadPublicKey(cfg.Crypto.PublicKey)
	if err != nil {
		return fmt.Errorf("erro ao carregar chave pública: %w", err)
	}

	cipherText, err := cryptoutil.Encrypt(plainText, pubKey)
	if err != nil {
		return fmt.Errorf("erro ao criptografar: %w", err)
	}

	fmt.Println(cipherText)
	return nil
}

// parseFlag percorre os argumentos procurando uma flag e retorna seu valor.
// Ex: parseFlag(["--config", "mock.yaml", "--port", "8080"], "--config") → "mock.yaml"
//
// Em Go, "range" itera sobre slices igual ao "for (int i...) " do Java,
// mas retornando índice e valor juntos.
func parseFlag(args []string, flag string) (string, error) {
	for i, arg := range args {
		if arg == flag && i+1 < len(args) {
			return args[i+1], nil
		}
	}
	return "", fmt.Errorf("flag %s não encontrada", flag)
}
