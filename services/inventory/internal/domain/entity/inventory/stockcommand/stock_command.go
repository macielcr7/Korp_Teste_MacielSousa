// Package stockcommand contains idempotent stock debit commands and their results.
package stockcommand

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	// StatusCommitted indicates that every stock movement was persisted.
	StatusCommitted = "COMMITTED"
	// StatusRejected indicates a terminal business rejection with no stock change.
	StatusRejected = "REJECTED"
)

// MaximumSafeInteger is the largest exact integer supported by JSON clients based on IEEE-754.
const MaximumSafeInteger int64 = 9_007_199_254_740_991

const (
	// ErrorProductNotFound identifies a missing product in a debit.
	ErrorProductNotFound = "PRODUCT_NOT_FOUND"
	// ErrorInsufficientStock identifies a product without enough balance.
	ErrorInsufficientStock = "INSUFFICIENT_STOCK"
)

var (
	// ErrInvalidCommand indicates malformed command data.
	ErrInvalidCommand = errors.New("invalid stock command")
	// ErrDuplicateProduct indicates repeated product IDs in the same command.
	ErrDuplicateProduct = errors.New("stock command contains duplicate product")
	// ErrInvalidQuantity indicates a debit outside the public JSON contract.
	ErrInvalidQuantity = errors.New("stock debit quantity must be a positive safe integer")
	// ErrNotFound indicates that a stock command does not exist.
	ErrNotFound = errors.New("stock command not found")
	// ErrIdempotencyConflict indicates reuse of a command ID with a different payload.
	ErrIdempotencyConflict = errors.New("stock command ID was already used with a different payload")
	// ErrProductNotFound indicates that a referenced product does not exist.
	ErrProductNotFound = errors.New("stock command references an unknown product")
	// ErrInsufficientStock indicates that at least one product cannot fulfill the debit.
	ErrInsufficientStock = errors.New("insufficient product balance")
)

// Item is one requested product debit.
type Item struct {
	ProductID string
	Quantity  int64
}

// Movement records the balance transition produced by a committed command.
type Movement struct {
	ID            string
	ProductID     string
	Quantity      int64
	BalanceBefore int64
	BalanceAfter  int64
	CreatedAt     time.Time
}

// StockCommand is an idempotent, all-or-nothing multi-product debit.
type StockCommand struct {
	commandID   string
	invoiceID   string
	payloadHash string
	status      string
	errorCode   string
	errorDetail string
	items       []Item
	movements   []Movement
	createdAt   time.Time
}

// Snapshot contains the persisted state required to rebuild a stock command.
type Snapshot struct {
	CommandID   string
	InvoiceID   string
	PayloadHash string
	Status      string
	ErrorCode   string
	ErrorDetail string
	Items       []Item
	Movements   []Movement
	CreatedAt   time.Time
}

// BalanceChange describes one validated balance transition.
type BalanceChange struct {
	ProductID     string
	Quantity      int64
	BalanceBefore int64
	BalanceAfter  int64
}

// InsufficientStockError identifies the product that cannot fulfill a debit.
type InsufficientStockError struct {
	ProductID string
	Balance   int64
	Required  int64
}

func (err *InsufficientStockError) Error() string {
	return fmt.Sprintf("product %s has balance %d but requires %d", err.ProductID, err.Balance, err.Required)
}

// Unwrap exposes the stable domain error used by callers.
func (err *InsufficientStockError) Unwrap() error { return ErrInsufficientStock }

// New validates, canonicalizes and fingerprints a stock debit command.
func New(commandID, invoiceID string, items []Item) (StockCommand, error) {
	commandID = strings.TrimSpace(commandID)
	invoiceID = strings.TrimSpace(invoiceID)
	if commandID == "" || invoiceID == "" || len(items) == 0 {
		return StockCommand{}, ErrInvalidCommand
	}

	canonicalItems := append([]Item(nil), items...)
	seen := make(map[string]struct{}, len(canonicalItems))
	for index := range canonicalItems {
		canonicalItems[index].ProductID = strings.TrimSpace(canonicalItems[index].ProductID)
		if canonicalItems[index].ProductID == "" {
			return StockCommand{}, ErrInvalidCommand
		}
		if canonicalItems[index].Quantity <= 0 || canonicalItems[index].Quantity > MaximumSafeInteger {
			return StockCommand{}, ErrInvalidQuantity
		}
		if _, exists := seen[canonicalItems[index].ProductID]; exists {
			return StockCommand{}, ErrDuplicateProduct
		}
		seen[canonicalItems[index].ProductID] = struct{}{}
	}

	sort.Slice(canonicalItems, func(i, j int) bool {
		return canonicalItems[i].ProductID < canonicalItems[j].ProductID
	})

	payload, err := json.Marshal(struct {
		InvoiceID string `json:"invoiceId"`
		Items     []Item `json:"items"`
	}{InvoiceID: invoiceID, Items: canonicalItems})
	if err != nil {
		return StockCommand{}, fmt.Errorf("fingerprint stock command: %w", err)
	}
	digest := sha256.Sum256(payload)
	return StockCommand{
		commandID: commandID, invoiceID: invoiceID,
		payloadHash: hex.EncodeToString(digest[:]), items: canonicalItems,
	}, nil
}

// Rehydrate rebuilds a stock command stored by a repository.
func Rehydrate(snapshot Snapshot) (StockCommand, error) {
	command, err := New(snapshot.CommandID, snapshot.InvoiceID, snapshot.Items)
	if err != nil {
		return StockCommand{}, err
	}
	if snapshot.PayloadHash == "" || (snapshot.Status != StatusCommitted && snapshot.Status != StatusRejected) || snapshot.CreatedAt.IsZero() {
		return StockCommand{}, ErrInvalidCommand
	}
	command.payloadHash = snapshot.PayloadHash
	command.status = snapshot.Status
	command.errorCode = snapshot.ErrorCode
	command.errorDetail = snapshot.ErrorDetail
	command.movements = cloneMovements(snapshot.Movements)
	command.createdAt = snapshot.CreatedAt.UTC()
	return command, nil
}

// PlanDebits applies the command rules to locked balance snapshots without mutating persistence.
func (command StockCommand) PlanDebits(balances map[string]int64) ([]BalanceChange, error) {
	changes := make([]BalanceChange, 0, len(command.items))
	for _, item := range command.items {
		balance, found := balances[item.ProductID]
		if !found {
			return nil, ErrProductNotFound
		}
		if balance < item.Quantity {
			return nil, &InsufficientStockError{ProductID: item.ProductID, Balance: balance, Required: item.Quantity}
		}
		changes = append(changes, BalanceChange{
			ProductID: item.ProductID, Quantity: item.Quantity,
			BalanceBefore: balance, BalanceAfter: balance - item.Quantity,
		})
	}
	return changes, nil
}

// CommandID returns the idempotency identifier.
func (command StockCommand) CommandID() string { return command.commandID }

// InvoiceID returns the invoice identifier.
func (command StockCommand) InvoiceID() string { return command.invoiceID }

// PayloadHash returns the canonical request fingerprint.
func (command StockCommand) PayloadHash() string { return command.payloadHash }

// Status returns the durable command status.
func (command StockCommand) Status() string { return command.status }

// ErrorCode returns the persisted terminal error code.
func (command StockCommand) ErrorCode() string { return command.errorCode }

// ErrorDetail returns the persisted terminal error detail.
func (command StockCommand) ErrorDetail() string { return command.errorDetail }

// Items returns a defensive copy of requested debits.
func (command StockCommand) Items() []Item { return append([]Item(nil), command.items...) }

// Movements returns a defensive copy of committed movements.
func (command StockCommand) Movements() []Movement { return cloneMovements(command.movements) }

// CreatedAt returns when the command was persisted.
func (command StockCommand) CreatedAt() time.Time { return command.createdAt }

// BusinessError maps a persisted rejected result back to its domain error.
func (c StockCommand) BusinessError() error {
	if c.status != StatusRejected {
		return nil
	}
	switch c.errorCode {
	case ErrorProductNotFound:
		return ErrProductNotFound
	case ErrorInsufficientStock:
		return ErrInsufficientStock
	default:
		return ErrInvalidCommand
	}
}

func cloneMovements(movements []Movement) []Movement {
	return append([]Movement(nil), movements...)
}
