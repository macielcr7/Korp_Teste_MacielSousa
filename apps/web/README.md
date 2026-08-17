# Korp Web

Frontend Angular 22 standalone para cadastro de produtos, criação, emissão e impressão de notas.

## Desenvolvimento

Requer Node.js 24 e pnpm 11.

```bash
pnpm install
pnpm start
```

O frontend usa URLs same-origin. O servidor de desenvolvimento já encaminha `/api/inventory` para `localhost:8081` e `/api/billing` para `localhost:8082`, preservando os caminhos completos.

## Qualidade

```bash
pnpm lint
pnpm test:ci
pnpm build
```

## Container

A imagem multi-stage compila o frontend e o entrega por Nginx na porta `8080`. O proxy preserva o caminho completo e encaminha:

- `/api/inventory/*` para `inventory:8081`;
- `/api/billing/*` para `billing-api:8082`.

O endpoint de saúde do frontend é `/healthz`.

## Arquitetura do frontend

As features `products` e `invoices` separam modelos de domínio, stores de aplicação, serviços HTTP e apresentação. Componentes são standalone, usam detecção `OnPush`, formulários tipados, Signals para estado local e RxJS para busca, HTTP e polling.

As rotas são carregadas sob demanda. Listas consultam filtros e páginas no backend; o formulário de nota pesquisa produtos remotamente e impede quantidades superiores ao saldo conhecido antes do envio.
