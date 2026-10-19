// Package ledger implements LedgerLite's double-entry core (SVC-002, SVC-003).
package ledger

import (
	"errors"
	"time"
)

// FundingAccountID is the system account every deposit is drawn from, so the
// sum of all balances stays zero (BR-001, ADR-004).
const FundingAccountID = "acc_funding"

// Domain errors. The HTTP layer maps each one to a stable machine-readable code (NFR-006).
var (
	ErrInvalidAmount          = errors.New("amount must be a positive integer number of minor units")
	ErrSameAccount            = errors.New("source and destination accounts must differ")
	ErrAccountNotFound        = errors.New("account not found")
	ErrAccountExists          = errors.New("account already exists")
	ErrInvalidAccount         = errors.New("invalid account definition")
	ErrTransferNotFound       = errors.New("transfer not found")
	ErrInsufficientFunds      = errors.New("insufficient funds")
	ErrDailyLimitExceeded     = errors.New("daily transfer limit exceeded")
	ErrIdempotencyKeyRequired = errors.New("idempotency key is required")
	ErrIdempotencyConflict    = errors.New("idempotency key already used with a different request")
	ErrUnbalancedPosting      = errors.New("posting entries do not sum to zero")
)

// Account is ENT-001. Amounts are integer minor units (ADR-001).
type Account struct {
	ID             string    `json:"id"`
	Owner          string    `json:"owner"`
	Balance        int64     `json:"balance"`
	AllowOverdraft bool      `json:"allow_overdraft"`
	DailyLimit     int64     `json:"daily_limit"` // 0 means unlimited
	CreatedAt      time.Time `json:"created_at"`
}

// Transfer is ENT-002.
type Transfer struct {
	ID             string    `json:"id"`
	IdempotencyKey string    `json:"idempotency_key"`
	From           string    `json:"from"`
	To             string    `json:"to"`
	Amount         int64     `json:"amount"`
	CreatedAt      time.Time `json:"created_at"`
}

// Entry is ENT-003, one signed leg of a transfer in the append-only journal.
type Entry struct {
	Seq        int64     `json:"seq"`
	TransferID string    `json:"transfer_id"`
	AccountID  string    `json:"account_id"`
	Delta      int64     `json:"delta"`
	At         time.Time `json:"at"`
}

// Posting is the atomic unit written to a Store: one transfer and its legs.
type Posting struct {
	Transfer Transfer
	Entries  []Entry
}

// NewAccount is the input of Service.CreateAccount.
type NewAccount struct {
	Owner          string
	AllowOverdraft bool
	DailyLimit     int64 // 0 selects the service default
	InitialDeposit int64
}

// TransferRequest is the input of Service.Transfer.
type TransferRequest struct {
	IdempotencyKey string
	From           string
	To             string
	Amount         int64
}
