# mockr — Guia de Uso

mockr é uma ferramenta de terminal para subir servidores HTTP mock durante o desenvolvimento e testes de stress. Substitui soluções como Postman Mock Server sem limitações de requisições.

---

## Sumário

1. [Instalação](#instalação)
2. [Início rápido](#início-rápido)
3. [Arquivo de configuração](#arquivo-de-configuração)
4. [Comandos](#comandos)
   - [serve](#serve)
   - [validate](#validate)
   - [stress](#stress)
5. [Templates dinâmicos](#templates-dinâmicos)
6. [Interface TUI](#interface-tui)
7. [Exemplos práticos](#exemplos-práticos)

---

## Instalação

### Pré-requisito

- Go 1.21 ou superior instalado

### Compilar o binário

```bash
git clone <repositório>
cd mockr
CGO_ENABLED=0 go build -o mockr .
```

> **Nota macOS (Sequoia / 26.x beta):** use sempre `CGO_ENABLED=0` ao compilar. Um bug do dyld nessas versões impede a execução de binários sem essa flag.

Após compilar, o binário `mockr` fica na pasta atual. Use `./mockr` para executá-lo:

```bash
./mockr serve --config ./examples/mock.yaml
```

Para usar o comando `mockr` sem o `./` de qualquer diretório, mova o binário para um diretório no seu `$PATH`:

```bash
mv mockr /usr/local/bin/mockr
```

---

## Início rápido

**1. Crie o arquivo de configuração** (ou use o exemplo incluso):

```yaml
# mock.yaml
routes:
  - method: GET
    path: /users
    response: ./users.json
    status: 200

  - method: POST
    path: /users
    response: ./user.json
    status: 201
```

**2. Crie o arquivo de resposta JSON:**

```json
// user.json
{
  "id": "{{seq}}",
  "name": "{{faker.name}}",
  "email": "{{faker.email}}"
}
```

**3. Compile e suba o servidor:**

```bash
CGO_ENABLED=0 go build -o mockr .
./mockr serve --config ./mock.yaml
```

O servidor escolhe uma porta aleatória e abre a interface TUI no terminal. Para usar uma porta fixa:

```bash
./mockr serve --config ./mock.yaml --port 8080
```

**4. Faça requisições:**

```bash
curl http://localhost:8080/users
curl -X POST http://localhost:8080/users
```

---

## Arquivo de configuração

O arquivo YAML define todas as rotas do servidor mock.

```yaml
routes:
  - method: GET
    path: /products
    response: ./responses/products.json
    status: 200
    delay: 100ms

  - method: GET
    path: /products/:id
    response: ./responses/product.json
    status: 200

  - method: POST
    path: /products
    response: ./responses/product.json
    status: 201

  - method: PUT
    path: /products/:id
    response: ./responses/product.json
    status: 200

  - method: DELETE
    path: /products/:id
    response: ./responses/deleted.json
    status: 200
```

### Campos de cada rota

| Campo | Obrigatório | Descrição |
|---|---|---|
| `method` | Sim | Método HTTP: `GET`, `POST`, `PUT`, `PATCH`, `DELETE` |
| `path` | Sim | Caminho da rota. Use `:param` para parâmetros dinâmicos |
| `response` | Sim | Caminho para o arquivo JSON de resposta |
| `status` | Não | Status HTTP retornado. Padrão: `200` |
| `delay` | Não | Atraso artificial por requisição. Ex: `50ms`, `1s`, `200ms` |

### Parâmetros de path

Use `:nome` para criar segmentos dinâmicos:

```yaml
path: /users/:id           # bate com /users/1, /users/abc, /users/xyz
path: /orgs/:org/users/:id # bate com /orgs/acme/users/42
```

---

## Comandos

### serve

Sobe o servidor HTTP mock.

```bash
./mockr serve --config <arquivo.yaml> [--port <porta>]
```

| Flag | Obrigatório | Descrição |
|---|---|---|
| `--config` | Sim | Caminho para o arquivo YAML de configuração |
| `--port` | Não | Porta do servidor. Se omitida, uma porta aleatória é escolhida |

**Exemplos:**

```bash
# Porta aleatória (exibida na TUI ao iniciar)
./mockr serve --config ./mock.yaml

# Porta fixa
./mockr serve --config ./mock.yaml --port 8080

# Porta fixa para uso em CI/CD (sem TUI, saída em texto)
./mockr serve --config ./mock.yaml --port 3000
```

> Quando rodando em ambiente sem terminal interativo (CI, scripts, pipes), o mockr detecta automaticamente a ausência de TTY e usa saída em texto simples, sem TUI.

---

### validate

Valida o arquivo de configuração sem subir o servidor. Útil para checar erros antes de um deploy ou rodar em CI.

```bash
./mockr validate --config <arquivo.yaml>
```

**Exemplo de saída:**

```
✓ config válida
config carregada com 3 rota(s):
  GET    /users                         → ./examples/users.json (status 200)
  GET    /users/:id                     → ./examples/user.json (status 200)
  POST   /users                         → ./examples/user.json (status 201)
```

---

### stress

Executa um teste de carga e exibe estatísticas de latência e throughput.

**Modo 1 — testar todas as rotas do config (servidor interno):**

```bash
./mockr stress --config <arquivo.yaml> [-n <requisições>] [-c <concorrência>]
```

O mockr sobe um servidor interno automaticamente, executa o teste em cada rota e exibe os resultados.

**Modo 2 — testar uma URL externa:**

```bash
./mockr stress --url <url> --method <METHOD> [-n <requisições>] [-c <concorrência>]
```

| Flag | Padrão | Descrição |
|---|---|---|
| `--config` | — | Arquivo de configuração (modo 1) |
| `--url` | — | URL alvo (modo 2) |
| `--method` | `GET` | Método HTTP (modo 2) |
| `-n` | `100` | Total de requisições |
| `-c` | `10` | Número de requisições simultâneas (goroutines) |

**Exemplos:**

```bash
# Testa todas as rotas com 500 reqs e 20 workers simultâneos
./mockr stress --config ./mock.yaml -n 500 -c 20

# Testa uma URL já rodando com 1000 reqs e 50 workers
./mockr stress --url http://localhost:8080/users --method GET -n 1000 -c 50
```

**Exemplo de saída:**

```
mockr stress  •  http://localhost:52341  •  500 req  •  20 concurrent

GET http://localhost:52341/users
  ──────────────────────────────────────────────────
  Requisições:   500
  Concorrência:  20
  Duração:       1.32s
  Throughput:    379.4 req/s

  Latência
    p50:    51ms
    p95:    54ms
    p99:    56ms
    min:    50ms
    max:    61ms
    média:  51ms

  Resultados
    Sucesso:  500 (100.0%)
    Erros:      0 (0.0%)
```

> Requisições com status HTTP >= 500 são contadas como erro. Status 4xx são considerados sucesso (resposta válida do servidor).

---

## Controle de volume por URL

Em requisições `GET`, você pode informar quantos itens quer receber diretamente na URL, colocando um número inteiro no **primeiro segmento do path**:

```
GET /{count}/{rota}
```

O mockr extrai o número, roteia para a rota correta e retorna um array JSON com exatamente `{count}` itens gerados dinamicamente.

**Exemplos:**

```bash
# Retorna array com 5 usuários
curl http://localhost:8080/5/users

# Retorna array com 100 produtos (bom para testar paginação)
curl http://localhost:8080/100/products

# Funciona com parâmetros de path também
curl http://localhost:8080/10/users/42
```

**Exemplo de resposta para `GET /3/users`:**

```json
[
  { "id": 1, "name": "Mariana Lopes", "email": "mariana.lopes@gmail.com" },
  { "id": 2, "name": "Carlos Silva",  "email": "carlos.silva@outlook.com" },
  { "id": 3, "name": "Ana Ferreira",  "email": "ana.ferreira@yahoo.com" }
]
```

Cada item é gerado independentemente — templates como `{{seq}}`, `{{uuid}}` e `{{faker.name}}` produzem valores diferentes para cada elemento.

**Comportamento em outros métodos:**

Em `POST`, `PUT`, `PATCH` e `DELETE`, o número é **ignorado** — o mock roteia normalmente e retorna a resposta padrão (item único). Isso permite que sua API bata em `POST /5/users` sem configuração adicional no mock.

```bash
# Resposta normal de POST (objeto único, não array)
curl -X POST http://localhost:8080/5/users
```

> **Atenção:** o número deve estar sempre no **primeiro** segmento. `/users/5` é interpretado como a rota `/users/:id`, não como "5 usuários".

---

## Templates dinâmicos

Os arquivos JSON de resposta suportam templates `{{...}}` que geram dados novos a cada requisição. Não é necessário nenhuma configuração extra — basta usar os tokens no JSON.

### Tipos e comportamento

Quando um template ocupa **o valor inteiro** de um campo, o tipo nativo é preservado no JSON:

```json
{ "id": "{{seq}}" }
```
→ `{ "id": 1 }` — o campo `id` é um inteiro real, não uma string.

Quando um template está **embutido em texto**, o resultado é sempre string:

```json
{ "label": "user-{{seq}}" }
```
→ `{ "label": "user-1" }`

### Referência completa de templates

**Identificadores**

| Template | Tipo | Exemplo de saída |
|---|---|---|
| `{{uuid}}` | string | `"550e8400-e29b-41d4-a716-446655440000"` |
| `{{seq}}` | int | `1`, `2`, `3`... (global, incrementa a cada req) |

**Data e hora**

| Template | Tipo | Exemplo de saída |
|---|---|---|
| `{{now}}` | string | `"2026-04-17T13:00:40Z"` |
| `{{now.unix}}` | int64 | `1745492440` |

**Números e booleanos**

| Template | Tipo | Exemplo de saída |
|---|---|---|
| `{{rand.int}}` | int | `4821` |
| `{{rand.bool}}` | bool | `true` ou `false` |
| `{{rand.float}}` | float64 | `0.73` |

**Dados faker**

| Template | Tipo | Exemplo de saída |
|---|---|---|
| `{{faker.name}}` | string | `"Mariana Lopes"` |
| `{{faker.first_name}}` | string | `"Mariana"` |
| `{{faker.last_name}}` | string | `"Lopes"` |
| `{{faker.email}}` | string | `"mariana.lopes@gmail.com"` |
| `{{faker.username}}` | string | `"mariana423"` |
| `{{faker.phone}}` | string | `"+55 11 91234-5678"` |
| `{{faker.word}}` | string | `"lorem"` |
| `{{faker.sentence}}` | string | `"Lorem ipsum dolor amet consectetur."` |

### Exemplo completo de JSON com templates

```json
{
  "id": "{{seq}}",
  "uuid": "{{uuid}}",
  "name": "{{faker.name}}",
  "email": "{{faker.email}}",
  "username": "{{faker.username}}",
  "phone": "{{faker.phone}}",
  "active": "{{rand.bool}}",
  "score": "{{rand.float}}",
  "createdAt": "{{now}}",
  "bio": "{{faker.sentence}}"
}
```

Cada chamada a esse endpoint retorna um objeto diferente.

---

## Interface TUI

Quando rodando em um terminal interativo, o `mockr serve` abre uma interface visual com:

- Endereço e porta do servidor
- Lista de rotas com contador de hits atualizado em tempo real
- Log das últimas 15 requisições com timestamp, método, path, status e latência

```
╭─ mockr  •  http://localhost:8080  •  3 rota(s) ──────╮

  Rotas
  ────────────────────────────────────────────────────
  GET    /users                  200    12 req
  GET    /users/:id              200     3 req
  POST   /users                  201     1 req

  Log
  ────────────────────────────────────────────────────
  13:00:40  GET    /users/1      200   1ms
  13:00:39  POST   /users        201  101ms
  13:00:38  GET    /users        200   52ms

  q para sair
```

Pressione `q` ou `Ctrl+C` para encerrar o servidor.

---

## Exemplos práticos

### Simular uma API REST de produtos

**`mock.yaml`**
```yaml
routes:
  - method: GET
    path: /products
    response: ./products.json
    status: 200

  - method: GET
    path: /products/:id
    response: ./product.json
    status: 200

  - method: POST
    path: /products
    response: ./product.json
    status: 201

  - method: DELETE
    path: /products/:id
    response: ./deleted.json
    status: 200
```

**`product.json`**
```json
{
  "id": "{{seq}}",
  "uuid": "{{uuid}}",
  "name": "{{faker.word}}",
  "price": "{{rand.float}}",
  "inStock": "{{rand.bool}}",
  "createdAt": "{{now}}"
}
```

### Simular latência de rede

Use `delay` para simular APIs lentas:

```yaml
routes:
  - method: GET
    path: /slow-endpoint
    response: ./data.json
    status: 200
    delay: 2s   # simula 2 segundos de latência

  - method: GET
    path: /fast-endpoint
    response: ./data.json
    status: 200
    delay: 10ms
```

### Stress test antes de integrar com a API real

```bash
# Suba o mock em um terminal
./mockr serve --config ./mock.yaml --port 8080

# Em outro terminal, rode o stress test
./mockr stress --url http://localhost:8080/products --method GET -n 2000 -c 100
```

### Usar em CI/CD para testes automatizados

```bash
# Sobe o mock em background (sem TTY, usa saída texto)
./mockr serve --config ./mock.yaml --port 3000 &
MOCK_PID=$!

# Aguarda estar pronto
sleep 1

# Roda os testes apontando para o mock
npm test

# Encerra o mock
kill $MOCK_PID
```
