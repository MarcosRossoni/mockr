package config

// Package config: responsável por ler e validar o arquivo YAML de configuração
// e os arquivos JSON de exemplo de resposta.

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// --- STRUCTS ---
// Em Go, structs são como classes sem métodos (de início).
// As tags `yaml:"nome"` e `json:"nome"` dizem ao Go como mapear
// campos do arquivo para a struct — igual ao @JsonProperty do Jackson no Java.

// Config representa o arquivo YAML inteiro.
type Config struct {
	Auth   *AuthConfig   `yaml:"auth"`
	Crypto *CryptoConfig `yaml:"crypto"`
	Routes []Route       `yaml:"routes"`
}

// CryptoConfig define as chaves RSA usadas para criptografia/descriptografia de requests e responses.
// Os campos public_key e private_key aceitam caminho para arquivo PEM ou string PEM inline.
type CryptoConfig struct {
	PublicKey  string `yaml:"public_key"`  // chave pública RSA (arquivo .pem ou PEM inline)
	PrivateKey string `yaml:"private_key"` // chave privada RSA (arquivo .pem ou PEM inline)
}

// AuthConfig define o fluxo de autenticação executado antes das rotas com auth: true.
type AuthConfig struct {
	URL     string `yaml:"url"`     // endpoint de autenticação
	Method  string `yaml:"method"`  // método HTTP (padrão: POST)
	Body    string `yaml:"body"`    // arquivo JSON de body para usuário único (suporta faker)
	Users   string `yaml:"users"`   // arquivo JSON array para múltiplos usuários (substitui body)
	Extract string `yaml:"extract"` // dot notation no response: "token", "data.access_token"
	Header  string `yaml:"header"`  // header onde injetar o token: "Authorization", "X-Api-Key"
	Prefix  string `yaml:"prefix"`  // prefixo antes do valor: "Bearer ", "Token ", ou vazio
}

// Route representa uma rota do mock.
type Route struct {
	Method          string            `yaml:"method"`
	Path            string            `yaml:"path"`
	ResponseFile    string            `yaml:"response"`
	BodyFile        string            `yaml:"body"`             // JSON de body para POST/PUT/PATCH (suporta templates faker)
	Headers         map[string]string `yaml:"headers"`          // headers extras enviados pelo mockr request (suporta templates)
	UseAuth         bool              `yaml:"auth"`             // se true, dispara o fluxo auth antes da requisição
	DecryptRequest  bool              `yaml:"decrypt_request"`  // descriptografa o body da request antes de processar templates
	EncryptResponse bool              `yaml:"encrypt_response"` // criptografa a resposta antes de enviar ao cliente
	StatusCode      int               `yaml:"status"`
	Delay           time.Duration     `yaml:"delay"`
}

// ResponseBody armazena o JSON carregado do arquivo de exemplo.
// Usamos "any" (interface{}) porque o JSON pode ter qualquer estrutura.
// Em Go, "any" é um alias moderno para "interface{}" — aceita qualquer tipo.
type ResponseBody struct {
	Raw  any    // dado original parseado do JSON
	File string // caminho do arquivo de origem
}

// --- FUNÇÕES ---

// Load lê o arquivo YAML e retorna a Config populada.
// Retorna (*Config, error) — ponteiro para Config ou erro.
// O asterisco (*) indica ponteiro, igual ao Java quando você passa objetos por referência.
func Load(path string) (*Config, error) {
	// os.ReadFile lê o arquivo inteiro como []byte (slice de bytes)
	data, err := os.ReadFile(path)
	if err != nil {
		// fmt.Errorf com %w "embrulha" o erro original para manter o contexto
		return nil, fmt.Errorf("não foi possível ler %s: %w", path, err)
	}

	// Declaramos cfg como Config vazia e passamos o endereço (&cfg) para o yaml.Unmarshal.
	// O & em Go pega o endereço de uma variável — equivale a passar por referência.
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("yaml inválido em %s: %w", path, err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// LoadResponseBody lê e parseia um arquivo JSON de exemplo.
func LoadResponseBody(path string) (*ResponseBody, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("não foi possível ler response file %s: %w", path, err)
	}

	// json.Unmarshal parseia JSON para "any" — o Go vai criar:
	//   map[string]any para objetos JSON  →  { "id": 1 }
	//   []any para arrays JSON            →  [1, 2, 3]
	//   string, float64, bool para valores primitivos
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("json inválido em %s: %w", path, err)
	}

	return &ResponseBody{Raw: raw, File: path}, nil
}

// ToJSON serializa o ResponseBody de volta para []byte (para enviar na resposta HTTP).
func (r *ResponseBody) ToJSON() ([]byte, error) {
	// json.MarshalIndent gera JSON formatado com indentação
	return json.MarshalIndent(r.Raw, "", "  ")
}

// --- MÉTODO DE VALIDAÇÃO ---
// Em Go, métodos são funções com um "receiver" — o (c *Config) antes do nome.
// É equivalente ao "this" do Java: c é a instância de Config.
func (c *Config) validate() error {
	if len(c.Routes) == 0 {
		return fmt.Errorf("config deve ter pelo menos uma rota")
	}

	for i, route := range c.Routes {
		if err := route.validate(i); err != nil {
			return err
		}
	}

	return nil
}

func (r *Route) validate(index int) error {
	// Slice de strings para acumular erros de validação
	var errs []string

	if r.Method == "" {
		errs = append(errs, "method é obrigatório")
	}
	if r.Path == "" {
		errs = append(errs, "path é obrigatório")
	}
	if r.ResponseFile == "" {
		errs = append(errs, "response é obrigatório")
	}
	if r.StatusCode == 0 {
		// Em Go podemos modificar o campo diretamente pelo receiver
		r.StatusCode = 200
	}

	if len(errs) > 0 {
		return fmt.Errorf("rota[%d]: %s", index, strings.Join(errs, ", "))
	}

	return nil
}

// Summary retorna um resumo da config para exibir no terminal.
func (c *Config) Summary() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "config carregada com %d rota(s):\n", len(c.Routes))
	for _, r := range c.Routes {
		fmt.Fprintf(&sb, "  %-6s %-30s → %s (status %d)\n",
			r.Method, r.Path, r.ResponseFile, r.StatusCode)
	}
	return sb.String()
}
