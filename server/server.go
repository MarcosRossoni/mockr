package server

// Package server: sobe o servidor HTTP e registra as rotas definidas no YAML.

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"mockr/config"
	"mockr/engine"
)

// RouteInfo expõe os metadados de uma rota para consumo externo (ex: TUI).
type RouteInfo struct {
	Method string
	Path   string
	Status int
	Delay  time.Duration
}

// Routes retorna as informações das rotas registradas.
func (s *Server) Routes() []RouteInfo {
	out := make([]RouteInfo, len(s.routes))
	for i, e := range s.routes {
		out[i] = RouteInfo{
			Method: e.route.Method,
			Path:   e.route.Path,
			Status: e.route.StatusCode,
			Delay:  e.route.Delay,
		}
	}
	return out
}

// RequestLog registra os dados de uma requisição atendida.
type RequestLog struct {
	Method    string
	Path      string
	Status    int
	Latency   time.Duration
	Timestamp time.Time
}

// routeEntry associa uma rota do YAML ao JSON já carregado.
type routeEntry struct {
	route config.Route
	body  *config.ResponseBody
}

// Server mantém a lista de rotas, a porta escolhida e um canal de eventos.
// Implementa http.Handler para poder ser passado ao http.ListenAndServe.
type Server struct {
	routes []routeEntry
	port   int
	// Events recebe um RequestLog a cada requisição atendida.
	// Bufferizado para não bloquear o handler HTTP se o consumidor estiver lento.
	Events chan RequestLog
}

// New carrega todos os JSON de resposta e devolve um Server pronto.
// Se port == 0, escolhe uma porta aleatória disponível.
func New(cfg *config.Config, port int) (*Server, error) {
	if port == 0 {
		var err error
		port, err = freePort()
		if err != nil {
			return nil, fmt.Errorf("não foi possível encontrar porta livre: %w", err)
		}
	}

	s := &Server{
		port:   port,
		Events: make(chan RequestLog, 64),
	}

	for _, route := range cfg.Routes {
		r := route
		body, err := config.LoadResponseBody(r.ResponseFile)
		if err != nil {
			return nil, fmt.Errorf("rota %s %s: %w", r.Method, r.Path, err)
		}
		s.routes = append(s.routes, routeEntry{r, body})
	}

	return s, nil
}

// Start inicia o servidor e bloqueia até encerrar.
func (s *Server) Start() error {
	return http.ListenAndServe(fmt.Sprintf(":%d", s.port), s)
}

// Port retorna a porta escolhida (útil para testes e TUI).
func (s *Server) Port() int { return s.port }

// responseWriter envolve http.ResponseWriter para capturar o status code escrito.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// ServeHTTP implementa http.Handler.
// Extrai o count do primeiro segmento da URL (se for um número inteiro positivo),
// roteia pelo path real e, em GETs com count, retorna um array com N itens gerados.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
	start := time.Now()

	// Extrai o count opcional do prefixo da URL.
	// GET /5/users  → path="/users", count=5
	// POST /5/users → path="/users", count ignorado (não-GET)
	// GET /users    → path="/users", count=0 (comportamento normal)
	path, count := extractCount(r.URL.Path)
	if !strings.EqualFold(r.Method, "GET") {
		count = 0 // count só é significativo em GETs
	}

	pathMatched := false
	for _, e := range s.routes {
		if matchPath(e.route.Path, path) {
			pathMatched = true
			if strings.EqualFold(r.Method, e.route.Method) {
				serveRoute(rw, e.route, e.body, count)
				s.emit(r.Method, path, rw.status, time.Since(start))
				return
			}
		}
	}

	if pathMatched {
		http.Error(rw, "método não permitido", http.StatusMethodNotAllowed)
	} else {
		http.NotFound(rw, r)
	}
	s.emit(r.Method, path, rw.status, time.Since(start))
}

// extractCount verifica se o primeiro segmento do path é um inteiro positivo.
// Se for, devolve o path sem esse segmento e o número; caso contrário, devolve
// o path original e 0.
//
// Exemplos:
//
//	"/5/users"    → "/users", 5
//	"/100/a/b/c"  → "/a/b/c", 100
//	"/users"      → "/users", 0
//	"/users/5"    → "/users/5", 0  (5 não é o primeiro segmento)
func extractCount(path string) (string, int) {
	trimmed := strings.TrimPrefix(path, "/")
	slash := strings.Index(trimmed, "/")

	var first, rest string
	if slash == -1 {
		first = trimmed
		rest = ""
	} else {
		first = trimmed[:slash]
		rest = trimmed[slash:] // inclui a "/" inicial do restante
	}

	n, err := strconv.Atoi(first)
	if err != nil || n <= 0 {
		return path, 0
	}

	if rest == "" {
		return "/", n
	}
	return rest, n
}

// emit envia um RequestLog ao canal sem bloquear.
func (s *Server) emit(method, path string, status int, latency time.Duration) {
	select {
	case s.Events <- RequestLog{
		Method:    method,
		Path:      path,
		Status:    status,
		Latency:   latency,
		Timestamp: time.Now(),
	}:
	default: // descarta se o canal estiver cheio
	}
}

// matchPath verifica se um path real bate com um padrão que pode conter segmentos :param.
// Ex: matchPath("/users/:id", "/users/42") → true
func matchPath(pattern, actual string) bool {
	// Divide ambos os paths em segmentos separados por "/"
	// strings.Split("/users/1", "/") → ["", "users", "1"]
	pp := strings.Split(strings.TrimRight(pattern, "/"), "/")
	ap := strings.Split(strings.TrimRight(actual, "/"), "/")

	if len(pp) != len(ap) {
		return false
	}

	for i := range pp {
		// Segmentos que começam com ":" são curingas — aceitam qualquer valor
		if strings.HasPrefix(pp[i], ":") {
			continue
		}
		if pp[i] != ap[i] {
			return false
		}
	}
	return true
}

// serveRoute aplica delay, renderiza templates e escreve a resposta HTTP.
// count > 0 em GETs: retorna um array JSON com count itens gerados dinamicamente.
func serveRoute(w http.ResponseWriter, r config.Route, body *config.ResponseBody, count int) {
	if r.Delay > 0 {
		time.Sleep(r.Delay)
	}

	var out []byte
	var err error

	if count > 0 {
		out, err = renderMany(body.Raw, count)
	} else {
		out, err = engine.RenderJSON(body.Raw)
	}

	if err != nil {
		http.Error(w, "erro ao serializar resposta", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(r.StatusCode)
	w.Write(out) //nolint:errcheck
}

// renderMany gera um array JSON com count itens.
// Se o template já é um array, usa o primeiro elemento como modelo.
// Se é um objeto, usa o objeto diretamente como modelo.
// Cada item é renderizado independentemente — templates como {{seq}} e {{uuid}}
// produzem valores distintos para cada elemento do array.
func renderMany(raw any, count int) ([]byte, error) {
	template := raw
	if arr, ok := raw.([]any); ok && len(arr) > 0 {
		template = arr[0]
	}

	result := make([]any, count)
	for i := range result {
		result[i] = engine.Render(template)
	}
	return json.MarshalIndent(result, "", "  ")
}

// freePort pede ao SO uma porta TCP disponível.
// Abrimos um listener na porta 0 — o kernel atribui uma porta livre — e fechamos logo.
func freePort() (int, error) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

