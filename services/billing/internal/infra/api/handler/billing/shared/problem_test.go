package shared

import (
	"testing"

	"github.com/stretchr/testify/require"

	invoiceentity "github.com/macielcr7/korp-teste/services/billing/internal/domain/entity/billing/invoice"
)

func TestClassifyReturnsPortugueseUserMessage(t *testing.T) {
	problem := classify(invoiceentity.ErrInvoiceNotClosed)

	require.Equal(t, "Conflito", problem.Title)
	require.Equal(t, "Somente notas emitidas podem ser impressas.", problem.Detail)
	require.Equal(t, "INVOICE_NOT_CLOSED", problem.Code)
}

func TestLocalizeOperationErrorSupportsLegacyInventoryMessage(t *testing.T) {
	require.Equal(
		t,
		"Saldo insuficiente para um ou mais produtos da nota.",
		LocalizeOperationError("insufficient product balance"),
	)
}

func TestLocalizeOperationErrorDoesNotExposeTechnicalDetail(t *testing.T) {
	detail := LocalizeOperationError(`inventory unavailable: Post "http://inventory:8081/internal/inventory/v1/stock-debits": connection refused`)

	require.Equal(t, "O serviço de estoque não respondeu. A emissão será tentada novamente automaticamente.", detail)
}

func TestLocalizeOperationErrorPreservesKnownPortugueseBusinessMessage(t *testing.T) {
	require.Equal(
		t,
		"Saldo insuficiente para um ou mais produtos da nota.",
		LocalizeOperationError("Saldo insuficiente para um ou mais produtos da nota."),
	)
}

func TestLocalizeOperationErrorUsesActionableFallback(t *testing.T) {
	require.Equal(
		t,
		"A emissão foi rejeitada pelo estoque. Verifique os produtos e as quantidades da nota antes de tentar novamente.",
		LocalizeOperationError("unexpected technical rejection"),
	)
}
