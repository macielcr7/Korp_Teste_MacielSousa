# ADR-003: Comandos duráveis e idempotentes


## Contexto

Criar e fechar uma nota envolve chamadas HTTP sujeitas a resposta perdida. O fechamento e a baixa de estoque também atravessam dois bancos e não podem depender de uma transação distribuída. Timeouts e reinícios não podem duplicar notas nem movimentos.

## Decisão

O Billing Service exige `Idempotency-Key` na criação da nota e persiste a chave junto com a assinatura do conteúdo. No fechamento, persiste uma operação durável, e um worker envia ao Inventory Service um comando com identificador estável. O Estoque grava idempotência, movimentos e saldos na mesma transação local.

Cada aquisição do worker recebe um token. As transições usam esse token como fencing para impedir escritas de uma execução cujo lease já foi perdido.

## Consequências

- A entrega é pelo menos uma vez, mas o efeito de negócio ocorre uma vez.
- Uma resposta perdida na criação ou no fechamento pode ser repetida com segurança.
- O usuário acompanha a operação por polling.
- Jobs interrompidos são recuperados após a expiração do lease.
- Reutilizar uma chave de criação com conteúdo diferente retorna conflito.
