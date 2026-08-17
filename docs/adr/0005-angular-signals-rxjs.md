# ADR-005: Angular Signals e RxJS sem NgRx


## Contexto

O frontend possui estado local de formulários e fluxos assíncronos de HTTP/polling, mas não possui colaboração em tempo real ou um grande grafo de estado global.

## Decisão

Usar Signals para estado síncrono e derivado, RxJS para HTTP/polling/cancelamento, stores locais nas listagens e facades nos fluxos de formulário, detalhe, impressão e dashboard. Os stores e facades dependem de portas abstratas; a configuração raiz associa essas portas aos adaptadores HTTP e de armazenamento da sessão. NgRx não será incluído.

## Consequências

- Menor quantidade de boilerplate.
- Estado próximo da feature consumidora.
- Componentes permanecem focados em apresentação e navegação.
- Integrações concretas podem ser substituídas sem alterar os fluxos da aplicação.
- NgRx poderá ser reavaliado se o domínio crescer substancialmente.
