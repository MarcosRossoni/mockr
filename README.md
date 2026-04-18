# mockr

Ferramenta de terminal para subir servidores mock HTTP para desenvolvimento e testes de stress. Substitui soluções como Postman Mock Server, sem limitações de requisições.

## O que é

mockr é um CLI escrito em Go que permite:

- Subir um servidor HTTP mock em segundos a partir de um arquivo YAML
- Definir rotas com respostas estáticas ou dinâmicas via templates
- Gerar dados falsos automaticamente (`{{faker.name}}`, `{{uuid}}`, etc.)
- Espelhar campos do body da request na resposta (`{{body.campo}}`)
- Executar testes de stress com worker pool configurável
- Visualizar requisições em tempo real via TUI no terminal

## Instalação

```bash
git clone <repo>
cd mockr
CGO_ENABLED=0 go build -o mockr .
```

## Uso rápido

```bash
# Sobe o servidor com TUI
./mockr serve --config ./examples/mock.yaml

# Valida o YAML sem subir o servidor
./mockr validate --config ./examples/mock.yaml

# Teste de stress (500 requisições, 20 simultâneas)
./mockr stress --config ./examples/mock.yaml -n 500 -c 20

# Teste de stress em URL externa
./mockr stress --url http://localhost:9090/users --method GET -n 1000 -c 50
```

## Exemplo de config

```yaml
routes:
  - method: GET
    path: /users
    response: ./examples/users.json
    status: 200
    delay: 50ms

  - method: POST
    path: /users
    response: ./examples/user_created.json
    status: 201
```

## Templates dinâmicos

As respostas JSON suportam templates `{{...}}` que são resolvidos a cada requisição:

```json
{
  "id": "{{uuid}}",
  "name": "{{body.name}}",
  "email": "{{faker.email}}",
  "createdAt": "{{now}}"
}
```

Em requisições POST/PUT/PATCH, `{{body.campo}}` espelha o valor enviado pelo cliente.

---

> Este projeto foi desenvolvido com o auxílio do [Claude](https://claude.ai) (Anthropic) como ferramenta de pair programming.

## Guia técnico completo

Para detalhes de arquitetura, módulos, templates disponíveis e conceitos Go utilizados, consulte o [GUIDE.md](./GUIDE.md).
