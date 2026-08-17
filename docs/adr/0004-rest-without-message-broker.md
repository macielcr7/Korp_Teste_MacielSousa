# ADR-004: REST sem broker no P0


## Contexto

O desafio exige dois microsserviços e recuperação de falhas, mas não exige mensageria nem alto volume.

## Decisão

Usar REST/JSON e OpenAPI, com job durável no banco do Billing Service. Kafka, RabbitMQ e Kubernetes ficam fora do P0.

## Consequências

- O ambiente permanece simples de executar e demonstrar.
- A operação continua recuperável sem infraestrutura adicional.
- Um broker pode substituir o polling interno no futuro sem alterar o domínio.
