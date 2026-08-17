# Roteiro da demonstração

## Preparação

1. Criar o arquivo local com `cp .env.example .env`, definir uma chave exclusiva em
   `INVENTORY_INTERNAL_TOKEN` e subir o ambiente com `docker compose up -d --build`.
2. Confirmar `GET /ready` no Inventory e no Billing e `GET /healthz` no web.
3. Abrir <http://localhost:4200> e apresentar o diagrama da arquitetura.

## Fluxo funcional

1. Abrir Produtos e mostrar paginação, busca e filtros remotos.
2. Cadastrar um produto com código no formato visual `PRD-001`, descrição e saldo.
3. Criar uma nota com vários itens usando a pesquisa remota de produtos.
4. Mostrar que quantidade acima do saldo bloqueia a continuação.
5. Criar a nota e confirmar status `Aberta`, número sequencial e snapshots dos itens.
6. Acionar `Emitir e imprimir` e acompanhar o processamento.
7. Confirmar status `Fechada`, visão imprimível e novos saldos.
8. Demonstrar pela API que uma nota fechada não pode ser processada novamente.

## Falha e recuperação

1. Criar previamente outra nota aberta com saldo suficiente.
2. Interromper somente o Inventory com `docker compose stop inventory`.
3. Solicitar o fechamento da nota preparada.
4. Mostrar o estado `RETRYING` e a mensagem de indisponibilidade em português.
5. Recarregar a página e confirmar que o acompanhamento da operação é retomado.
6. Restaurar o serviço com `docker compose start inventory`.
7. Confirmar a transição para `COMPLETED`, o fechamento e uma única baixa.

O cenário completo pode ser reproduzido com:

```bash
node scripts/failure-recovery.mjs
```

## Concorrência e idempotência

1. Executar duas notas concorrentes sobre um produto com saldo 1.
2. Confirmar que uma fecha, uma falha e o saldo final é zero.
3. Repetir a criação de uma nota com a mesma `Idempotency-Key` e confirmar que o identificador da nota não muda.
4. Repetir o fechamento com a mesma `Idempotency-Key`.
5. Confirmar que a mesma operação é retornada e nenhum movimento adicional é criado.

O smoke test reproduzível é:

```bash
node scripts/e2e-smoke.mjs
```

## Encerramento

1. Mostrar as camadas `domain/application/infra` e o wiring em `cmd/`.
2. Mostrar OpenAPI, migrations, testes e pipeline.
3. Mostrar logs correlacionados por `operationId`, `commandId` e request ID.
4. Mostrar `/metrics` nos dois serviços.
5. Explicar os trade-offs registrados no detalhamento técnico.
