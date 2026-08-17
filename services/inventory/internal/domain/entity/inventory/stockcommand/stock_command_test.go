package stockcommand

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCanonicalizesItems(t *testing.T) {
	command, err := New("command", "invoice", []Item{
		{ProductID: "b", Quantity: 1},
		{ProductID: "a", Quantity: 2},
	})

	require.NoError(t, err)
	assert.Equal(t, "a", command.Items()[0].ProductID)
	assert.Equal(t, "b", command.Items()[1].ProductID)
	assert.Len(t, command.PayloadHash(), 64)
}

func TestNewRejectsDuplicateProducts(t *testing.T) {
	_, err := New("command", "invoice", []Item{
		{ProductID: "same", Quantity: 1},
		{ProductID: "same", Quantity: 2},
	})

	assert.ErrorIs(t, err, ErrDuplicateProduct)
}

func TestNewRejectsQuantityAboveJSONSafeInteger(t *testing.T) {
	_, err := New("command", "invoice", []Item{
		{ProductID: "product", Quantity: MaximumSafeInteger + 1},
	})

	assert.ErrorIs(t, err, ErrInvalidQuantity)
}

func TestBusinessError(t *testing.T) {
	command, err := Rehydrate(Snapshot{
		CommandID: "command", InvoiceID: "invoice", PayloadHash: "hash", Status: StatusRejected,
		ErrorCode: ErrorInsufficientStock, Items: []Item{{ProductID: "product", Quantity: 1}}, CreatedAt: time.Now(),
	})
	require.NoError(t, err)
	assert.ErrorIs(t, command.BusinessError(), ErrInsufficientStock)
}

func TestPlanDebitsRejectsInsufficientStock(t *testing.T) {
	command, err := New("command", "invoice", []Item{{ProductID: "product", Quantity: 3}})
	require.NoError(t, err)

	_, err = command.PlanDebits(map[string]int64{"product": 2})

	assert.ErrorIs(t, err, ErrInsufficientStock)
}
