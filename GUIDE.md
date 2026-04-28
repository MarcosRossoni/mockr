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
   - [request](#request)
   - [decrypt / encrypt](#decrypt--encrypt)
5. [Criptografia RSA](#criptografia-rsa)
6. [Templates dinâmicos](#templates-dinâmicos)
7. [Respostas de erro](#respostas-de-erro)
8. [Interface TUI](#interface-tui)
9. [Exemplos práticos](#exemplos-práticos)

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
    response: ./user_created.json
    body: ./user.json        # JSON enviado ao disparar mockr request
    status: 201
    headers:
      Authorization: "Bearer meu-token"
      X-Request-ID: "{{uuid}}"
      X-Timestamp: "{{now.unix}}"
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
| `body` | Não | Caminho para o JSON de body de saída (usado pelo `mockr request`). Suporta templates faker. Aplicável apenas em POST, PUT e PATCH |
| `headers` | Não | Mapa de headers extras enviados pelo `mockr request`. Valores suportam templates faker |
| `auth` | Não | Se `true`, executa o fluxo de autenticação do bloco `auth` raiz antes de disparar |
| `status` | Não | Status HTTP retornado. Padrão: `200` |
| `delay` | Não | Atraso artificial por requisição. Ex: `50ms`, `1s`, `200ms` |

### Parâmetros de path

Use `:nome` para criar segmentos dinâmicos:

```yaml
path: /users/:id           # bate com /users/1, /users/abc, /users/xyz
path: /orgs/:org/users/:id # bate com /orgs/acme/users/42
```

### Query params

O roteador ignora query params ao fazer o match da rota — `/users?page=2` bate normalmente com a rota `/users`. Os parâmetros ficam disponíveis nos templates via `{{query.*}}`:

```bash
curl "http://localhost:8080/users?page=2&limit=10"
# roteia para a rota GET /users
# {{query.page}} → "2", {{query.limit}} → "10"
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

### request

Dispara uma ou mais requisições HTTP e exibe a resposta formatada no terminal. Ideal para testar rapidamente uma rota ou integrar com uma API real usando dados faker.

**Modo mock interno** — sobe o servidor e dispara contra ele:

```bash
./mockr request --config <arquivo.yaml> --path <path> [--method <METHOD>] [--repeat <n>]
```

**Modo URL externa** — dispara diretamente na URL informada, sem subir mock:

```bash
./mockr request --config <arquivo.yaml> --url <url> --method <METHOD> [--path <path>] [--repeat <n>]
```

| Flag | Obrigatório | Padrão | Descrição |
|---|---|---|---|
| `--config` | Sim | — | Arquivo YAML com as rotas |
| `--path` | Sim* | — | Path da rota a chamar (*dispensável com `--url` se a rota não tiver body) |
| `--method` | Não | `GET` | Método HTTP |
| `--url` | Não | — | URL externa. Se ausente, usa o mock interno |
| `--repeat` | Não | `1` | Número de vezes que a requisição é disparada |

**Como os headers são configurados:**

Defina o campo `headers` na rota do YAML com um mapa chave/valor. Os headers são enviados em toda requisição disparada pelo `mockr request` para aquela rota. Valores aceitam qualquer template faker:

```yaml
routes:
  - method: POST
    path: /users
    response: ./user_created.json
    body: ./user.json
    status: 201
    headers:
      Authorization: "Bearer meu-token-secreto"
      X-User-Secret: "abc123"
      X-Pass-Secret: "xyz789"
      X-Request-ID: "{{uuid}}"       # UUID único por requisição
      X-Timestamp: "{{now.unix}}"    # epoch atual
      X-Tenant: "acme"
```

Rotas sem `headers` não enviam nenhum header extra. O `Content-Type: application/json` é sempre adicionado automaticamente quando há body — mas pode ser sobrescrito declarando-o explicitamente no mapa.

O output do `mockr request` exibe os headers enviados com os valores já resolvidos:

```
POST http://minha-api.com/users  →  201  (45ms)
  headers enviados:
    Authorization: Bearer meu-token-secreto
    X-Request-ID: 79d166ab-9e06-410b-89d2-cb33250accbf
    X-Timestamp: 1745492440
    X-Tenant: acme
```

**Como o body é resolvido:**

O comando localiza a rota correspondente no config pelo `--method` + `--path`. Se a rota tem o campo `body` definido e o método é POST, PUT ou PATCH, o JSON é carregado, os templates faker são resolvidos e o payload é enviado. Cada `--repeat` gera um body novo com dados distintos.

---

**Autenticação automática**

Quando a rota declara `auth: true`, o `mockr request` dispara o fluxo de login antes de qualquer requisição, extrai o token da resposta e o injeta automaticamente no header configurado.

**Passo 1 — defina o bloco `auth` na raiz do YAML:**

```yaml
auth:
  url: http://minha-api.com/auth
  method: POST
  body: ./examples/auth_body.json   # credenciais enviadas no login
  extract: "data.token"             # dot notation no JSON de resposta
  header: "Authorization"           # header onde o token é injetado
  prefix: "Bearer "                 # prefixo antes do valor (opcional)
```

**Passo 2 — marque as rotas que precisam de auth:**

```yaml
routes:
  - method: POST
    path: /orders
    response: ./examples/order.json
    body: ./examples/order.json
    status: 201
    auth: true                      # dispara o login antes desta rota
```

O token é buscado **uma única vez** antes do loop de `--repeat` e reutilizado em todas as requisições — não há novo login a cada disparo.

**`extract` com dot notation** — acessa campos aninhados no JSON de resposta:

```yaml
extract: "token"              # resposta: { "token": "abc" }
extract: "data.token"         # resposta: { "data": { "token": "abc" } }
extract: "auth.access_token"  # resposta: { "auth": { "access_token": "abc" } }
```

---

**Múltiplos usuários simultâneos**

Para simular N usuários autenticados ao mesmo tempo — cada um com seu próprio token — substitua `body` por `users` no bloco `auth`, apontando para um array JSON de credenciais:

```yaml
auth:
  url: http://minha-api.com/auth
  method: POST
  users: ./examples/auth_users.json   # array de credenciais, uma por usuário
  extract: "data.token"
  header: "Authorization"
  prefix: "Bearer "
```

`auth_users.json`:
```json
[
  { "username": "alice", "password": "pass_alice" },
  { "username": "bob",   "password": "pass_bob"   },
  { "username": "carol", "password": "pass_carol" },
  { "username": "dave",  "password": "pass_dave"  },
  { "username": "eve",   "password": "pass_eve"   }
]
```

O mockr autentica todos os usuários **em paralelo** na inicialização. Durante o `--repeat`, cada requisição sorteia um usuário aleatoriamente e usa o token correspondente:

```
  auth → autenticando 5 usuário(s)...
  auth ✓  alice | bob | carol | dave | eve  (5 tokens prontos)

── requisição 1/10  [carol]
POST http://minha-api.com/orders  →  201  (23ms)
  headers enviados:
    Authorization: Bearer c22b7b7a-b56c-4867-8093-858b088e0493

── requisição 2/10  [alice]
POST http://minha-api.com/orders  →  201  (19ms)
  headers enviados:
    Authorization: Bearer fa9ef575-ced3-456e-839a-c8df533199dc
```

O `extractLabel` detecta automaticamente o campo de identificação do usuário — tenta `username`, `email`, `user`, `login`, `name` (nessa ordem). Se nenhum for encontrado, usa `user1`, `user2`...

**Exemplos:**

```bash
# GET simples contra o mock
./mockr request --config ./mock.yaml --path /users

# POST contra o mock — body lido do campo body da rota, faker resolvido
./mockr request --config ./mock.yaml --path /users --method POST

# 5 POSTs com dados faker diferentes em cada disparo
./mockr request --config ./mock.yaml --path /users --method POST --repeat 5

# POST direto na sua API — body ainda vem do config
./mockr request --config ./mock.yaml --url http://minha-api.com/users --method POST --path /users
```

**Exemplo de saída:**

```
POST http://localhost:52981/users  →  201  (102ms)

{
  "id": "bb44cca7-268a-42fd-a946-0f4e6b5f3cc3",
  "name": "Helena Silva",
  "email": "helena.silva@gmail.com",
  "phone": "+55 11 97432-8812",
  "createdAt": "2026-04-19T14:32:01Z"
}
```

O status é colorido: verde para 2xx, amarelo para 3xx, vermelho para 4xx/5xx.

---

### decrypt / encrypt

Utilitários para inspecionar e gerar payloads criptografados, compatíveis com a lógica Java legada (`URLEncode → RSA/PKCS1v15 chunks → Base64`).

**decrypt** — recebe um payload base64/RSA e exibe o JSON descriptografado com indentação:

```bash
./mockr decrypt --config ./mock.yaml --text "SGVsbG8gV29ybGQ..."
```

**encrypt** — recebe um JSON plaintext e exibe o payload base64 pronto para envio:

```bash
./mockr encrypt --config ./mock.yaml --text '{"id":1,"name":"Marco"}'
```

| Flag | Obrigatório | Descrição |
|---|---|---|
| `--config` | Sim | Arquivo YAML com o bloco `crypto` configurado |
| `--text` | Sim | Payload a processar — base64 para `decrypt`; JSON para `encrypt` |

Pré-requisito no YAML:
- `decrypt` → precisa de `crypto.private_key`
- `encrypt` → precisa de `crypto.public_key`

---

## Criptografia RSA

O mockr suporta criptografia RSA compatível com a implementação Java legada. A sequência de operações espelha exatamente o que o Java executa:

| Operação | Java | mockr (Go) |
|---|---|---|
| Encode antes de criptografar | `URLEncoder.encode(json, UTF-8)` | `url.QueryEscape` |
| Encrypt em chunks | `cipher.doFinal(data, offset, len)` com PKCS1v15 | `rsa.EncryptPKCS1v15` em loop |
| Encode final | `Base64.getEncoder().encodeToString` | `base64.StdEncoding` |
| Chunk encrypt | `ENCRYPT_CHUNK_SIZE` (ex: 245) | `keySize - 11` (calculado da chave) |
| Chunk decrypt | `DECRYPT_CHUNK_SIZE` (ex: 256) | `keySize` (calculado da chave) |

O tamanho dos chunks é derivado automaticamente do tamanho da chave — sem necessidade de configuração manual.

### Configuração no YAML

```yaml
crypto:
  public_key: ./keys/public.pem    # chave pública — usada por encrypt_response e mockr encrypt
  private_key: ./keys/private.pem  # chave privada — usada por decrypt_request e mockr decrypt

routes:
  - method: POST
    path: /api/pagamentos
    response: ./examples/pagamento.json
    status: 200
    decrypt_request: true    # body recebido (base64) é descriptografado antes de processar {{body.*}}
    encrypt_response: true   # resposta JSON é criptografada e enviada como base64 (Content-Type: text/plain)
```

Os campos `public_key` e `private_key` aceitam dois formatos:
- **Caminho de arquivo** — relativo ao diretório onde `mockr` é executado (ex: `./keys/public.pem`)
- **PEM inline** — string PEM colada diretamente no YAML, detectada pelo prefixo `-----BEGIN`

### Flags por rota

| Flag | Requer | Descrição |
|---|---|---|
| `decrypt_request: true` | `crypto.private_key` | Descriptografa o body da request (base64 → JSON) antes de processar templates `{{body.*}}` |
| `encrypt_response: true` | `crypto.public_key` | Criptografa a resposta JSON (JSON → base64) antes de enviar. `Content-Type` muda para `text/plain` |

Quando `decrypt_request` falha (payload inválido ou chave errada), o body original é usado sem modificação — o handler não retorna erro para não bloquear o fluxo.

### Formato dos arquivos PEM

Arquivos PEM são texto puro com cabeçalho, conteúdo em Base64 e rodapé. O mockr detecta automaticamente os formatos abaixo.

---

**Chave pública — PKIX / SubjectPublicKeyInfo** (`-----BEGIN PUBLIC KEY-----`)

Formato padrão gerado pelo Java (`KeyFactory`, `X509EncodedKeySpec`) e pelo OpenSSL. Recomendado para interoperabilidade.

```
-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAwRydvlAy+hAmWHNjIVfY
o9zaul6+2aqI1g4xXjYX1rrl/bzvMj0snznLV2elgjRv80qLuNX2sSJGu2t+EHfH
GAbcwVZLbsfstEpmB2Cjghnd2DPudOc3e6oSQmrhqB6/1xIjBbwvjjJmXjHgCb1X
tXHXXcblm1XhlIT/RKuVC6zorrCHyzpnFFIdASYCaIpXQ/uHUg+YgAMIvEAgLl3B
eaJS8QkplA1KS3Vya2TzPBnFZlPwW8LRoB0jTIztLVIE+KAc0pqebdHK2KA5LmOD
gJPYtT6fY1ea3VyuWih4h4fOSt7SswfkGehY/r0OfLwrr4F0fIi52eFzliD4AlkL
awIDAQAB
-----END PUBLIC KEY-----
```

> **Atenção:** a chave acima é um exemplo gerado para ilustração de formato — não a utilize em produção.

---

**Chave privada — PKCS8** (`-----BEGIN PRIVATE KEY-----`)

Formato moderno, gerado por Java (`PKCS8EncodedKeySpec`) e pelo OpenSSL com `-pkcs8`. Suportado pelo mockr com prioridade sobre PKCS1.

```
-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQDBHJ2+UDL6ECZY
c2MhV9ij3Nq6Xr7ZqojWDjFeNhfWuuX9vO8yPSyfOctXZ6WCNG/zSou41faxIka7
a34Qd8cYBtzBVktux+y0SmYHYKOCGd3YM+505zd7qhJCauGoHr/XEiMFvC+OMmZe
MeAJvVe1cdddxuWbVeGUhP9Eq5ULrOiusIfLOmcUUh0BJgJoildD+4dSD5iAAwi8
QCAuXcF5olLxCSmUDUpLdXJrZPM8GcVmU/BbwtGgHSNMjO0tUgT4oBzSmp5t0crY
oDkuY4OAk9i1Pp9jV5rdXK5aKHiHh85K3tKzB+QZ6Fj+vQ58vCuvgXR8iLnZ4XOW
IPgCWQtrAgMBAAECggEAGSjbynBTe+3/uPVrnGwADzVWkAKJFf3Ybx8RQoeEqpbW
IS1s8pwyxJn4iq43y5N+ZHG5a38A8YoYMhHqrkWUvGYke9H86PWGOuUYjuyB0REC
rMjzT+rR+TwqvdmEBe3fq6sywewHPB0H5kzUNiCjnUG0QAUKZCcXyUxcn7tFI2yM
uCc1RsrJxFE/scEsLbhQG79DDPdOu3B70Vb9lkdJ5uxIOg5ROmNS0we7OtR1zyyo
vN1vKWuC1kz3tK4jqMlQvpOdNmOgBxrQDTW5pvXLFUp7xQEzAX/hAWC0SWd77NOg
yWXYmWVl8DhZX494QAOGO8lrGxL+DN6SCZWICeOXjQKBgQD+1qKLr9rRWMGV4xgH
zakGk5177ZU4qmDrdWe+5BS+WvgLCMGMg3kD45AUY/QN7yDUJiTyYS/641gdWVUH
qK9iU7H3Qu4lwSj970IKbAmZwaSTks3O7gUnTcf4LYeWbVX3LaJAC4TOY+1jeN0u
zF87DCanfsCO9I3td9BbxvoLhQKBgQDB/fQwAWg7nntQtxnWuE69NNiWHJX3eawm
i+JZ6BVZGfPWPGwnI9dLBwXf9BysKo3hPVwwXh+FW3gqIjwJ8sJmcaaKmZZsFxWw
l9LvpE+gxHqy5f+K5PpZf7qtvwWNaMaIJ/8BVw5LpQgtov3YqJ6XwziI40ZnA3gq
HU71JT2WLwKBgFZyyUFJrplF2qXWG9jJ04T/nnTj67MFWUuG0GLZTJAmm+5iUgYw
s2S2VbIVFj2D2UceiXoPrg8WAFY0b7vhZV2dtsmCUvdLx3QdLfILJFaccg/9xQNl
iyoCIMv35f8JqBfpyxiez7aT/3u3DJFLkyKcxKAdR4U3QM5FztDTdw9hAoGBAIO+
0IN5sSwsKVtbvnSQnwjoRt9Lcp2rSDq1bXfZogXtFig8yMacFK6jil3aD07tZ4W8
gktm4MlUoXsb3OJ0t4mTTaN7QykNVHGl6l9Nm7Z3OU0d3jqZ31vA3yr4O/EZYxs8
7nYehVriLXew5eyqN2qh7lco4rs7I/SWCvV54SWJAoGAVVuqn0lNwqBO7ngVqO5t
6sjDh6U0AouT9lpmhUIeITrBRgEl40v2ytrrSJB/4AymNHSz6GnKTX7drhpUV+zE
1ISBdT4TTRhSGX5dWSbc9S28zgHf1TZlJNbkvgH0SF6bZm3w5N/5r/Q17OZlRz2Y
oQwQvj05CNC+iuDXSsbQ/dQ=
-----END PRIVATE KEY-----
```

> **Atenção:** a chave acima é um exemplo gerado para ilustração de formato — não a utilize em produção.

---

**Chave privada — PKCS1 / formato legado RSA** (`-----BEGIN RSA PRIVATE KEY-----`)

Formato mais antigo, gerado pelo OpenSSL sem a flag `-pkcs8`. Ainda comum em aplicações legadas. Também suportado pelo mockr como fallback.

```
-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAwRydvlAy+hAmWHNjIVfYo9zaul6+2aqI1g4xXjYX1rrl/bzv
Mj0snznLV2elgjRv80qLuNX2sSJGu2t+EHfHGAbcwVZLbsfstEpmB2Cjghnd2DPu
dOc3e6oSQmrhqB6/1xIjBbwvjjJmXjHgCb1XtXHXXcblm1XhlIT/RKuVC6zorrCH
yzpnFFIdASYCaIpXQ/uHUg+YgAMIvEAgLl3BeaJS8QkplA1KS3Vya2TzPBnFZlPw
W8LRoB0jTIztLVIE+KAc0pqebdHK2KA5LmODgJPYtT6fY1ea3VyuWih4h4fOSt7S
swfkGehY/r0OfLwrr4F0fIi52eFzliD4AlkLawIDAQABAoIBABko28pwU3vt/7j1
a5xsAA81VpACiRX92G8fEUKHhKqW1iEtbPKcMsSZ+IquN8uTfmRxuWt/APGKGDIR
6q5FlLxmJHvR/Oj1hjrlGI7sgdERAqzI80/q0fk8Kr3ZhAXt36urMsHsBzwdB+ZM
1DYgo51BtEAFCmQnF8lMXJ+7RSNsjLgnNUbKycRRP7HBLC24UBu/Qwz3Trtwe9FW
/ZZHSebsSDoOUTpjUtMHuzrUdc8sqLzdbylrgtZM97SuI6jJUL6TnTZjoAca0A01
uab1yxVKe8UBMwF/4QFgtElne+zToMll2JllZfA4WV+PeEADhjvJaxsS/gzekgmV
iAnjl40CgYEA/taii6/a0VjBleMYB82pBpOde+2VOKpg63VnvuQUvlr4CwjBjIN5
A+OQFGP0De8g1CYk8mEv+uNYHVlVB6ivYlOx90LuJcEo/e9CCmwJmcGkk5LNzu4F
J03H+C2Hlm1V9y2iQAuEzmPtY3jdLsxfOwwmp37AjvSN7XfQW8b6C4UCgYEAwf30
MAFoO557ULcZ1rhOvTTYlhyV93msJoviWegVWRnz1jxsJyPXSwcF3/QcrCqN4T1c
MF4fhVt4KiI8CfLCZnGmipmWbBcVsJfS76RPoMR6suX/iuT6WX+6rb8FjWjGiCf/
AVcOS6UILaL92Kiel8M4iONGZwN4Kh1O9SU9li8CgYBWcslBSa6ZRdql1hvYydOE
/5504+uzBVlLhtBi2UyQJpvuYlIGMLNktlWyFRY9g9lHHol6D64PFgBWNG+74WVd
nbbJglL3S8d0HS3yCyRWnHIP/cUDZYsqAiDL9+X/CagX6csYns+2k/97twyRS5Mi
nMSgHUeFN0DORc7Q03cPYQKBgQCDvtCDebEsLClbW750kJ8I6EbfS3Kdq0g6tW13
2aIF7RYoPMjGnBSuo4pd2g9O7WeFvIJLZuDJVKF7G9zidLeJk02je0MpDVRxpepf
TZu2dzlNHd46md9bwN8q+DvxGWMbPO52HoVa4i13sOXsqjdqoe5XKOK7OyP0lgr1
eeEliQKBgFVbqp9JTcKgTu54FajuberIw4elNAKLk/ZaZoVCHiE6wUYBJeNL9sra
60iQf+AMpjR0s+hpyk1+3a4aVFfsxNSEgXU+E00YUhl+XVkm3PUtvM4B39U2ZSTW
5L4B9Ehem2Zt8OTf+a/0NezmZUc9mKEMEL49OQjQvorg10rG0P3U
-----END RSA PRIVATE KEY-----
```

> **Atenção:** a chave acima é um exemplo gerado para ilustração de formato — não a utilize em produção.

### Gerar um par de chaves para teste

Use o OpenSSL (disponível em macOS, Linux e WSL):

```bash
# Cria o diretório de chaves
mkdir -p keys

# Gera a chave privada RSA 2048-bit no formato PKCS8 (compatível com Java moderno)
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out keys/private.pem

# Extrai a chave pública correspondente no formato PKIX
openssl pkey -pubout -in keys/private.pem -out keys/public.pem
```

Para gerar no formato legado PKCS1 (se a aplicação Java usar `RSAPrivateKey` / `PKCS1EncodedKeySpec`):

```bash
# Chave privada no formato PKCS1 (header: -----BEGIN RSA PRIVATE KEY-----)
openssl genrsa -out keys/private_pkcs1.pem 2048

# Chave pública correspondente (formato PKIX — mesmo resultado)
openssl rsa -pubout -in keys/private_pkcs1.pem -out keys/public.pem
```

> O mockr detecta o formato automaticamente pelo cabeçalho do arquivo — `-----BEGIN PRIVATE KEY-----` (PKCS8) ou `-----BEGIN RSA PRIVATE KEY-----` (PKCS1). Não é necessário configurar o formato manualmente.

### Verificar a compatibilidade com a aplicação Java

Gere um payload com o mockr e tente descriptografar com a chave privada da aplicação Java (ou vice-versa):

```bash
# 1. Criptografa um JSON de teste com a chave pública
./mockr encrypt --config ./mock.yaml --text '{"nome":"Marco","valor":1500.00}'

# Saída (exemplo):
# SGVsbG8gV29ybGQhLi4u...

# 2. Descriptografa o mesmo payload para confirmar o round-trip
./mockr decrypt --config ./mock.yaml --text "SGVsbG8gV29ybGQhLi4u..."

# Saída esperada:
# {
#   "nome": "Marco",
#   "valor": 1500.00
# }
```

Se a aplicação Java consegue descriptografar o payload gerado pelo `mockr encrypt`, e o `mockr decrypt` consegue ler os payloads gerados pelo Java, a compatibilidade está confirmada.

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
| `{{now}}` | string | `"2026-04-19T13:00:40Z"` |
| `{{now.unix}}` | int64 | `1745492440` |
| `{{now.date}}` | string | `"2026-04-19"` |
| `{{now.time}}` | string | `"13:00:40"` |

**Números e booleanos**

| Template | Tipo | Exemplo de saída |
|---|---|---|
| `{{rand.int}}` | int | `4821` |
| `{{rand.bool}}` | bool | `true` ou `false` |
| `{{rand.float}}` | float64 | `0.73` |

**Pessoa**

| Template | Tipo | Exemplo de saída |
|---|---|---|
| `{{faker.name}}` | string | `"Mariana Lopes"` |
| `{{faker.first_name}}` | string | `"Mariana"` |
| `{{faker.last_name}}` | string | `"Lopes"` |
| `{{faker.gender}}` | string | `"Female"` |
| `{{faker.ssn}}` | string | `"296-62-1671"` |
| `{{faker.job_title}}` | string | `"Senior Designer"` |
| `{{faker.job_company}}` | string | `"Acme Corp"` |
| `{{faker.username}}` | string | `"mariana423"` |
| `{{faker.password}}` | string | `"xK3mP9qL2nRt"` |

**Contato e internet**

| Template | Tipo | Exemplo de saída |
|---|---|---|
| `{{faker.email}}` | string | `"mariana.lopes@gmail.com"` |
| `{{faker.phone}}` | string | `"+55 11 91234-5678"` |
| `{{faker.url}}` | string | `"https://example.com/path"` |
| `{{faker.domain}}` | string | `"example.com"` |
| `{{faker.ip}}` | string | `"192.168.1.42"` |
| `{{faker.ipv6}}` | string | `"2001:db8::1"` |
| `{{faker.mac}}` | string | `"a1:b2:c3:d4:e5:f6"` |
| `{{faker.useragent}}` | string | `"Mozilla/5.0 ..."` |

**Endereço**

| Template | Tipo | Exemplo de saída |
|---|---|---|
| `{{faker.address}}` | string | `"123 Main St, Springfield"` |
| `{{faker.street}}` | string | `"123 Main St"` |
| `{{faker.city}}` | string | `"Springfield"` |
| `{{faker.state}}` | string | `"Ohio"` |
| `{{faker.country}}` | string | `"United States"` |
| `{{faker.country_code}}` | string | `"US"` |
| `{{faker.zip}}` | string | `"62701"` |
| `{{faker.latitude}}` | float64 | `-23.55` |
| `{{faker.longitude}}` | float64 | `-46.63` |

**Finanças**

| Template | Tipo | Exemplo de saída |
|---|---|---|
| `{{faker.price}}` | float64 | `149.90` |
| `{{faker.currency}}` | string | `"BRL"` |
| `{{faker.credit_card}}` | string | `"4111111111111111"` |
| `{{faker.iban}}` | string | `"123456789012"` |
| `{{faker.bitcoin}}` | string | `"1A2B3C4D..."` |

**Cores e tech**

| Template | Tipo | Exemplo de saída |
|---|---|---|
| `{{faker.color}}` | string | `"Crimson"` |
| `{{faker.hex_color}}` | string | `"#e63946"` |
| `{{faker.http_method}}` | string | `"POST"` |
| `{{faker.http_status}}` | int | `200` |
| `{{faker.mime_type}}` | string | `"pdf"` |
| `{{faker.image_url}}` | string | `"https://picsum.photos/400/300"` |

**Texto**

| Template | Tipo | Exemplo de saída |
|---|---|---|
| `{{faker.word}}` | string | `"lorem"` |
| `{{faker.sentence}}` | string | `"Lorem ipsum dolor amet."` |
| `{{faker.paragraph}}` | string | `"Lorem ipsum dolor..."` |
| `{{faker.lorem}}` | string | `"Lorem ipsum dolor sit amet."` |

**Datas**

| Template | Tipo | Exemplo de saída |
|---|---|---|
| `{{faker.date}}` | string | `"1994-07-22"` |
| `{{faker.date_time}}` | string | `"1994-07-22T10:30:00Z"` |
| `{{faker.future_date}}` | string | `"2027-01-15"` |
| `{{faker.past_date}}` | string | `"2025-03-08"` |

**IDs brasileiros**

| Template | Tipo | Exemplo de saída |
|---|---|---|
| `{{faker.cpf}}` | string | `"123.456.789-09"` |
| `{{faker.cnpj}}` | string | `"12.345.678/0001-90"` |
| `{{faker.cep}}` | string | `"01310-100"` |

**Contexto da request**

| Template | Tipo | Descrição |
|---|---|---|
| `{{body.campo}}` | any | Valor do campo do body da request (POST/PUT/PATCH) |
| `{{body.a.b}}` | any | Dot notation para campos aninhados |
| `{{query.param}}` | string | Valor do query param da URL (qualquer método) |

### Templates do body da request — `{{body.*}}`

Em requisições **POST, PUT e PATCH**, você pode espelhar campos do body enviado pelo cliente diretamente na resposta. Use `{{body.campo}}` para acessar qualquer campo do JSON recebido.

Suporta dot notation para campos aninhados: `{{body.address.city}}`.

Se o campo não existir no body, retorna string vazia.

**Exemplo:**

Request — `POST /users` com body:
```json
{ "name": "Marco", "email": "marco@exemplo.com", "document": "123.456.789-00" }
```

`user_created.json`:
```json
{
  "id": "{{uuid}}",
  "name": "{{body.name}}",
  "email": "{{body.email}}",
  "document": "{{body.document}}",
  "createdAt": "{{now}}"
}
```

Resposta gerada:
```json
{
  "id": "bb44cca7-268a-42fd-a946-0f4e6b5f3cc3",
  "name": "Marco",
  "email": "marco@exemplo.com",
  "document": "123.456.789-00",
  "createdAt": "2026-04-17T14:32:01Z"
}
```

> `{{body.*}}` em GETs e DELETEs retorna string vazia — esses métodos não têm body.

### Templates de query params — `{{query.*}}`

Permite que a resposta reflita query parameters da URL. Funciona em qualquer método HTTP — ao contrário de `{{body.*}}`, que só opera em POST/PUT/PATCH.

Use `{{query.nome_do_param}}` para acessar qualquer parâmetro da query string.

**Exemplo:**

Request — `GET /search?name=Claude&page=2`:

`search_result.json`:
```json
{
  "query": "{{query.name}}",
  "page": "{{query.page}}",
  "results": []
}
```

Resposta gerada:
```json
{
  "query": "Claude",
  "page": "2",
  "results": []
}
```

> Se o parâmetro não estiver na URL, o template retorna string vazia.

> `{{query.*}}` e `{{body.*}}` podem ser usados juntos na mesma resposta — por exemplo, em um POST com query params.

### Exemplo completo de JSON com templates

```json
{
  "id": "{{seq}}",
  "uuid": "{{uuid}}",
  "name": "{{faker.name}}",
  "email": "{{faker.email}}",
  "username": "{{faker.username}}",
  "phone": "{{faker.phone}}",
  "cpf": "{{faker.cpf}}",
  "company": "{{faker.job_company}}",
  "jobTitle": "{{faker.job_title}}",
  "address": "{{faker.address}}",
  "city": "{{faker.city}}",
  "country": "{{faker.country_code}}",
  "balance": "{{faker.price}}",
  "active": "{{rand.bool}}",
  "score": "{{rand.float}}",
  "createdAt": "{{now}}",
  "birthDate": "{{faker.past_date}}",
  "bio": "{{faker.sentence}}"
}
```

Cada chamada a esse endpoint retorna um objeto diferente.

---

## Respostas de erro

Quando o servidor não consegue atender uma requisição, retorna sempre um JSON estruturado com `Content-Type: application/json` — nunca texto puro. Isso facilita o tratamento de erros em clientes e testes automatizados.

### Formato

```json
{ "error": "<mensagem descritiva>" }
```

### Casos e status codes

| Situação | Status | Corpo |
|---|---|---|
| Path não existe no config | `404` | `{ "error": "route not found" }` |
| Path existe mas método é diferente | `405` | `{ "error": "method not allowed" }` |
| Falha ao renderizar o template de resposta | `500` | `{ "error": "error rendering response template" }` |

**Exemplo — rota não encontrada:**

```bash
curl -i http://localhost:8080/naoexiste
```
```
HTTP/1.1 404 Not Found
Content-Type: application/json

{"error":"route not found"}
```

**Exemplo — método não permitido:**

```bash
curl -i -X DELETE http://localhost:8080/users
```
```
HTTP/1.1 405 Method Not Allowed
Content-Type: application/json

{"error":"method not allowed"}
```

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

### Disparar requisições com dados faker

Use `mockr request` para testar uma rota rapidamente sem precisar de curl ou Postman:

```bash
# GET simples
./mockr request --config ./mock.yaml --path /products

# POST com body faker gerado automaticamente
./mockr request --config ./mock.yaml --path /products --method POST

# 10 POSTs consecutivos, cada um com dados diferentes
./mockr request --config ./mock.yaml --path /products --method POST --repeat 10
```

Para disparar contra sua API real (não o mock), use `--url`:

```bash
./mockr request --config ./mock.yaml \
  --url http://minha-api.com/products \
  --method POST \
  --path /products
```

O body continua sendo lido do campo `body` da rota no config e os templates faker são resolvidos antes do envio.

### Teste de carga com múltiplos usuários autenticados

Simule N usuários simultâneos se autenticando e disparando requisições — cada um com seu próprio token, distribuídos aleatoriamente:

**`mock.yaml`**
```yaml
auth:
  url: http://minha-api.com/auth
  method: POST
  users: ./auth_users.json
  extract: "data.token"
  header: "Authorization"
  prefix: "Bearer "

routes:
  - method: POST
    path: /orders
    response: ./order.json
    body: ./order_body.json
    status: 201
    auth: true
```

**`auth_users.json`**
```json
[
  { "username": "alice", "password": "pass_alice" },
  { "username": "bob",   "password": "pass_bob"   },
  { "username": "carol", "password": "pass_carol" }
]
```

```bash
# 3 usuários autenticados em paralelo, 100 requisições distribuídas aleatoriamente entre eles
./mockr request --config ./mock.yaml --path /orders --method POST --repeat 100
```

Para um teste de fluxo completo com múltiplos endpoints e múltiplos usuários em paralelo, combine com shell script:

```bash
#!/bin/bash
CONFIG="./mock.yaml"
API="http://minha-api.com"

./mockr request --config $CONFIG --url $API/orders  --method POST --path /orders  --repeat 50 &
./mockr request --config $CONFIG --url $API/products --method POST --path /products --repeat 50 &
./mockr request --config $CONFIG --url $API/users   --method GET  --path /users   --repeat 50 &
wait
```

### Stress test antes de integrar com a API real

```bash
# Suba o mock em um terminal
./mockr serve --config ./mock.yaml --port 8080

# Em outro terminal, rode o stress test
./mockr stress --url http://localhost:8080/products --method GET -n 2000 -c 100
```

### Teste de fluxo paralelo com shell script

A combinação de `mockr request --url` com shell script permite simular fluxos reais de uso de uma API com múltiplos endpoints sendo atingidos ao mesmo tempo — algo próximo do comportamento de uma aplicação em produção, onde usuários diferentes disparam operações distintas simultaneamente.

Diferente do `mockr stress`, que bombardeia uma única rota para medir throughput, o teste de fluxo valida **comportamento sob carga distribuída**: autenticação, criação de recursos, consultas e deleções acontecendo em paralelo, cada chamada com um payload faker único gerado na hora.

Isso ajuda a detectar problemas reais que testes sequenciais não revelam:

- **Condições de corrida** — dois POSTs simultâneos disputando o mesmo recurso
- **Degradação seletiva** — um endpoint lento impactando os demais por esgotamento de pool de conexões
- **Falhas de validação sob carga** — regras de negócio que passam em chamadas únicas mas quebram quando o banco recebe volume
- **Comportamento de cache** — endpoints de leitura que deveriam ser rápidos ficando lentos porque os de escrita estão saturando o banco

**Exemplo de script:**

```bash
#!/bin/bash

CONFIG="./mock.yaml"
API="http://minha-api.com"
REPEAT=50

echo "=== iniciando teste de fluxo ==="
echo ""

# Todos os endpoints disparam em paralelo com & (background)
./mockr request --config $CONFIG \
  --url $API/users --method GET \
  --repeat $REPEAT &

./mockr request --config $CONFIG \
  --url $API/users --method POST --path /users \
  --repeat $REPEAT &

./mockr request --config $CONFIG \
  --url $API/products --method GET \
  --repeat $REPEAT &

./mockr request --config $CONFIG \
  --url $API/products --method POST --path /products \
  --repeat $REPEAT &

./mockr request --config $CONFIG \
  --url $API/orders --method POST --path /orders \
  --repeat $REPEAT &

# Aguarda todos os processos terminarem
wait

echo ""
echo "=== teste concluído ==="
```

O `&` envia cada `mockr request` para background. O `wait` segura o script até o último processo terminar. Com 5 endpoints e `REPEAT=50`, são **250 requisições com payloads faker distintos** disparadas em paralelo — tudo isso com um único arquivo YAML como fonte de verdade.

Para escalar, aumente o `REPEAT` ou adicione mais blocos. Para testar um fluxo de negócio específico — como criação de usuário seguida de criação de pedido — basta encadear os comandos com dependência explícita:

```bash
# Primeiro cria o usuário, depois dispara os pedidos em paralelo
./mockr request --config $CONFIG --url $API/users --method POST --path /users --repeat 10
wait

./mockr request --config $CONFIG --url $API/orders --method POST --path /orders --repeat 50 &
./mockr request --config $CONFIG --url $API/payments --method POST --path /payments --repeat 50 &
wait
```

> Combine com `mockr stress` para os endpoints críticos: use o script de fluxo para aquecimento e o stress para medir o limite de cada rota individualmente.

---

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
