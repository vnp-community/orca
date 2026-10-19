package ledger

import (
	"sync"
	"time"
)

// Store is the persistence port (ADR-002). It deliberately has no update or
// delete for journal entries: the journal is append-only (BR-005).
type Store interface {
	CreateAccount(a Account) error
	Account(id string) (Account, error)
	Transfer(id string) (Transfer, error)
	TransferByKey(key string) (Transfer, bool)
	// Post atomically records the transfer, appends its entries, assigns their
	// sequence numbers and applies the balance changes.
	Post(p Posting) (Posting, error)
	// EntriesSince returns the account's entries with At >= since in append
	// order; an empty accountID selects every account.
	EntriesSince(accountID string, since time.Time) []Entry
}

// MemoryStore is the in-memory Store used by the server and the tests.
type MemoryStore struct {
	mu        sync.Mutex
	accounts  map[string]Account
	transfers map[string]Transfer
	byKey     map[string]string
	entries   []Entry
	byAccount map[string][]int // per-account index keeps the daily-limit scan short (NFR-001)
}

// NewMemoryStore returns an empty MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{accounts: map[string]Account{}, transfers: map[string]Transfer{}, byKey: map[string]string{}, byAccount: map[string][]int{}}
}

func (m *MemoryStore) CreateAccount(a Account) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.accounts[a.ID]; ok {
		return ErrAccountExists
	}
	m.accounts[a.ID] = a
	return nil
}

func (m *MemoryStore) Account(id string) (Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.accounts[id]
	if !ok {
		return Account{}, ErrAccountNotFound
	}
	return a, nil
}

func (m *MemoryStore) Transfer(id string) (Transfer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.transfers[id]
	if !ok {
		return Transfer{}, ErrTransferNotFound
	}
	return t, nil
}

func (m *MemoryStore) TransferByKey(key string) (Transfer, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.byKey[key]
	if !ok {
		return Transfer{}, false
	}
	return m.transfers[id], true
}

func (m *MemoryStore) Post(p Posting) (Posting, error) {
	var sum int64
	for _, e := range p.Entries {
		sum += e.Delta
	}
	if sum != 0 { // BR-001
		return Posting{}, ErrUnbalancedPosting
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range p.Entries {
		if _, ok := m.accounts[e.AccountID]; !ok {
			return Posting{}, ErrAccountNotFound
		}
	}
	out := Posting{Transfer: p.Transfer, Entries: make([]Entry, len(p.Entries))}
	for i, e := range p.Entries {
		a := m.accounts[e.AccountID]
		a.Balance += e.Delta
		m.accounts[e.AccountID] = a
		e.Seq = int64(len(m.entries)) + 1
		m.byAccount[e.AccountID] = append(m.byAccount[e.AccountID], len(m.entries))
		m.entries = append(m.entries, e)
		out.Entries[i] = e
	}
	m.transfers[p.Transfer.ID] = p.Transfer
	m.byKey[p.Transfer.IdempotencyKey] = p.Transfer.ID
	return out, nil
}

func (m *MemoryStore) EntriesSince(accountID string, since time.Time) []Entry {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Entry
	// Entries are appended in time order, so walk back and stop at the first older one.
	if accountID == "" {
		for i := len(m.entries) - 1; i >= 0 && !m.entries[i].At.Before(since); i-- {
			out = append(out, m.entries[i])
		}
	} else {
		idx := m.byAccount[accountID]
		for i := len(idx) - 1; i >= 0 && !m.entries[idx[i]].At.Before(since); i-- {
			out = append(out, m.entries[idx[i]])
		}
	}
	for l, r := 0, len(out)-1; l < r; l, r = l+1, r-1 {
		out[l], out[r] = out[r], out[l]
	}
	return out
}
