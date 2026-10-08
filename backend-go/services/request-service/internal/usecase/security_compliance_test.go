package usecase

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/secretscan"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

const (
	tA = "11111111-1111-4111-8111-111111111111"
	tB = "22222222-2222-4222-8222-222222222222"
)

func newRecorder(out *memAuditOutbox, tx TxRunner) (*AuditRecorder, *memSink) {
	sink := &memSink{}
	return NewAuditRecorder(sink, out, tx, nil), sink
}

func TestAuditRecorder_DurableGoesToOutboxOthersToSink(t *testing.T) {
	out := &memAuditOutbox{}
	rec, sink := newRecorder(out, rollbackTx{out})
	ctx := adminCtx(tA)

	if err := rec.Record(ctx, AuditEvent{Action: domain.AuditRequestErase, TargetType: "request", TargetID: "r1", RequestID: "r1"}); err != nil {
		t.Fatal(err)
	}
	if err := rec.Record(ctx, AuditEvent{Action: domain.AuditRequestAccessDenied, TargetType: "request", TargetID: "r1", Outcome: "denied", RequestID: "r1", Metadata: map[string]any{"rpc": "GetRequest"}}); err != nil {
		t.Fatal(err)
	}
	if len(out.rows) != 1 || out.rows[0].Action != domain.AuditRequestErase || out.rows[0].AuditID == "" {
		t.Fatalf("outbox: %+v", out.rows)
	}
	if len(sink.entries) != 1 || sink.entries[0].Action != domain.AuditRequestAccessDenied || sink.entries[0].Outcome != "denied" {
		t.Fatalf("sink: %+v", sink.entries)
	}
	if !strings.Contains(out.rows[0].MetadataJSON, out.rows[0].AuditID) {
		t.Error("audit_id must be in the metadata so a duplicate delivery can be collapsed")
	}
}

func TestAuditRecorder_DurableEntryRollsBackWithTheChange(t *testing.T) {
	out := &memAuditOutbox{}
	rec, _ := newRecorder(out, rollbackTx{out})
	boom := errors.New("write failed")
	err := rollbackTx{out}.InTx(adminCtx(tA), func(ctx context.Context) error {
		if err := rec.RecordDurable(ctx, AuditEvent{Action: domain.AuditRequestErase, TargetType: "request", TargetID: "r1"}); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) || len(out.rows) != 0 {
		t.Fatalf("a rolled back change must not leave an audit row: err=%v rows=%d", err, len(out.rows))
	}
}

func TestAuditRecorder_DurableFailureIsReturnedBestEffortIsNot(t *testing.T) {
	out := &memAuditOutbox{failEnq: errors.New("db down")}
	rec, _ := newRecorder(out, memTx{})
	if err := rec.Record(adminCtx(tA), AuditEvent{Action: domain.AuditRequestExport, TargetType: "request"}); err == nil {
		t.Error("an export that cannot be audited must fail")
	}
	// A malformed best-effort event never fails the action it describes.
	if err := rec.Record(adminCtx(tA), AuditEvent{Action: domain.AuditRequestReadDenied, Metadata: map[string]any{"title": "x"}}); err != nil {
		t.Errorf("best effort: %v", err)
	}
}

// Property: whatever the use cases audit, the metadata never contains the Request's title or body.
func TestAuditMetadataNeverContainsContent(t *testing.T) {
	out := &memAuditOutbox{}
	rec, sink := newRecorder(out, rollbackTx{out})
	const secretTitle, secretBody = "SECRET-TITLE-123", "SECRET-BODY-456"
	ctx := adminCtx(tA)
	store := newFakeRetention()
	store.tenants = []string{tA}
	store.expired[tA] = []string{"r1"}
	store.reporters["r1"] = "rep"

	clock := &fakeClock{now: time.Now()}
	_, _ = NewRunRequestRetention(store, rec, []byte("k"), clock, nil).Execute(ctx)
	reqs := fakeReader{r: domain.Request{ID: "r2", Title: secretTitle, Body: secretBody, Status: domain.RequestStatusCompleted}}
	_, _ = NewEraseRequest(reqs, store, rollbackTx{out}, rec, nil, []byte("k"), clock).Execute(ctx, "r2", "cleanup")
	rec.RecordDenied(ctx, DeniedAccess{RPC: "GetRequest", RequestID: "r2", ActorType: "user"})
	// Content in metadata is refused outright.
	_ = rec.Record(ctx, AuditEvent{Action: domain.AuditRequestExport, Metadata: map[string]any{"title": secretTitle}})

	var all []string
	for _, r := range out.rows {
		all = append(all, r.MetadataJSON, r.Action, r.TargetID, r.ActorID, r.TargetType)
	}
	for _, e := range sink.entries {
		all = append(all, e.MetadataJSON, e.Action, e.Target, e.TargetID)
	}
	if len(all) == 0 {
		t.Fatal("expected audit records")
	}
	for _, s := range all {
		if strings.Contains(s, secretTitle) || strings.Contains(s, secretBody) {
			t.Fatalf("content leaked into audit: %s", s)
		}
	}
}

type fakeReader struct{ r domain.Request }

func (f fakeReader) Get(context.Context, string) (domain.Request, error) { return f.r, nil }

func TestRateLimiter_BurstThenRefillAndRetryAfter(t *testing.T) {
	now := time.Unix(1000, 0)
	l := NewRateLimiter(map[string]domain.Limit{domain.RateAI: {PerSecond: 2, Burst: 4}}, func() time.Time { return now })
	for i := 0; i < 4; i++ {
		if ok, _ := l.Allow(tA, domain.RateAI, ""); !ok {
			t.Fatalf("burst call %d refused", i)
		}
	}
	ok, wait := l.Allow(tA, domain.RateAI, "")
	if ok || wait <= 0 || wait > time.Second {
		t.Fatalf("fifth call: ok=%v wait=%v", ok, wait)
	}
	now = now.Add(500 * time.Millisecond) // refills one token at 2/s
	if ok, _ := l.Allow(tA, domain.RateAI, ""); !ok {
		t.Fatal("a token should have refilled")
	}
	if ok, _ := l.Allow(tA, domain.RateAI, ""); ok {
		t.Fatal("only one token refilled")
	}
}

func TestRateLimiter_TenantsClassesAndSubjectsAreIsolated(t *testing.T) {
	now := time.Unix(1, 0)
	l := NewRateLimiter(map[string]domain.Limit{domain.RateAI: {PerSecond: 1, Burst: 1}, domain.RateRead: {PerSecond: 1, Burst: 1}}, func() time.Time { return now })
	l.Allow(tA, domain.RateAI, "")
	if ok, _ := l.Allow(tA, domain.RateAI, ""); ok {
		t.Fatal("tenant A exhausted")
	}
	if ok, _ := l.Allow(tB, domain.RateAI, ""); !ok {
		t.Fatal("tenant B must not be affected by tenant A")
	}
	if ok, _ := l.Allow(tA, domain.RateRead, ""); !ok {
		t.Fatal("class read is separate from ai")
	}
	if ok, _ := l.Allow(tA, domain.RateAI, "mcp-client-1"); !ok {
		t.Fatal("a subject has its own bucket")
	}
	if ok, _ := l.Allow(tA, domain.RateNone, ""); !ok {
		t.Fatal("unlimited class")
	}
}

func TestRateLimiter_EvictsIdleBucketsAndCapsMemory(t *testing.T) {
	now := time.Unix(1, 0)
	l := NewRateLimiter(map[string]domain.Limit{domain.RateRead: {PerSecond: 1, Burst: 1}}, func() time.Time { return now })
	for i := 0; i < 50; i++ {
		l.Allow(tA, domain.RateRead, string(rune('a'+i%26))+string(rune('a'+i/26)))
	}
	if l.Size() != 50 {
		t.Fatalf("size %d", l.Size())
	}
	now = now.Add(rateBucketIdle + 2*rateSweepPeriod)
	l.Allow(tA, domain.RateRead, "fresh")
	if l.Size() != 1 {
		t.Fatalf("idle buckets should be swept, size %d", l.Size())
	}
}

type countingCounter struct{ open, running, project int }

func (c countingCounter) CountOpenRequests(context.Context) (int, error) { return c.open, nil }
func (c countingCounter) CountRunning(_ context.Context, p string) (int, error) {
	if p == "" {
		return c.running, nil
	}
	return c.project, nil
}

func TestConcurrencyGate(t *testing.T) {
	caps := domain.DefaultConcurrencyCaps
	if err := NewConcurrencyGate(caps, countingCounter{open: 1999}).CheckCreate(context.Background()); err != nil {
		t.Errorf("below the cap: %v", err)
	}
	if rl, ok := domain.IsRateLimited(NewConcurrencyGate(caps, countingCounter{open: 2000}).CheckCreate(context.Background())); !ok || rl.Detail != "concurrency" {
		t.Errorf("at the cap: %v", rl)
	}
	if _, ok := domain.IsRateLimited(NewConcurrencyGate(caps, countingCounter{running: 10}).CheckRun(context.Background(), "p")); !ok {
		t.Error("tenant running cap")
	}
	if _, ok := domain.IsRateLimited(NewConcurrencyGate(caps, countingCounter{running: 3, project: 2}).CheckRun(context.Background(), "p")); !ok {
		t.Error("project running cap")
	}
	if err := NewConcurrencyGate(caps, countingCounter{running: 3, project: 2}).CheckRun(context.Background(), ""); err != nil {
		t.Errorf("without a project only the tenant cap applies: %v", err)
	}
}

func TestSecretIngressGuard(t *testing.T) {
	g := SecretIngressGuard{}
	pem := "-----BEGIN RSA PRIVATE KEY-----\nMIIBOgIBAAJBAKj34GkxFhD90vcNLYLInFEX6Ppy1tPf9Cnzj4p4WGeKLs1Pt8Qu\n-----END RSA PRIVATE KEY-----"
	title, body, suspected, kinds, err := g.Apply("deploy fails", "see key:\n"+pem+"\nand ghp_abcdefghijklmnopqrstuvwxyz0123456789")
	if err != nil || !suspected {
		t.Fatalf("suspected=%v err=%v", suspected, err)
	}
	if strings.Contains(body, "BEGIN RSA PRIVATE KEY") || strings.Contains(body, "ghp_abcdef") || !strings.Contains(body, "[REDACTED:private_key]") || title != "deploy fails" {
		t.Errorf("body not masked: %q", body)
	}
	for _, k := range kinds {
		if strings.Contains(string(k), "ghp_") {
			t.Error("kinds must not hold values")
		}
	}
	// Medium confidence is left alone at the door (the prompt step masks it).
	_, b2, s2, _, _ := g.Apply("t", "header Authorization: Bearer abc12345def")
	if s2 || strings.Contains(b2, "REDACTED") {
		t.Errorf("medium confidence must pass untouched: %q", b2)
	}
	// Vietnamese text with diacritics is untouched.
	vi := "Lỗi đăng nhập: người dùng không thể đặt lại mật khẩu"
	_, b3, s3, _, _ := g.Apply(vi, vi)
	if s3 || b3 != vi {
		t.Errorf("Vietnamese changed: %q", b3)
	}
	// Text past the scan window cannot be checked and is refused.
	if _, _, _, _, err := g.Apply("t", strings.Repeat("a", 1<<20+10)); err == nil {
		t.Error("oversize body must be refused")
	}
}

type memNonces struct {
	mu   sync.Mutex
	seen map[string]time.Time
}

func (m *memNonces) Remember(_ context.Context, tn, src, hash string, exp time.Time) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := tn + "|" + src + "|" + hash
	if until, ok := m.seen[k]; ok && time.Now().Before(until) {
		return false, nil
	}
	m.seen[k] = exp
	return true, nil
}
func (m *memNonces) PruneExpired(context.Context, time.Time, int) (int, error) { return 0, nil }

func TestWebhookReplayGuard(t *testing.T) {
	store := &memNonces{seen: map[string]time.Time{}}
	g := NewWebhookReplayGuard(store, func() time.Time { return time.Now() })
	ctx := context.Background()
	if ok, _ := g.FirstSeen(ctx, tA, "jira", "sha256=abc"); !ok {
		t.Fatal("first sighting accepted")
	}
	if ok, _ := g.FirstSeen(ctx, tA, "jira", "sha256=abc"); ok {
		t.Fatal("replay must be refused")
	}
	if ok, _ := g.FirstSeen(ctx, tA, "github", "sha256=abc"); !ok {
		t.Fatal("another source is a different nonce space")
	}
	if ok, _ := g.FirstSeen(ctx, tB, "jira", "sha256=abc"); !ok {
		t.Fatal("another tenant is a different nonce space")
	}
	if WebhookNonceTTL <= 5*time.Minute {
		t.Fatal("nonce TTL must exceed the accepted clock skew")
	}
}

func TestPromptRedactor(t *testing.T) {
	pii := maskEmails{}
	text := "contact bob@example.com token ghp_abcdefghijklmnopqrstuvwxyz0123456789 Bearer abc12345def"
	off := NewPromptRedactor(fixedSettings{redact: false}, pii).Apply(context.Background(), text)
	if strings.Contains(off, "ghp_abcdef") || strings.Contains(off, "abc12345def") || !strings.Contains(off, "bob@example.com") {
		t.Errorf("secrets always masked, PII only on request: %q", off)
	}
	on := NewPromptRedactor(fixedSettings{redact: true}, pii).Apply(context.Background(), text)
	if strings.Contains(on, "bob@example.com") {
		t.Errorf("PII must be masked when the tenant asks: %q", on)
	}
	failed := NewPromptRedactor(fixedSettings{err: errors.New("db")}, pii).Apply(context.Background(), text)
	if strings.Contains(failed, "bob@example.com") {
		t.Errorf("a settings error must mask PII (fail closed): %q", failed)
	}
	if got := NewPromptRedactor(nil, nil).Apply(context.Background(), "plain"); got != "plain" {
		t.Errorf("got %q", got)
	}
	_ = secretscan.PatternsVersion
}

type fixedSettings struct {
	redact bool
	err    error
}

func (f fixedSettings) RedactPII(context.Context) (bool, error) { return f.redact, f.err }

type maskEmails struct{}

func (maskEmails) Mask(s string) string { return strings.ReplaceAll(s, "bob@example.com", "[email]") }

func TestRunRequestRetention(t *testing.T) {
	store := newFakeRetention()
	store.tenants = []string{tA, tB}
	store.settings[tB] = domain.RetentionSettings{RequestDays: 0} // keeps forever
	store.expired[tA] = []string{"old1", "old2"}
	store.expired[tB] = []string{"oldB"}
	for _, id := range []string{"old1", "old2", "oldB"} {
		store.reporters[id] = "reporter-" + id
	}
	out := &memAuditOutbox{}
	rec, _ := newRecorder(out, rollbackTx{out})
	clock := &fakeClock{now: time.Date(2026, 10, 8, 3, 0, 0, 0, time.UTC)}
	uc := NewRunRequestRetention(store, rec, []byte("key"), clock, nil)

	sum, err := uc.Execute(context.Background())
	if err != nil || sum.Anonymized != 2 || sum.Tenants != 2 {
		t.Fatalf("summary %+v err %v", sum, err)
	}
	if !store.erased["old1"] || !store.erased["old2"] || store.erased["oldB"] {
		t.Errorf("only tenant A's expired Requests are anonymized: %v", store.erased)
	}
	if store.reporters["old1"] == "reporter-old1" || store.reporters["old1"] != domain.PseudonymizeReporter([]byte("key"), tA, "reporter-old1") {
		t.Errorf("reporter must be replaced by its pseudonym: %q", store.reporters["old1"])
	}
	if want := clock.now.AddDate(0, 0, -730); !store.lastCut[tA].Equal(want) {
		t.Errorf("cutoff %v want %v", store.lastCut[tA], want)
	}
	if len(out.rows) != 1 || out.rows[0].Action != domain.AuditRetentionRun || !strings.Contains(out.rows[0].MetadataJSON, `"anonymized":2`) || out.rows[0].ActorType != "system" {
		t.Errorf("retention run audit: %+v", out.rows)
	}
	// Idempotent: a second run changes nothing and audits nothing.
	out.rows = nil
	if sum, _ := uc.Execute(context.Background()); sum.Anonymized != 0 || len(out.rows) != 0 {
		t.Errorf("second run: %+v rows %d", sum, len(out.rows))
	}
}

func TestRunRequestRetention_FailsClosedWithoutKey(t *testing.T) {
	store := newFakeRetention()
	store.tenants = []string{tA}
	store.expired[tA] = []string{"x"}
	_, err := NewRunRequestRetention(store, nil, nil, nil, nil).Execute(context.Background())
	if err == nil || !strings.Contains(err.Error(), "REQUEST_ERASE_KEY_MISSING") || store.erased["x"] {
		t.Fatalf("err=%v erased=%v", err, store.erased)
	}
}

func TestRunRequestRetention_TwoReplicasDoNotDoubleProcess(t *testing.T) {
	store := newFakeRetention()
	store.tenants = []string{tA}
	for i := 0; i < 150; i++ {
		id := string(rune('A'+i/26)) + string(rune('a'+i%26))
		store.expired[tA] = append(store.expired[tA], id)
		store.reporters[id] = "r"
	}
	out := &memAuditOutbox{}
	rec, _ := newRecorder(out, memTx{})
	var wg sync.WaitGroup
	var mu sync.Mutex
	total := 0
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, _ := NewRunRequestRetention(store, rec, []byte("k"), &fakeClock{now: time.Now()}, nil).Execute(context.Background())
			mu.Lock()
			total += s.Anonymized
			mu.Unlock()
		}()
	}
	wg.Wait()
	if total != 150 {
		t.Fatalf("every Request is anonymized exactly once across replicas, got %d", total)
	}
}

type failingTasks struct{ err error }

func (f failingTasks) Erase(context.Context, string) error { return f.err }

func TestEraseRequest(t *testing.T) {
	newUC := func(status domain.RequestStatus, tasks TaskContentEraser, key string) (*EraseRequest, *fakeRetentionStore, *memAuditOutbox) {
		store := newFakeRetention()
		store.reporters["r1"] = "rep"
		out := &memAuditOutbox{}
		rec, _ := newRecorder(out, rollbackTx{out})
		return NewEraseRequest(fakeReader{r: domain.Request{ID: "r1", Status: status}}, store, rollbackTx{out}, rec, tasks, []byte(key), &fakeClock{now: time.Unix(5, 0)}), store, out
	}
	ctx := adminCtx(tA)

	uc, store, out := newUC(domain.RequestStatusCompleted, nil, "k")
	res, err := uc.Execute(ctx, "r1", "customer asked")
	if err != nil || !store.erased["r1"] || len(out.rows) != 1 || out.rows[0].Action != domain.AuditRequestErase {
		t.Fatalf("erase: res=%+v err=%v erased=%v rows=%d", res, err, store.erased, len(out.rows))
	}
	if len(res.External) != 1 || res.External[0].Status != "unsupported" || res.NotCoveredNote == "" {
		t.Errorf("unsupported task-service must be reported, not fail: %+v", res)
	}
	// Idempotent.
	if res, err := uc.Execute(ctx, "r1", "again"); err != nil || !res.AlreadyErased || len(out.rows) != 1 {
		t.Errorf("second erase: %+v %v rows %d", res, err, len(out.rows))
	}

	if uc, store, _ := newUC(domain.RequestStatusExecuting, nil, "k"); true {
		if _, err := uc.Execute(ctx, "r1", "x"); err == nil || !strings.Contains(err.Error(), "REQUEST_ERASE_NOT_ALLOWED") || store.erased["r1"] {
			t.Errorf("executing must be refused: %v", err)
		}
	}
	uc, _, _ = newUC(domain.RequestStatusCompleted, nil, "k")
	for _, reason := range []string{"", "   ", strings.Repeat("x", 501)} {
		if _, err := uc.Execute(ctx, "r1", reason); err == nil || !strings.Contains(err.Error(), "REQUEST_ERASE_REASON_REQUIRED") {
			t.Errorf("reason %d chars: %v", len(reason), err)
		}
	}
	uc, store, _ = newUC(domain.RequestStatusCompleted, nil, "")
	if _, err := uc.Execute(ctx, "r1", "x"); err == nil || !strings.Contains(err.Error(), "REQUEST_ERASE_KEY_MISSING") || store.erased["r1"] {
		t.Errorf("missing key must fail closed: %v", err)
	}
	uc, _, _ = newUC(domain.RequestStatusCompleted, nil, "k")
	nonAdmin := tenant.WithUserID(tenant.WithTenantID(context.Background(), tA), "u")
	agentAdmin := tenant.WithActorType(adminCtx(tA), "agent")
	for name, c := range map[string]context.Context{"non admin": nonAdmin, "agent": agentAdmin} {
		if _, err := uc.Execute(c, "r1", "x"); err == nil || !strings.Contains(err.Error(), "REQUEST_FORBIDDEN") {
			t.Errorf("%s: %v", name, err)
		}
	}
	uc, _, _ = newUC(domain.RequestStatusCompleted, failingTasks{err: errors.New("down")}, "k")
	if res, err := uc.Execute(ctx, "r1", "x"); err != nil || res.External[0].Status != "failed" {
		t.Errorf("a failing external eraser is reported, not fatal: %+v %v", res, err)
	}
}

func TestEraseRequest_AuditFailureRollsBackErase(t *testing.T) {
	store := newFakeRetention()
	out := &memAuditOutbox{failEnq: errors.New("outbox down")}
	rec, _ := newRecorder(out, rollbackTx{out})
	uc := NewEraseRequest(fakeReader{r: domain.Request{ID: "r1", Status: domain.RequestStatusCompleted}}, store, rollbackTx{out}, rec, nil, []byte("k"), nil)
	if _, err := uc.Execute(adminCtx(tA), "r1", "x"); err == nil {
		t.Fatal("erase without an audit entry must fail")
	}
}
