# Korp - Sistema de emissão de notas fiscais

Aplicação do desafio técnico Korp, implementada com Angular e dois microsserviços Go independentes:

- **Inventory Service:** produtos, saldos e movimentos de estoque.
- **Billing Service:** notas fiscais e operação durável de fechamento.
- **Angular Web:** cadastro, consulta, emissão e impressão.


## Arquitetura

```mermaid
flowchart LR
    U["Usuário"] --> W["Angular"]
    W --> N["Nginx"]
    N --> I["Inventory Service"]
    N --> B["Billing API"]
    B -->|"consulta autenticada"| I
    K["Billing Worker"] -->|"baixa autenticada"| I
    B --> BD["PostgreSQL Billing"]
    K --> BD
    I --> ID["PostgreSQL Inventory"]
```

Cada serviço segue a arquitetura `domain/application/infra`, replica seu Bounded Context nas camadas e realiza wiring somente em `cmd/`.

## Requisitos

- Docker Desktop com Docker Compose.
- Go 1.26.5 ou superior para desenvolvimento local do backend.
- Node.js 24 e pnpm 11 para desenvolvimento local do frontend.

## Execução com Docker

Crie a configuração local antes da primeira execução e substitua o valor de
`INVENTORY_INTERNAL_TOKEN` por uma chave exclusiva do ambiente:

```bash
cp .env.example .env
```

No PowerShell, use `Copy-Item .env.example .env`. O Compose interrompe a inicialização quando o
token interno não está configurado, evitando a comunicação entre serviços com uma credencial
padrão conhecida.

```bash
docker compose up --build
```

Depois da inicialização:

- Aplicação web: <http://localhost:4200>
- Inventory API: <http://localhost:8081/health>
- Billing API: <http://localhost:8082/health>
- Métricas Inventory: <http://localhost:8081/metrics>
- Métricas Billing: <http://localhost:8082/metrics>

A porta interna `8083` do Inventory fica apenas na rede do Compose e não é publicada no host. As rotas `/internal` retornam `404` na porta pública `8081`.

Para encerrar:

```bash
docker compose down
```

Os dados permanecem em volumes Docker. Para remover também os dados de desenvolvimento:

```bash
docker compose down --volumes
```

## Desenvolvimento local

A execução dos serviços fora do Docker pressupõe dois bancos PostgreSQL acessíveis no host, um para Inventory e outro para Billing. Crie os bancos antes de iniciar os serviços e aplique as migrations. Os exemplos abaixo usam Bash; no PowerShell, defina as variáveis com `$env:NOME="valor"`.

### Inventory Service

```bash
cd services/inventory
export DATABASE_URL="postgres://inventory:inventory@localhost:5432/inventory?sslmode=disable"
export INVENTORY_INTERNAL_TOKEN="change-this-internal-token"

go run ./cmd/migrate -migrations ./migrations
go test ./...
go run ./cmd/app
```

O processo inicia a API pública em `:8081` e a API privada em `:8083`.

### Billing Service

```bash
cd services/billing
export DATABASE_URL="postgres://billing:billing@localhost:5432/billing?sslmode=disable"
export INVENTORY_BASE_URL="http://localhost:8083"
export INVENTORY_INTERNAL_TOKEN="change-this-internal-token"

go run ./cmd/migrate -migrations ./migrations
go test ./...
go run ./cmd/app
```

Com a API do Billing em execução, inicie o worker de fechamento em outro terminal usando as mesmas variáveis:

```bash
cd services/billing
export DATABASE_URL="postgres://billing:billing@localhost:5432/billing?sslmode=disable"
export INVENTORY_BASE_URL="http://localhost:8083"
export INVENTORY_INTERNAL_TOKEN="change-this-internal-token"

go run ./cmd/worker
```

### Angular

```bash
cd apps/web
pnpm install
pnpm start
```

Os contratos OpenAPI ficam no diretório `api/` de cada serviço.

### Smoke tests

Com o ambiente Docker em execução:

```bash
node scripts/e2e-smoke.mjs
node scripts/failure-recovery.mjs
```

O primeiro comando valida fluxo feliz, snapshot do produto, replay idempotente, atomicidade multiproduto e concorrência com saldo 1. O segundo interrompe temporariamente o Inventory Service e comprova `RETRYING -> COMPLETED`, inclusive a retomada após reload, depois da recuperação.

## Documentação

- [Índice da documentação](./docs/README.md)
- [ADRs](./docs/adr/)
- [Detalhamento técnico](./docs/technical_details.md)
- [Roteiro de demonstração](./docs/demo_script.md)
