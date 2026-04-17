package tui

// Package tui: interface de terminal (TUI) usando Bubble Tea.
// Exibe as rotas configuradas, contador de hits por rota e log de requisições.

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mockr/server"
)

// --- ESTILOS ---
// lipgloss é a biblioteca de estilo de terminal da Charm.
// Funciona como CSS para o terminal.

var (
	styleBold    = lipgloss.NewStyle().Bold(true)
	styleGreen   = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	styleYellow  = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	styleRed     = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	styleDim     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleCyan    = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	styleHeader  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	styleBorder  = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1)
)

const maxLogs = 15

// --- MENSAGENS ---
// Em Bubble Tea, mensagens (tea.Msg) são o único meio de atualizar o modelo.
// Criamos um tipo próprio para carregar um RequestLog recebido do servidor.

type requestMsg server.RequestLog

// --- MODELO ---
// O modelo (Model) é o estado da TUI.
// Bubble Tea segue a arquitetura Elm: Model + Update + View.

type routeStat struct {
	method string
	path   string
	status int
	hits   int
}

type Model struct {
	addr   string            // ex: "http://localhost:9090"
	routes []routeStat       // rotas com contador de hits
	logs   []server.RequestLog // últimas N requisições
	events <-chan server.RequestLog
	quitting bool
}

// --- INIT ---
// Init é chamado uma vez no início. Retorna um tea.Cmd opcional.
// Usamos para disparar o primeiro "escuta o canal".

func (m Model) Init() tea.Cmd {
	return waitForEvent(m.events)
}

// waitForEvent retorna um tea.Cmd que bloqueia até chegar um evento no canal.
// tea.Cmd é uma função que roda em goroutine separada e devolve uma tea.Msg.
func waitForEvent(ch <-chan server.RequestLog) tea.Cmd {
	return func() tea.Msg {
		return requestMsg(<-ch)
	}
}

// --- UPDATE ---
// Update recebe mensagens e devolve o modelo atualizado + próximo Cmd.

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// Teclas de saída
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}

	// Novo log de requisição chegou do servidor
	case requestMsg:
		log := server.RequestLog(msg)

		// Atualiza contador da rota correspondente
		for i := range m.routes {
			if strings.EqualFold(m.routes[i].method, log.Method) &&
				m.routes[i].path == log.Path {
				m.routes[i].hits++
				break
			}
		}

		// Adiciona ao log circular (máximo maxLogs entradas)
		m.logs = append([]server.RequestLog{log}, m.logs...)
		if len(m.logs) > maxLogs {
			m.logs = m.logs[:maxLogs]
		}

		// Agenda próxima escuta no canal
		return m, waitForEvent(m.events)
	}

	return m, nil
}

// --- VIEW ---
// View renderiza o estado atual como string. Bubble Tea imprime no terminal.

func (m Model) View() string {
	if m.quitting {
		return "mockr encerrado.\n"
	}

	var b strings.Builder

	// Cabeçalho
	header := fmt.Sprintf("  mockr  •  %s  •  %d rota(s)",
		styleCyan.Render(m.addr), len(m.routes))
	b.WriteString(styleBorder.Render(header))
	b.WriteString("\n\n")

	// Seção de rotas
	b.WriteString(styleHeader.Render("  Rotas"))
	b.WriteString("\n")
	b.WriteString(styleDim.Render("  " + strings.Repeat("─", 55)))
	b.WriteString("\n")
	for _, r := range m.routes {
		hits := styleDim.Render(fmt.Sprintf("%4d req", r.hits))
		if r.hits > 0 {
			hits = fmt.Sprintf("%4d req", r.hits)
		}
		line := fmt.Sprintf("  %-6s %-30s %s   %s",
			methodColor(r.method),
			r.path,
			styleDim.Render(fmt.Sprintf("%d", r.status)),
			hits,
		)
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Seção de log de requisições
	b.WriteString(styleHeader.Render("  Log"))
	b.WriteString("\n")
	b.WriteString(styleDim.Render("  " + strings.Repeat("─", 55)))
	b.WriteString("\n")

	if len(m.logs) == 0 {
		b.WriteString(styleDim.Render("  aguardando requisições...\n"))
	} else {
		for _, l := range m.logs {
			b.WriteString(formatLog(l))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(styleDim.Render("  q para sair"))
	b.WriteString("\n")

	return b.String()
}

// formatLog formata uma linha de log.
func formatLog(l server.RequestLog) string {
	ts := styleDim.Render(l.Timestamp.Format("15:04:05"))
	method := methodColor(l.Method)
	path := fmt.Sprintf("%-30s", l.Path)
	status := statusColor(l.Status)
	latency := styleDim.Render(fmt.Sprintf("%6s", roundLatency(l.Latency)))

	return fmt.Sprintf("  %s  %-6s %s  %s  %s", ts, method, path, status, latency)
}

// roundLatency formata a latência de forma legível.
func roundLatency(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	return fmt.Sprintf("%dms", d.Milliseconds())
}

func methodColor(m string) string {
	switch strings.ToUpper(m) {
	case "GET":
		return styleGreen.Render(fmt.Sprintf("%-6s", m))
	case "POST":
		return styleCyan.Render(fmt.Sprintf("%-6s", m))
	case "PUT", "PATCH":
		return styleYellow.Render(fmt.Sprintf("%-6s", m))
	case "DELETE":
		return styleRed.Render(fmt.Sprintf("%-6s", m))
	default:
		return styleBold.Render(fmt.Sprintf("%-6s", m))
	}
}

func statusColor(code int) string {
	s := fmt.Sprintf("%d", code)
	switch {
	case code >= 500:
		return styleRed.Render(s)
	case code >= 400:
		return styleYellow.Render(s)
	default:
		return styleGreen.Render(s)
	}
}

// --- ENTRY POINT ---

// Run inicia a TUI. Recebe o endereço exibido, o servidor (para o canal de eventos)
// e as rotas do YAML já carregadas (via srv.Routes()).
func Run(addr string, srv *server.Server) error {
	stats := make([]routeStat, 0, len(srv.Routes()))
	for _, r := range srv.Routes() {
		stats = append(stats, routeStat{
			method: r.Method,
			path:   r.Path,
			status: r.Status,
		})
	}

	m := Model{
		addr:   addr,
		routes: stats,
		events: srv.Events,
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
