package stress

// Package stress: executa testes de carga contra uma URL e coleta estatísticas.

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"time"
)

// Plan descreve o que testar.
type Plan struct {
	URL         string
	Method      string
	Requests    int // total de requisições
	Concurrency int // goroutines simultâneas
}

// Result contém as estatísticas coletadas após o teste.
type Result struct {
	Plan      Plan
	Total     int
	Success   int
	Errors    int
	Duration  time.Duration // tempo total do teste
	Latencies []time.Duration
}

// Throughput retorna requisições por segundo.
func (r *Result) Throughput() float64 {
	if r.Duration == 0 {
		return 0
	}
	return float64(r.Total) / r.Duration.Seconds()
}

// Percentile retorna o percentil p (0–100) das latências.
// Requer que as latências já estejam ordenadas (garantido por Run).
func (r *Result) Percentile(p float64) time.Duration {
	if len(r.Latencies) == 0 {
		return 0
	}
	// Índice arredondado para cima, clampado ao último elemento
	idx := int(p/100*float64(len(r.Latencies)+1)) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(r.Latencies) {
		idx = len(r.Latencies) - 1
	}
	return r.Latencies[idx]
}

func (r *Result) Min() time.Duration {
	if len(r.Latencies) == 0 {
		return 0
	}
	return r.Latencies[0]
}

func (r *Result) Max() time.Duration {
	if len(r.Latencies) == 0 {
		return 0
	}
	return r.Latencies[len(r.Latencies)-1]
}

func (r *Result) Mean() time.Duration {
	if len(r.Latencies) == 0 {
		return 0
	}
	var total time.Duration
	for _, l := range r.Latencies {
		total += l
	}
	return total / time.Duration(len(r.Latencies))
}

// --- EXECUÇÃO ---

// sample é o resultado de uma única requisição.
type sample struct {
	latency time.Duration
	err     bool
}

// Run executa o plano de stress e retorna o resultado.
// Usa um worker pool: um canal de jobs alimenta N goroutines simultâneas.
func Run(plan Plan) (*Result, error) {
	if plan.Requests <= 0 {
		return nil, fmt.Errorf("requests deve ser > 0")
	}
	if plan.Concurrency <= 0 {
		plan.Concurrency = 1
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Canal de jobs: cada valor é apenas um sinal para "faz mais uma req"
	jobs := make(chan struct{}, plan.Concurrency)
	// Canal de amostras: cada worker envia o resultado aqui
	samples := make(chan sample, plan.Requests)

	var wg sync.WaitGroup

	// Sobe os workers
	// sync.WaitGroup é o equivalente ao CountDownLatch do Java:
	// Add(n) define o contador, Done() decrementa, Wait() bloqueia até zerar.
	for i := 0; i < plan.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range jobs {
				s := doRequest(client, plan.Method, plan.URL)
				samples <- s
			}
		}()
	}

	start := time.Now()

	// Alimenta o canal de jobs com o total de requisições
	for i := 0; i < plan.Requests; i++ {
		jobs <- struct{}{}
	}
	close(jobs) // sinaliza aos workers que não há mais jobs

	wg.Wait()       // aguarda todos os workers terminarem
	close(samples)  // seguro fechar agora: todos os writes já ocorreram

	elapsed := time.Since(start)

	// Coleta e ordena as latências
	result := &Result{
		Plan:     plan,
		Total:    plan.Requests,
		Duration: elapsed,
	}

	for s := range samples {
		if s.err {
			result.Errors++
		} else {
			result.Success++
			result.Latencies = append(result.Latencies, s.latency)
		}
	}

	// Ordena para poder calcular percentis com acesso por índice
	sort.Slice(result.Latencies, func(i, j int) bool {
		return result.Latencies[i] < result.Latencies[j]
	})

	return result, nil
}

// doRequest executa uma única requisição HTTP e mede a latência.
func doRequest(client *http.Client, method, url string) sample {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return sample{err: true}
	}

	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start)

	if err != nil {
		return sample{err: true}
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body) // drena o body para reusar a conexão TCP

	// Considera erro qualquer status >= 500
	if resp.StatusCode >= 500 {
		return sample{latency: latency, err: true}
	}

	return sample{latency: latency}
}

// --- OUTPUT ---

// Print imprime o resultado formatado no terminal.
func Print(r *Result) {
	fmt.Printf("\n%s %s\n", r.Plan.Method, r.Plan.URL)
	fmt.Println("  " + line(50))
	fmt.Printf("  Requisições:   %d\n", r.Total)
	fmt.Printf("  Concorrência:  %d\n", r.Plan.Concurrency)
	fmt.Printf("  Duração:       %s\n", roundDuration(r.Duration))
	fmt.Printf("  Throughput:    %.1f req/s\n", r.Throughput())
	fmt.Println()
	fmt.Println("  Latência")
	if len(r.Latencies) > 0 {
		fmt.Printf("    p50:    %s\n", roundDuration(r.Percentile(50)))
		fmt.Printf("    p95:    %s\n", roundDuration(r.Percentile(95)))
		fmt.Printf("    p99:    %s\n", roundDuration(r.Percentile(99)))
		fmt.Printf("    min:    %s\n", roundDuration(r.Min()))
		fmt.Printf("    max:    %s\n", roundDuration(r.Max()))
		fmt.Printf("    média:  %s\n", roundDuration(r.Mean()))
	} else {
		fmt.Println("    sem dados (todas as requisições falharam)")
	}
	fmt.Println()
	fmt.Println("  Resultados")
	fmt.Printf("    Sucesso:  %d (%.1f%%)\n", r.Success, pct(r.Success, r.Total))
	fmt.Printf("    Erros:    %d (%.1f%%)\n", r.Errors, pct(r.Errors, r.Total))
	fmt.Println()
}

func pct(part, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total) * 100
}

func roundDuration(d time.Duration) string {
	switch {
	case d < time.Microsecond:
		return fmt.Sprintf("%dns", d.Nanoseconds())
	case d < time.Millisecond:
		return fmt.Sprintf("%dµs", d.Microseconds())
	case d < time.Second:
		return fmt.Sprintf("%dms", d.Milliseconds())
	default:
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
}

func line(n int) string {
	s := ""
	for i := 0; i < n; i++ {
		s += "─"
	}
	return s
}
