# ADR-002: Banco de dados por serviço


## Contexto

Estoque e Faturamento são Bounded Contexts distintos e precisam evoluir sem acoplamento por tabelas.

## Decisão

Cada serviço possui PostgreSQL, usuário, migrations e repositórios próprios. Nenhum serviço acessa diretamente o banco do outro.

## Consequências

- Não existem foreign keys ou joins entre serviços.
- A comunicação acontece exclusivamente por contratos HTTP; as rotas internas exigem autenticação de serviço.
- Operações que atravessam serviços usam consistência eventual controlada.
