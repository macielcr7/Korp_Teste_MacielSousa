# ADR-006: Snapshot de produtos nos itens da nota


## Contexto

Código e descrição pertencem ao catálogo do Inventory, mas uma nota precisa preservar o conteúdo apresentado quando foi criada. Consultar sempre o cadastro atual faria alterações posteriores reescreverem o histórico da nota.

## Decisão

O Billing copia `productId`, código e descrição para cada item durante a criação da nota. O snapshot é usado nas consultas e na impressão; o Inventory continua sendo a fonte autoritativa do saldo.

Alterações posteriores do catálogo não são sincronizadas com os itens já registrados, mesmo quando a nota permanece aberta. Não existe fluxo administrativo de atualização desses snapshots.

## Consequências

- Consultar ou imprimir uma nota não depende da disponibilidade do Inventory.
- Alterações no catálogo não modificam documentos fechados.
- Código e descrição ficam duplicados entre contextos de forma deliberada.
- O saldo continua sendo validado atomicamente no momento da baixa.
