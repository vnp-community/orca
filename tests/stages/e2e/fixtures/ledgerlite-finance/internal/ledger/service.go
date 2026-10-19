package ledger

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Config tunes a Service.
type Config struct {
	DefaultDailyLimit int64            // applied when an account is created without a limit
	Clock             func() time.Time // injectable for tests; defaults to time.Now
}

// Service is SVC-002: it validates and posts transfers. One mutex serializes
// writes so balance and limit checks cannot race (ADR-003, NFR-002).
type Service struct {
	mu    sync.Mutex
	store Store
	cfg   Config
	seq   int64
}

// NewService builds a Service and makes sure the funding account exists.
func NewService(store Store, cfg Config) (*Service, error) {
	if cfg.Clock == nil {
		cfg.Clock = time.Now
	}
	if cfg.DefaultDailyLimit <= 0 {
		cfg.DefaultDailyLimit = 1_000_000
	}
	s := &Service{store: store, cfg: cfg}
	err := store.CreateAccount(Account{ID: FundingAccountID, Owner: "system", AllowOverdraft: true, CreatedAt: cfg.Clock().UTC()})
	if err != nil && err != ErrAccountExists {
		return nil, err
	}
	return s, nil
}

// CreateAccount implements FR-001.
func (s *Service) CreateAccount(in NewAccount) (Account, error) {
	owner := strings.TrimSpace(in.Owner)
	if owner == "" || in.DailyLimit < 0 || in.InitialDeposit < 0 {
		return Account{}, ErrInvalidAccount
	}
	limit := in.DailyLimit
	if limit == 0 {
		limit = s.cfg.DefaultDailyLimit
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	acc := Account{ID: fmt.Sprintf("acc_%d", s.seq), Owner: owner, AllowOverdraft: in.AllowOverdraft, DailyLimit: limit, CreatedAt: s.cfg.Clock().UTC()}
	if err := s.store.CreateAccount(acc); err != nil {
		return Account{}, err
	}
	if in.InitialDeposit > 0 {
		req := TransferRequest{IdempotencyKey: "open-" + acc.ID, From: FundingAccountID, To: acc.ID, Amount: in.InitialDeposit}
		if _, _, err := s.transferLocked(req); err != nil {
			return Account{}, err
		}
	}
	return s.store.Account(acc.ID)
}

// Account implements FR-002.
func (s *Service) Account(id string) (Account, error) { return s.store.Account(id) }

// GetTransfer implements FR-006.
func (s *Service) GetTransfer(id string) (Transfer, error) { return s.store.Transfer(id) }

// Journal implements FR-005: the append-only entries, optionally of one account.
func (s *Service) Journal(accountID string) []Entry {
	return s.store.EntriesSince(accountID, time.Time{})
}

// Transfer implements FR-003 and FR-004. The bool is true when the result is
// a replay of an earlier request with the same idempotency key.
func (s *Service) Transfer(req TransferRequest) (Transfer, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.transferLocked(req)
}

func (s *Service) transferLocked(req TransferRequest) (Transfer, bool, error) {
	if strings.TrimSpace(req.IdempotencyKey) == "" {
		return Transfer{}, false, ErrIdempotencyKeyRequired
	}
	if prev, ok := s.store.TransferByKey(req.IdempotencyKey); ok { // BR-004
		if prev.From == req.From && prev.To == req.To && prev.Amount == req.Amount {
			return prev, true, nil
		}
		return Transfer{}, false, ErrIdempotencyConflict
	}
	if req.Amount <= 0 { // BR-006
		return Transfer{}, false, ErrInvalidAmount
	}
	if req.From == req.To {
		return Transfer{}, false, ErrSameAccount
	}
	from, err := s.store.Account(req.From)
	if err != nil {
		return Transfer{}, false, err
	}
	if _, err := s.store.Account(req.To); err != nil {
		return Transfer{}, false, err
	}
	if !from.AllowOverdraft && from.Balance-req.Amount < 0 { // BR-002
		return Transfer{}, false, ErrInsufficientFunds
	}
	now := s.cfg.Clock().UTC()
	if from.DailyLimit > 0 && s.sentToday(from.ID, now)+req.Amount > from.DailyLimit { // BR-003
		return Transfer{}, false, ErrDailyLimitExceeded
	}
	s.seq++
	t := Transfer{ID: fmt.Sprintf("txn_%d", s.seq), IdempotencyKey: req.IdempotencyKey, From: req.From, To: req.To, Amount: req.Amount, CreatedAt: now}
	_, err = s.store.Post(Posting{Transfer: t, Entries: []Entry{
		{TransferID: t.ID, AccountID: req.From, Delta: -req.Amount, At: now},
		{TransferID: t.ID, AccountID: req.To, Delta: req.Amount, At: now},
	}})
	if err != nil {
		return Transfer{}, false, err
	}
	return t, false, nil
}

// sentToday sums what the account sent during the UTC calendar day of now (ADR-005).
func (s *Service) sentToday(accountID string, now time.Time) int64 {
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	var sent int64
	for _, e := range s.store.EntriesSince(accountID, dayStart) {
		if e.Delta < 0 {
			sent += -e.Delta
		}
	}
	return sent
}
