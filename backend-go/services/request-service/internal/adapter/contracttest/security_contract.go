package contracttest

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// SecurityEnv wires one dialect's CR-REQ-035 repositories. The dialect-specific hooks insert and read rows the
// repositories themselves cannot create (analysis runs, content in every erasable table).
type SecurityEnv struct {
	Base      Env
	Audit     usecase.AuditOutboxStore
	Nonces    usecase.WebhookNonceStore
	Flags     usecase.SecurityFlagStore
	Counts    usecase.ConcurrencyCounter
	Retention usecase.RetentionStore

	SeedRun     func(t *testing.T, tenantID, requestID, status string)
	SeedContent func(t *testing.T, tenantID, requestID string)
	// ReadColumn returns the value (as text) and whether it is NULL.
	ReadColumn func(t *testing.T, c domain.ErasableColumn, requestID string) (string, bool)
	// SeedSettings writes tenant_security_settings.
	SeedSettings func(t *testing.T, tenantID string, requestDays int)
}

func auditRec(tenantID string) usecase.AuditOutboxRecord {
	id := uuid.NewString()
	return usecase.AuditOutboxRecord{
		ID: uuid.NewString(), AuditID: id, TenantID: tenantID, Action: "request.erase", ActorID: "u", ActorType: "user",
		TargetType: "request", TargetID: "r1", Outcome: "allowed", MetadataJSON: `{"audit_id":"` + id + `"}`,
	}
}

func fixedBackoff(int) time.Duration { return time.Minute }

// RunSecurityContract runs the audit outbox, nonce, flag, concurrency and retention scenarios.
func RunSecurityContract(t *testing.T, newEnv func(t *testing.T) SecurityEnv) {
	for name, fn := range map[string]func(*testing.T, SecurityEnv){
		"AuditEnqueueRollsBackWithTheTransaction": auditEnqueueRollback,
		"AuditDeliversOnceAndMarksDelivered":      auditDeliverOnce,
		"AuditRetriesWithBackoff":                 auditRetries,
		"AuditLostNeverWhenAuthDown":              auditNotLostWhenAuthDown,
		"AuditSkipLockedTwoDeliverers":            auditSkipLocked,
		"AuditAuditIDIsUnique":                    auditUniqueAuditID,
		"AuditTenantMismatchRejected":             auditTenantMismatch,
		"AuditPurgeKeepsUndelivered":              auditPurge,
		"NonceFirstSeenReplayExpiry":              nonceLifecycle,
		"NonceTenantAndSourceIsolation":           nonceIsolation,
		"FlagsRoundTripAndTenantIsolation":        flagsRoundTrip,
		"ConcurrencyCounts":                       concurrencyCounts,
		"AnonymizeClearsAllErasableColumns":       anonymizeClearsAll,
		"AnonymizeExpiredOnlyFinishedAndOld":      anonymizeExpiredSelection,
		"AnonymizeExpiredTwoReplicasNoOverlap":    anonymizeTwoReplicas,
		"RetentionSettingsDefaultsAndOverride":    retentionSettings,
		"ListTenantsAndCrossTenantErase":          listTenantsAndIsolation,
	} {
		fn := fn
		t.Run(name, func(t *testing.T) { fn(t, newEnv(t)) })
	}
}

func auditEnqueueRollback(t *testing.T, env SecurityEnv) {
	tenantID := newTenant()
	ctx := CtxForTenant(tenantID)
	boom := errors.New("change failed")
	err := env.Base.Tx.InTx(ctx, func(txCtx context.Context) error {
		if err := env.Audit.Enqueue(txCtx, auditRec(tenantID)); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("got %v", err)
	}
	if n, _ := env.Audit.CountPending(ctx); n != 0 {
		t.Fatalf("a rolled back change left %d audit rows", n)
	}
	if err := env.Base.Tx.InTx(ctx, func(txCtx context.Context) error { return env.Audit.Enqueue(txCtx, auditRec(tenantID)) }); err != nil {
		t.Fatal(err)
	}
	if n, _ := env.Audit.CountPending(ctx); n != 1 {
		t.Fatalf("committed entry missing: %d", n)
	}
}

func enqueue(t *testing.T, env SecurityEnv, tenantID string, n int) []usecase.AuditOutboxRecord {
	t.Helper()
	var out []usecase.AuditOutboxRecord
	for i := 0; i < n; i++ {
		r := auditRec(tenantID)
		if err := env.Audit.Enqueue(CtxForTenant(tenantID), r); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

func auditDeliverOnce(t *testing.T, env SecurityEnv) {
	a, b := newTenant(), newTenant()
	enqueue(t, env, a, 2)
	enqueue(t, env, b, 1)
	var mu sync.Mutex
	seen := map[string]int{}
	deliver := func(_ context.Context, r usecase.AuditOutboxRecord) error {
		mu.Lock()
		seen[r.AuditID]++
		mu.Unlock()
		return nil
	}
	ok, failed, err := env.Audit.ProcessDue(context.Background(), time.Now().Add(time.Second), 100, fixedBackoff, deliver)
	if err != nil || ok != 3 || failed != 0 {
		t.Fatalf("delivered=%d failed=%d err=%v (the relay serves every tenant)", ok, failed, err)
	}
	ok, _, _ = env.Audit.ProcessDue(context.Background(), time.Now().Add(time.Second), 100, fixedBackoff, deliver)
	if ok != 0 {
		t.Fatalf("delivered rows are delivered once, second pass delivered %d", ok)
	}
	for id, n := range seen {
		if n != 1 {
			t.Errorf("%s delivered %d times", id, n)
		}
	}
}

func auditRetries(t *testing.T, env SecurityEnv) {
	tenantID := newTenant()
	rec := enqueue(t, env, tenantID, 1)[0]
	fails := 2
	var attemptsSeen []int
	deliver := func(_ context.Context, r usecase.AuditOutboxRecord) error {
		attemptsSeen = append(attemptsSeen, r.Attempts)
		if fails > 0 {
			fails--
			return errors.New("auth down")
		}
		return nil
	}
	now := time.Now().Add(time.Second)
	for i := 0; i < 2; i++ {
		_, failed, err := env.Audit.ProcessDue(context.Background(), now, 10, fixedBackoff, deliver)
		if err != nil || failed != 1 {
			t.Fatalf("pass %d failed=%d err=%v", i, failed, err)
		}
		// Not due again until the backoff has passed.
		if ok, f, _ := env.Audit.ProcessDue(context.Background(), now, 10, fixedBackoff, deliver); ok+f != 0 {
			t.Fatalf("a failed row must wait for its backoff (delivered=%d failed=%d)", ok, f)
		}
		now = now.Add(2 * time.Minute)
	}
	ok, _, err := env.Audit.ProcessDue(context.Background(), now, 10, fixedBackoff, deliver)
	if err != nil || ok != 1 {
		t.Fatalf("third attempt delivers: ok=%d err=%v", ok, err)
	}
	if fmt.Sprint(attemptsSeen) != "[0 1 2]" {
		t.Errorf("attempts seen %v, want [0 1 2] for %s", attemptsSeen, rec.AuditID)
	}
}

func auditNotLostWhenAuthDown(t *testing.T, env SecurityEnv) {
	tenantID := newTenant()
	ctx := CtxForTenant(tenantID)
	enqueue(t, env, tenantID, 1)
	down := func(context.Context, usecase.AuditOutboxRecord) error { return errors.New("auth-service unavailable") }
	now := time.Now().Add(time.Second)
	for i := 0; i < 3; i++ {
		_, _, _ = env.Audit.ProcessDue(context.Background(), now, 10, fixedBackoff, down)
		now = now.Add(2 * time.Minute)
	}
	if n, _ := env.Audit.CountPending(ctx); n != 1 {
		t.Fatalf("the entry must stay queued while auth is down, pending=%d", n)
	}
	ok, _, _ := env.Audit.ProcessDue(context.Background(), now, 10, fixedBackoff, func(context.Context, usecase.AuditOutboxRecord) error { return nil })
	if n, _ := env.Audit.CountPending(ctx); ok != 1 || n != 0 {
		t.Fatalf("delivered after recovery: ok=%d pending=%d", ok, n)
	}
}

func auditSkipLocked(t *testing.T, env SecurityEnv) {
	tenantID := newTenant()
	enqueue(t, env, tenantID, 10)
	var mu sync.Mutex
	seen := map[string]int{}
	holding := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	slow := func(_ context.Context, r usecase.AuditOutboxRecord) error {
		once.Do(func() { close(holding) })
		<-release
		mu.Lock()
		seen[r.AuditID]++
		mu.Unlock()
		return nil
	}
	fast := func(_ context.Context, r usecase.AuditOutboxRecord) error {
		mu.Lock()
		seen[r.AuditID]++
		mu.Unlock()
		return nil
	}
	now := time.Now().Add(time.Second)
	var wg sync.WaitGroup
	var firstOK int
	wg.Add(1)
	go func() {
		defer wg.Done()
		firstOK, _, _ = env.Audit.ProcessDue(context.Background(), now, 6, fixedBackoff, slow)
	}()
	<-holding // the first deliverer holds six locked rows
	secondOK, _, err := env.Audit.ProcessDue(context.Background(), now, 10, fixedBackoff, fast)
	if err != nil {
		t.Fatal(err)
	}
	close(release)
	wg.Wait()
	// The second pass may find fewer than 4 free rows (InnoDB can lock more than LIMIT on a tiny table), never more.
	if firstOK != 6 || secondOK > 4 {
		t.Fatalf("locked rows must be skipped: first=%d second=%d", firstOK, secondOK)
	}
	drained, _, err := env.Audit.ProcessDue(context.Background(), now, 10, fixedBackoff, fast)
	if err != nil || secondOK+drained != 4 {
		t.Fatalf("the remaining rows are delivered afterwards: second=%d drained=%d err=%v", secondOK, drained, err)
	}
	if len(seen) != 10 {
		t.Fatalf("%d distinct entries delivered, want 10", len(seen))
	}
	for id, n := range seen {
		if n != 1 {
			t.Errorf("%s delivered %d times", id, n)
		}
	}
}

func auditUniqueAuditID(t *testing.T, env SecurityEnv) {
	tenantID := newTenant()
	r := auditRec(tenantID)
	ctx := CtxForTenant(tenantID)
	if err := env.Audit.Enqueue(ctx, r); err != nil {
		t.Fatal(err)
	}
	dup := r
	dup.ID = uuid.NewString()
	if err := env.Audit.Enqueue(ctx, dup); err == nil {
		t.Fatal("the same audit_id twice must be refused so a retry cannot double an entry")
	}
}

func auditTenantMismatch(t *testing.T, env SecurityEnv) {
	if err := env.Audit.Enqueue(CtxForTenant(newTenant()), auditRec(newTenant())); err == nil {
		t.Fatal("an entry for another tenant must be refused")
	}
}

func auditPurge(t *testing.T, env SecurityEnv) {
	tenantID := newTenant()
	enqueue(t, env, tenantID, 2)
	now := time.Now().Add(time.Second)
	first := true
	_, _, _ = env.Audit.ProcessDue(context.Background(), now, 1, fixedBackoff, func(context.Context, usecase.AuditOutboxRecord) error {
		if first {
			first = false
			return nil
		}
		return errors.New("x")
	})
	n, err := env.Audit.PurgeDelivered(context.Background(), now.Add(time.Hour), 100)
	if err != nil || n != 1 {
		t.Fatalf("purge removed %d (err %v), want only the delivered one", n, err)
	}
	if p, _ := env.Audit.CountPending(CtxForTenant(tenantID)); p != 1 {
		t.Fatalf("undelivered entry must survive the purge, pending=%d", p)
	}
}

func nonceLifecycle(t *testing.T, env SecurityEnv) {
	tenantID := newTenant()
	ctx := CtxForTenant(tenantID)
	hash := strings.Repeat("a", 64)
	ok, err := env.Nonces.Remember(ctx, tenantID, "sentry", hash, time.Now().Add(10*time.Minute))
	if err != nil || !ok {
		t.Fatalf("first sighting: %v %v", ok, err)
	}
	if ok, _ := env.Nonces.Remember(ctx, tenantID, "sentry", hash, time.Now().Add(10*time.Minute)); ok {
		t.Fatal("replay accepted")
	}
	// An expired nonce no longer blocks, and can be remembered again.
	old := strings.Repeat("b", 64)
	if ok, _ := env.Nonces.Remember(ctx, tenantID, "sentry", old, time.Now().Add(-time.Minute)); !ok {
		t.Fatal("first sighting of the old nonce")
	}
	if ok, _ := env.Nonces.Remember(ctx, tenantID, "sentry", old, time.Now().Add(10*time.Minute)); !ok {
		t.Fatal("an expired nonce must be accepted again")
	}
	if ok, _ := env.Nonces.Remember(ctx, tenantID, "sentry", old, time.Now().Add(10*time.Minute)); ok {
		t.Fatal("and then it blocks again")
	}
	// Prune removes expired rows only.
	gone := strings.Repeat("c", 64)
	_, _ = env.Nonces.Remember(ctx, tenantID, "sentry", gone, time.Now().Add(-time.Hour))
	n, err := env.Nonces.PruneExpired(context.Background(), time.Now(), 100)
	if err != nil || n < 1 {
		t.Fatalf("prune removed %d (err %v)", n, err)
	}
	if ok, _ := env.Nonces.Remember(ctx, tenantID, "sentry", hash, time.Now().Add(10*time.Minute)); ok {
		t.Fatal("a live nonce must survive the prune")
	}
}

func nonceIsolation(t *testing.T, env SecurityEnv) {
	a, b := newTenant(), newTenant()
	hash := strings.Repeat("d", 64)
	exp := time.Now().Add(10 * time.Minute)
	if ok, _ := env.Nonces.Remember(CtxForTenant(a), a, "s1", hash, exp); !ok {
		t.Fatal("tenant A first")
	}
	if ok, _ := env.Nonces.Remember(CtxForTenant(b), b, "s1", hash, exp); !ok {
		t.Fatal("tenant B has its own nonce space")
	}
	if ok, _ := env.Nonces.Remember(CtxForTenant(a), a, "s2", hash, exp); !ok {
		t.Fatal("another source has its own nonce space")
	}
	if _, err := env.Nonces.Remember(CtxForTenant(a), b, "s1", hash, exp); err == nil {
		t.Fatal("a nonce for a tenant other than the context tenant must be refused")
	}
}

func flagsRoundTrip(t *testing.T, env SecurityEnv) {
	a, b := newTenant(), newTenant()
	ctxA, ctxB := CtxForTenant(a), CtxForTenant(b)
	rA := createRequest(t, env.Base, ctxA, nil)
	if f, err := env.Flags.Get(ctxA, rA.ID); err != nil || f.ContainsSecretSuspected || f.ErasedAt != nil {
		t.Fatalf("default flags: %+v %v", f, err)
	}
	if err := env.Flags.MarkSecretSuspected(ctxA, rA.ID); err != nil {
		t.Fatal(err)
	}
	if err := env.Flags.MarkSecretSuspected(ctxA, rA.ID); err != nil {
		t.Fatalf("marking twice is idempotent: %v", err)
	}
	if f, _ := env.Flags.Get(ctxA, rA.ID); !f.ContainsSecretSuspected {
		t.Fatal("flag not stored")
	}
	if f, _ := env.Flags.Get(ctxB, rA.ID); f.ContainsSecretSuspected {
		t.Fatal("tenant B must not see tenant A's flag")
	}
}

func concurrencyCounts(t *testing.T, env SecurityEnv) {
	a, b := newTenant(), newTenant()
	ctxA, ctxB := CtxForTenant(a), CtxForTenant(b)
	project1, project2 := uuid.NewString(), uuid.NewString()
	mk := func(ctx context.Context, project, status string) domain.Request {
		return createRequest(t, env.Base, ctx, func(r *domain.Request) {
			r.ProjectID = project
			if status == "completed" {
				r.Status = domain.RequestStatusCompleted
			}
		})
	}
	r1, r2, r3 := mk(ctxA, project1, "new"), mk(ctxA, project1, "new"), mk(ctxA, project2, "new")
	done := mk(ctxA, project1, "completed")
	rb := mk(ctxB, project1, "new")
	env.SeedRun(t, a, r1.ID, "running")
	env.SeedRun(t, a, r2.ID, "running")
	env.SeedRun(t, a, r3.ID, "running")
	env.SeedRun(t, a, done.ID, "succeeded")
	env.SeedRun(t, b, rb.ID, "running")

	if n, err := env.Counts.CountRunning(ctxA, ""); err != nil || n != 3 {
		t.Fatalf("running per tenant = %d (%v), want 3", n, err)
	}
	if n, _ := env.Counts.CountRunning(ctxA, project1); n != 2 {
		t.Fatalf("running per project = %d, want 2", n)
	}
	if n, _ := env.Counts.CountRunning(ctxB, ""); n != 1 {
		t.Fatalf("tenant B counts only its own runs, got %d", n)
	}
	if n, _ := env.Counts.CountOpenRequests(ctxA); n != 3 {
		t.Fatalf("open requests = %d, want 3 (the completed one is not open)", n)
	}
	if n, _ := env.Counts.CountOpenRequests(ctxB); n != 1 {
		t.Fatalf("tenant B open = %d", n)
	}
}

func finishedRequest(t *testing.T, env SecurityEnv, ctx context.Context, status domain.RequestStatus, age time.Duration) domain.Request {
	t.Helper()
	return createRequest(t, env.Base, ctx, func(r *domain.Request) {
		r.Status = status
		r.UpdatedAt = time.Now().UTC().Add(-age)
		r.CreatedAt = r.UpdatedAt
	})
}

func pseudo(r string) string {
	return "00000000-0000-4000-8000-" + strings.Repeat("0", 11) + string(r[0])
}

func anonymizeClearsAll(t *testing.T, env SecurityEnv) {
	tenantID, other := newTenant(), newTenant()
	ctx := CtxForTenant(tenantID)
	req := finishedRequest(t, env, ctx, domain.RequestStatusCompleted, 800*24*time.Hour)
	bystander := finishedRequest(t, env, CtxForTenant(other), domain.RequestStatusCompleted, 800*24*time.Hour)
	env.SeedContent(t, tenantID, req.ID)
	env.SeedContent(t, other, bystander.ID)
	for _, c := range domain.ErasableColumns {
		key := req.ID
		if v, null := env.ReadColumn(t, c, key); null || v == "" || v == "[]" || v == "{}" {
			t.Fatalf("seed for %s.%s is empty (%q, null=%v): the test would prove nothing", c.Table, c.Column, v, null)
		}
	}
	reporter := req.ReporterID

	changed, err := env.Retention.Anonymize(ctx, req.ID, func(string) string { return "99999999-9999-4999-8999-999999999999" }, time.Now().UTC(), uuid.NewString())
	if err != nil || !changed {
		t.Fatalf("anonymize: changed=%v err=%v", changed, err)
	}
	for _, c := range domain.ErasableColumns {
		v, null := env.ReadColumn(t, c, req.ID)
		switch c.Mode {
		case domain.EraseClearText:
			if null || v != "" {
				t.Errorf("%s.%s = %q null=%v, want empty string", c.Table, c.Column, v, null)
			}
		case domain.EraseMarker:
			if v != "[erased]" {
				t.Errorf("%s.%s = %q, want [erased]", c.Table, c.Column, v)
			}
		case domain.EraseNull:
			if !null {
				t.Errorf("%s.%s = %q, want NULL", c.Table, c.Column, v)
			}
		case domain.EraseClearJSONArray:
			if strings.ReplaceAll(v, " ", "") != "[]" {
				t.Errorf("%s.%s = %q, want []", c.Table, c.Column, v)
			}
		case domain.EraseClearJSONObject:
			if strings.ReplaceAll(v, " ", "") != "{}" {
				t.Errorf("%s.%s = %q, want {}", c.Table, c.Column, v)
			}
		}
		// The other tenant's identical content is untouched.
		if bv, bnull := env.ReadColumn(t, c, bystander.ID); bnull || bv == "" || bv == "[]" || bv == "{}" {
			t.Errorf("%s.%s of another tenant was changed to %q", c.Table, c.Column, bv)
		}
	}
	got, err := env.Base.Requests.Get(ctx, req.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ReporterID == reporter || got.ReporterID != "99999999-9999-4999-8999-999999999999" {
		t.Errorf("reporter_id = %s, want the pseudonym", got.ReporterID)
	}
	if got.ID != req.ID || got.Status != req.Status || got.Number != req.Number || !withinMicro(got.CreatedAt, req.CreatedAt) || got.Version != req.Version+1 {
		t.Errorf("id, status, number and created_at survive and version bumps once: %+v vs %+v", got, req)
	}
	f, _ := env.Flags.Get(ctx, req.ID)
	if f.ErasedAt == nil || f.ErasedBy == "" {
		t.Errorf("erase marker missing: %+v", f)
	}
	if again, err := env.Retention.Anonymize(ctx, req.ID, pseudo, time.Now(), ""); err != nil || again {
		t.Errorf("a second erase is a no-op: changed=%v err=%v", again, err)
	}
	if _, err := env.Retention.Anonymize(ctx, uuid.NewString(), pseudo, time.Now(), ""); err == nil {
		t.Error("erasing an unknown Request must fail")
	}
	if _, err := env.Retention.Anonymize(CtxForTenant(other), req.ID, pseudo, time.Now(), ""); err == nil {
		t.Error("another tenant cannot erase this Request")
	}
}

func anonymizeExpiredSelection(t *testing.T, env SecurityEnv) {
	tenantID := newTenant()
	ctx := CtxForTenant(tenantID)
	cutoff := time.Now().UTC().Add(-730 * 24 * time.Hour)
	oldDone := finishedRequest(t, env, ctx, domain.RequestStatusCompleted, 800*24*time.Hour)
	oldCancelled := finishedRequest(t, env, ctx, domain.RequestStatusCancelled, 800*24*time.Hour)
	oldExecuting := finishedRequest(t, env, ctx, domain.RequestStatusExecuting, 800*24*time.Hour)
	recentDone := finishedRequest(t, env, ctx, domain.RequestStatusCompleted, 10*24*time.Hour)
	for _, r := range []domain.Request{oldDone, oldCancelled, oldExecuting, recentDone} {
		env.SeedContent(t, tenantID, r.ID)
	}
	n, err := env.Retention.AnonymizeExpired(ctx, cutoff, 100, pseudo, time.Now().UTC())
	if err != nil || n != 2 {
		t.Fatalf("anonymized %d (err %v), want the 2 finished and old ones", n, err)
	}
	erased := func(id string) bool { f, _ := env.Flags.Get(ctx, id); return f.ErasedAt != nil }
	if !erased(oldDone.ID) || !erased(oldCancelled.ID) || erased(oldExecuting.ID) || erased(recentDone.ID) {
		t.Errorf("selection wrong: done=%v cancelled=%v executing=%v recent=%v", erased(oldDone.ID), erased(oldCancelled.ID), erased(oldExecuting.ID), erased(recentDone.ID))
	}
	if v, _ := env.ReadColumn(t, domain.ErasableColumn{Table: "requests", Column: "body", KeyColumn: "id"}, oldExecuting.ID); v == "" {
		t.Error("an executing Request keeps its content")
	}
	if n, err := env.Retention.AnonymizeExpired(ctx, cutoff, 100, pseudo, time.Now().UTC()); err != nil || n != 0 {
		t.Errorf("idempotent: second run anonymized %d (err %v)", n, err)
	}
}

func anonymizeTwoReplicas(t *testing.T, env SecurityEnv) {
	tenantID := newTenant()
	ctx := CtxForTenant(tenantID)
	const total = 40
	for i := 0; i < total; i++ {
		finishedRequest(t, env, ctx, domain.RequestStatusCompleted, 800*24*time.Hour)
	}
	cutoff := time.Now().UTC().Add(-730 * 24 * time.Hour)
	var wg sync.WaitGroup
	var mu sync.Mutex
	sum := 0
	errs := []error{}
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				var n int
				err := retryDeadlock(func() (e error) {
					n, e = env.Retention.AnonymizeExpired(ctx, cutoff, 7, pseudo, time.Now().UTC())
					return e
				})
				mu.Lock()
				sum += n
				if err != nil {
					errs = append(errs, err)
				}
				mu.Unlock()
				if n == 0 || err != nil {
					return
				}
			}
		}()
	}
	wg.Wait()
	if len(errs) > 0 || sum != total {
		t.Fatalf("two replicas anonymized %d of %d (errors %v): each Request exactly once", sum, total, errs)
	}
}

func retentionSettings(t *testing.T, env SecurityEnv) {
	tenantID := newTenant()
	ctx := CtxForTenant(tenantID)
	if s, err := env.Retention.Settings(ctx); err != nil || s != domain.DefaultRetention {
		t.Fatalf("defaults 730/30/400: %+v %v", s, err)
	}
	env.SeedSettings(t, tenantID, 90)
	if s, err := env.Retention.Settings(ctx); err != nil || s.RequestDays != 90 {
		t.Fatalf("override: %+v %v", s, err)
	}
	if s, _ := env.Retention.Settings(CtxForTenant(newTenant())); s.RequestDays != 730 {
		t.Fatalf("another tenant keeps the default, got %d", s.RequestDays)
	}
}

func listTenantsAndIsolation(t *testing.T, env SecurityEnv) {
	a, b := newTenant(), newTenant()
	createRequest(t, env.Base, CtxForTenant(a), nil)
	createRequest(t, env.Base, CtxForTenant(b), nil)
	tenants, err := env.Retention.ListTenants(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, id := range tenants {
		found[id] = true
	}
	if !found[a] || !found[b] {
		t.Fatalf("ListTenants missed a tenant: %v", tenants)
	}
}

// CheckTextColumnsDeclared fails for any text-like column of the schema that is neither erased on anonymization
// nor exempted with a reason, and for stale entries in either list. columns returns "table.column" names.
func CheckTextColumnsDeclared(t *testing.T, columns func() []string) {
	t.Helper()
	have := map[string]bool{}
	for _, c := range columns() {
		have[c] = true
	}
	erasable := map[string]bool{}
	for _, c := range domain.ErasableColumns {
		erasable[c.Table+"."+c.Column] = true
		if !have[c.Table+"."+c.Column] {
			t.Errorf("ErasableColumns lists %s.%s which is not a text-like column of the schema", c.Table, c.Column)
		}
	}
	for c := range have {
		if erasable[c] {
			continue
		}
		if !domain.IsExemptTextColumn(c) {
			t.Errorf("text column %s is neither in ErasableColumns nor in ExemptTextColumns: erasure would miss it", c)
		}
	}
}

// CheckExemptColumnsExist reports exempt entries that no longer exist. Run it on the reference dialect only: the
// lists differ per dialect because MySQL types some columns differently (uuid vs VARCHAR).
func CheckExemptColumnsExist(t *testing.T, columns func() []string, exempt map[string]string) {
	t.Helper()
	have := map[string]bool{}
	for _, c := range columns() {
		have[c] = true
	}
	for c := range exempt {
		if !have[c] {
			t.Errorf("ExemptTextColumns lists %s which no longer exists", c)
		}
	}
}

// withinMicro tolerates the rounding of TIMESTAMP(6) columns.
func withinMicro(a, b time.Time) bool {
	d := a.Sub(b)
	if d < 0 {
		d = -d
	}
	return d <= 2*time.Microsecond
}
