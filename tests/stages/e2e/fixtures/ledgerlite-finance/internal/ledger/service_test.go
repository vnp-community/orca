package ledger

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// fakeClock is a settable clock so day-boundary rules are deterministic.
type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time          { return c.now }
func (c *fakeClock) Advance(d time.Duration) { c.now = c.now.Add(d) }

func newTestService(t *testing.T) (*Service, *fakeClock) {
	t.Helper()
	clock := &fakeClock{now: time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)}
	svc, err := NewService(NewMemoryStore(), Config{DefaultDailyLimit: 10_000, Clock: clock.Now})
	if err != nil {
		t.Fatal(err)
	}
	return svc, clock
}

func mustAccount(t *testing.T, svc *Service, in NewAccount) Account {
	t.Helper()
	acc, err := svc.CreateAccount(in)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	return acc
}

// FR-001: creating an account with an initial deposit funds it from the funding account.
func TestCreateAccountWithInitialDeposit(t *testing.T) {
	svc, _ := newTestService(t)
	acc := mustAccount(t, svc, NewAccount{Owner: "alice", InitialDeposit: 5_000})
	if acc.Balance != 5_000 || acc.DailyLimit != 10_000 {
		t.Fatalf("got balance %d limit %d, want 5000 and the default 10000", acc.Balance, acc.DailyLimit)
	}
	funding, _ := svc.Account(FundingAccountID)
	if funding.Balance != -5_000 {
		t.Fatalf("funding balance %d, want -5000", funding.Balance)
	}
}

// FR-001: invalid definitions are rejected.
func TestCreateAccountRejectsInvalidInput(t *testing.T) {
	svc, _ := newTestService(t)
	for _, in := range []NewAccount{{Owner: "  "}, {Owner: "a", DailyLimit: -1}, {Owner: "a", InitialDeposit: -5}} {
		if _, err := svc.CreateAccount(in); !errors.Is(err, ErrInvalidAccount) {
			t.Errorf("%+v: got %v, want ErrInvalidAccount", in, err)
		}
	}
}

// FR-003 and BR-001: a transfer posts two legs that sum to zero.
func TestTransferPostsBalancedEntries(t *testing.T) {
	svc, _ := newTestService(t)
	a := mustAccount(t, svc, NewAccount{Owner: "a", InitialDeposit: 1_000})
	b := mustAccount(t, svc, NewAccount{Owner: "b"})
	if _, _, err := svc.Transfer(TransferRequest{IdempotencyKey: "k1", From: a.ID, To: b.ID, Amount: 400}); err != nil {
		t.Fatal(err)
	}
	var sum int64
	for _, e := range svc.Journal("") {
		sum += e.Delta
	}
	if sum != 0 {
		t.Fatalf("journal sums to %d, want 0", sum)
	}
	if got, _ := svc.Account(a.ID); got.Balance != 600 {
		t.Errorf("source balance %d, want 600", got.Balance)
	}
	if got, _ := svc.Account(b.ID); got.Balance != 400 {
		t.Errorf("destination balance %d, want 400", got.Balance)
	}
}

// BR-002: no overdraft unless the account allows it.
func TestInsufficientFundsAndOverdraft(t *testing.T) {
	svc, _ := newTestService(t)
	a := mustAccount(t, svc, NewAccount{Owner: "a", InitialDeposit: 100})
	od := mustAccount(t, svc, NewAccount{Owner: "od", AllowOverdraft: true})
	if _, _, err := svc.Transfer(TransferRequest{IdempotencyKey: "k1", From: a.ID, To: od.ID, Amount: 101}); !errors.Is(err, ErrInsufficientFunds) {
		t.Fatalf("got %v, want ErrInsufficientFunds", err)
	}
	if _, _, err := svc.Transfer(TransferRequest{IdempotencyKey: "k2", From: od.ID, To: a.ID, Amount: 50}); err != nil {
		t.Fatalf("overdraft account should be allowed to go negative: %v", err)
	}
	if got, _ := svc.Account(od.ID); got.Balance != -50 {
		t.Errorf("overdraft balance %d, want -50", got.Balance)
	}
}

// BR-006: amount and account validation.
func TestTransferValidation(t *testing.T) {
	svc, _ := newTestService(t)
	a := mustAccount(t, svc, NewAccount{Owner: "a", InitialDeposit: 100})
	b := mustAccount(t, svc, NewAccount{Owner: "b"})
	cases := []struct {
		name string
		req  TransferRequest
		want error
	}{
		{"zero", TransferRequest{IdempotencyKey: "z", From: a.ID, To: b.ID, Amount: 0}, ErrInvalidAmount},
		{"negative", TransferRequest{IdempotencyKey: "n", From: a.ID, To: b.ID, Amount: -1}, ErrInvalidAmount},
		{"same", TransferRequest{IdempotencyKey: "s", From: a.ID, To: a.ID, Amount: 1}, ErrSameAccount},
		{"unknown source", TransferRequest{IdempotencyKey: "u1", From: "nope", To: b.ID, Amount: 1}, ErrAccountNotFound},
		{"unknown destination", TransferRequest{IdempotencyKey: "u2", From: a.ID, To: "nope", Amount: 1}, ErrAccountNotFound},
		{"no key", TransferRequest{From: a.ID, To: b.ID, Amount: 1}, ErrIdempotencyKeyRequired},
	}
	for _, c := range cases {
		if _, _, err := svc.Transfer(c.req); !errors.Is(err, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, err, c.want)
		}
	}
}

// FR-004 and BR-004: replays return the original transfer and post nothing; a different payload conflicts.
func TestIdempotency(t *testing.T) {
	svc, _ := newTestService(t)
	a := mustAccount(t, svc, NewAccount{Owner: "a", InitialDeposit: 1_000})
	b := mustAccount(t, svc, NewAccount{Owner: "b"})
	req := TransferRequest{IdempotencyKey: "pay-1", From: a.ID, To: b.ID, Amount: 300}
	first, replayed, err := svc.Transfer(req)
	if err != nil || replayed {
		t.Fatalf("first call: replayed=%v err=%v", replayed, err)
	}
	second, replayed, err := svc.Transfer(req)
	if err != nil || !replayed || second.ID != first.ID {
		t.Fatalf("replay: id=%s replayed=%v err=%v, want id %s replayed", second.ID, replayed, err, first.ID)
	}
	if got, _ := svc.Account(a.ID); got.Balance != 700 {
		t.Errorf("replay moved money again: balance %d, want 700", got.Balance)
	}
	req.Amount = 301
	if _, _, err := svc.Transfer(req); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("got %v, want ErrIdempotencyConflict", err)
	}
}

// BR-003: the limit is inclusive and counts only the current UTC day.
func TestDailyLimitIsInclusiveAndResetsAtUTCMidnight(t *testing.T) {
	svc, clock := newTestService(t)
	a := mustAccount(t, svc, NewAccount{Owner: "a", InitialDeposit: 50_000, DailyLimit: 1_000})
	b := mustAccount(t, svc, NewAccount{Owner: "b"})
	send := func(key string, amount int64) error {
		_, _, err := svc.Transfer(TransferRequest{IdempotencyKey: key, From: a.ID, To: b.ID, Amount: amount})
		return err
	}
	if err := send("d1", 600); err != nil {
		t.Fatal(err)
	}
	if err := send("d2", 401); !errors.Is(err, ErrDailyLimitExceeded) {
		t.Fatalf("401 over 600 used: got %v, want ErrDailyLimitExceeded", err)
	}
	if err := send("d3", 400); err != nil {
		t.Fatalf("a transfer that reaches the limit exactly must be allowed: %v", err)
	}
	if err := send("d4", 1); !errors.Is(err, ErrDailyLimitExceeded) {
		t.Fatalf("got %v, want ErrDailyLimitExceeded", err)
	}
	clock.Advance(15*time.Hour + time.Minute) // 2026-03-11 00:01 UTC
	if err := send("d5", 1_000); err != nil {
		t.Fatalf("a new UTC day resets the window: %v", err)
	}
}

// BR-003: received money does not consume the limit.
func TestDailyLimitCountsOnlyOutgoing(t *testing.T) {
	svc, _ := newTestService(t)
	a := mustAccount(t, svc, NewAccount{Owner: "a", InitialDeposit: 1_000, DailyLimit: 500})
	b := mustAccount(t, svc, NewAccount{Owner: "b", InitialDeposit: 1_000, DailyLimit: 500})
	if _, _, err := svc.Transfer(TransferRequest{IdempotencyKey: "in", From: b.ID, To: a.ID, Amount: 500}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Transfer(TransferRequest{IdempotencyKey: "out", From: a.ID, To: b.ID, Amount: 500}); err != nil {
		t.Fatalf("received funds must not count against a's limit: %v", err)
	}
}

// BR-005: the Store exposes entries read-only and returns copies.
func TestJournalIsAppendOnly(t *testing.T) {
	svc, _ := newTestService(t)
	a := mustAccount(t, svc, NewAccount{Owner: "a", InitialDeposit: 100})
	before := svc.Journal(a.ID)
	before[0].Delta = 999_999
	after := svc.Journal(a.ID)
	if after[0].Delta != 100 {
		t.Fatalf("journal entry changed through a returned slice: %d", after[0].Delta)
	}
	if after[0].Seq != 2 { // seq 1 is the funding account's leg of the same deposit
		t.Errorf("first entry of the account has seq %d, want 2", after[0].Seq)
	}
}

// BR-001: the store refuses a posting that does not sum to zero.
func TestStoreRejectsUnbalancedPosting(t *testing.T) {
	st := NewMemoryStore()
	_ = st.CreateAccount(Account{ID: "x"})
	_, err := st.Post(Posting{Transfer: Transfer{ID: "t", IdempotencyKey: "k"}, Entries: []Entry{{AccountID: "x", Delta: 5}}})
	if !errors.Is(err, ErrUnbalancedPosting) {
		t.Fatalf("got %v, want ErrUnbalancedPosting", err)
	}
}

// NFR-002: concurrent transfers never overspend or break conservation. Run with -race.
func TestConcurrentTransfersKeepInvariants(t *testing.T) {
	svc, _ := newTestService(t)
	a := mustAccount(t, svc, NewAccount{Owner: "a", InitialDeposit: 1_000, DailyLimit: 1_000_000})
	b := mustAccount(t, svc, NewAccount{Owner: "b"})
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _, _ = svc.Transfer(TransferRequest{IdempotencyKey: fmt.Sprintf("c-%d", i), From: a.ID, To: b.ID, Amount: 10})
		}(i)
	}
	wg.Wait()
	ga, _ := svc.Account(a.ID)
	gb, _ := svc.Account(b.ID)
	if ga.Balance < 0 || ga.Balance+gb.Balance != 1_000 {
		t.Fatalf("balances a=%d b=%d, want a>=0 and a+b=1000", ga.Balance, gb.Balance)
	}
	if ga.Balance != 0 {
		t.Errorf("200 x 10 attempts against 1000 should drain a exactly, got %d", ga.Balance)
	}
}

// NFR-001: throughput benchmark for the transfer path, spread over 100 sender accounts.
func BenchmarkTransfer(b *testing.B) {
	svc, err := NewService(NewMemoryStore(), Config{})
	if err != nil {
		b.Fatal(err)
	}
	to, _ := svc.CreateAccount(NewAccount{Owner: "to"})
	senders := make([]string, 100)
	for i := range senders {
		acc, _ := svc.CreateAccount(NewAccount{Owner: "s", AllowOverdraft: true, DailyLimit: 1 << 60})
		senders[i] = acc.ID
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, err := svc.Transfer(TransferRequest{IdempotencyKey: fmt.Sprintf("b-%d", i), From: senders[i%100], To: to.ID, Amount: 1}); err != nil {
			b.Fatal(err)
		}
	}
}
