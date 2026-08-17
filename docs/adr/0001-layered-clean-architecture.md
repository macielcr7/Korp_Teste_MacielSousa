# ADR-001: Arquitetura em camadas para os serviços Go


## Contexto

Os serviços precisam manter regras de negócio independentes de HTTP, PostgreSQL e bibliotecas externas, além de permitir testes isolados.

## Decisão

Cada serviço usa `domain/application/infra`, replica o Bounded Context nas camadas e realiza wiring exclusivamente em `cmd/`.

O fluxo de dependências permitido é `infra -> application -> domain`.

## Consequências

- Entidades e invariantes ficam no domínio.
- Casos de uso coordenam o fluxo sem importar infra.
- Handlers, banco e clientes HTTP ficam em infra.
