// Package product contains the inventory product aggregate.
package product

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// MaximumSafeInteger is the largest exact integer supported by JSON clients based on IEEE-754.
const MaximumSafeInteger int64 = 9_007_199_254_740_991

var (
	// ErrNotFound indicates that a product does not exist.
	ErrNotFound = errors.New("product not found")
	// ErrDuplicateCode indicates that another product already uses the code.
	ErrDuplicateCode = errors.New("product code already exists")
	// ErrInvalidID indicates an empty product identifier.
	ErrInvalidID = errors.New("product ID is required")
	// ErrInvalidCode indicates an invalid product catalog identifier.
	ErrInvalidCode = errors.New("product code must contain 1 to 64 ASCII letters, digits, dots, underscores or hyphens")
	// ErrInvalidDescription indicates an empty or oversized description.
	ErrInvalidDescription = errors.New("product description must contain between 1 and 255 characters")
	// ErrInvalidBalance indicates an opening balance outside the public JSON contract.
	ErrInvalidBalance = errors.New("product balance must be a safe non-negative integer")
)

// Product is an inventory item with a non-negative available balance.
type Product struct {
	id          string
	code        string
	description string
	balance     int64
	version     int64
	createdAt   time.Time
	updatedAt   time.Time
}

// Snapshot contains the persisted state required to rebuild a product.
type Snapshot struct {
	ID          string
	Code        string
	Description string
	Balance     int64
	Version     int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// New creates a valid inventory product.
func New(id, code, description string, balance int64, now time.Time) (Product, error) {
	id = strings.TrimSpace(id)
	code = strings.ToUpper(strings.TrimSpace(code))
	description = strings.TrimSpace(description)

	switch {
	case id == "":
		return Product{}, ErrInvalidID
	case !validCode(code):
		return Product{}, ErrInvalidCode
	case utf8.RuneCountInString(description) == 0 || utf8.RuneCountInString(description) > 255:
		return Product{}, ErrInvalidDescription
	case balance < 0 || balance > MaximumSafeInteger:
		return Product{}, ErrInvalidBalance
	}

	return Product{
		id:          id,
		code:        code,
		description: description,
		balance:     balance,
		version:     1,
		createdAt:   now.UTC(),
		updatedAt:   now.UTC(),
	}, nil
}

// Rehydrate rebuilds a product stored by a repository.
func Rehydrate(snapshot Snapshot) (Product, error) {
	product, err := New(snapshot.ID, snapshot.Code, snapshot.Description, snapshot.Balance, snapshot.CreatedAt)
	if err != nil {
		return Product{}, err
	}
	if snapshot.Version <= 0 || snapshot.Version > MaximumSafeInteger || snapshot.CreatedAt.IsZero() || snapshot.UpdatedAt.IsZero() {
		return Product{}, errors.New("invalid persisted product")
	}
	product.version = snapshot.Version
	product.createdAt = snapshot.CreatedAt.UTC()
	product.updatedAt = snapshot.UpdatedAt.UTC()
	return product, nil
}

// ID returns the product identifier.
func (product Product) ID() string { return product.id }

// Code returns the normalized product code.
func (product Product) Code() string { return product.code }

// Description returns the product description.
func (product Product) Description() string { return product.description }

// Balance returns the available product balance.
func (product Product) Balance() int64 { return product.balance }

// Version returns the optimistic-lock version.
func (product Product) Version() int64 { return product.version }

// CreatedAt returns when the product was created.
func (product Product) CreatedAt() time.Time { return product.createdAt }

// UpdatedAt returns when the product was last updated.
func (product Product) UpdatedAt() time.Time { return product.updatedAt }

func validCode(code string) bool {
	if len(code) == 0 || len(code) > 64 {
		return false
	}
	for index := range len(code) {
		character := code[index]
		if (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			character == '.' || character == '_' || character == '-' {
			continue
		}
		return false
	}
	return true
}
