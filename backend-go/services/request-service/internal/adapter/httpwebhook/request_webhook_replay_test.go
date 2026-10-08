package httpwebhook

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type memGuard struct {
	mu   sync.Mutex
	seen map[string]bool
	err  error
}

func (g *memGuard) FirstSeen(_ context.Context, tenantID, source, sig string) (bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.err != nil {
		return false, g.err
	}
	k := tenantID + "|" + source + "|" + sig
	if g.seen[k] {
		return false, nil
	}
	g.seen[k] = true
	return true, nil
}

var fixedNow = time.Unix(1_800_000_000, 0)

func replaySetup(t *testing.T, requireTimestamp bool) (*http.ServeMux, *fakeCreator, *memGuard) {
	t.Helper()
	srcs, err := ParseStaticSources(`[{"tenant_id":"`+tenantA+`","source":"sentry","reporter_id":"`+reporter+`","secret_env":"S"}]`, func(string) string { return "topsecret" }, nil)
	if err != nil {
		t.Fatal(err)
	}
	cr := &fakeCreator{seen: map[string]string{}}
	g := &memGuard{seen: map[string]bool{}}
	mux := http.NewServeMux()
	NewHandler(srcs, cr).WithReplayProtection(g, requireTimestamp).WithClock(func() time.Time { return fixedNow }).Mount(mux)
	return mux, cr, g
}

func postTS(mux *http.ServeMux, ts, sig, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/request-webhooks/sentry", strings.NewReader(body))
	req.Header.Set(HeaderTenant, tenantA)
	req.Header.Set(HeaderSignature, sig)
	if ts != "" {
		req.Header.Set(HeaderTimestamp, ts)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func stamp(d time.Duration) string { return strconv.FormatInt(fixedNow.Add(d).Unix(), 10) }

func TestRequestWebhook_TimestampWindow(t *testing.T) {
	mux, cr, _ := replaySetup(t, true)
	body := goodBody("evt-ts")
	for name, tc := range map[string]struct {
		off  time.Duration
		want int
	}{"4 minutes old": {-4 * time.Minute, 200}, "6 minutes old": {-6 * time.Minute, 401}, "6 minutes ahead": {6 * time.Minute, 401}} {
		ts := stamp(tc.off)
		rec := postTS(mux, ts, sign("topsecret", ts+"."+body), body)
		if rec.Code != tc.want {
			t.Errorf("%s: status %d want %d (%s)", name, rec.Code, tc.want, rec.Body)
		}
	}
	if len(cr.inputs) != 1 {
		t.Errorf("only the in-window call creates, got %d", len(cr.inputs))
	}
}

func TestRequestWebhook_TimestampMissingOrTamperedIs401(t *testing.T) {
	mux, cr, _ := replaySetup(t, true)
	body := goodBody("evt-t2")
	ts := stamp(0)
	good := sign("topsecret", ts+"."+body)
	for name, rec := range map[string]*httptest.ResponseRecorder{
		"missing timestamp":   postTS(mux, "", sign("topsecret", body), body),
		"unparseable":         postTS(mux, "yesterday", good, body),
		"timestamp edited":    postTS(mux, stamp(time.Second), good, body),
		"body-only signature": postTS(mux, ts, sign("topsecret", body), body),
		"old signature no ts": postTS(mux, "", good, body),
	} {
		if rec.Code != 401 || rec.Body.String() != invalidSignatureBody+"\n" && rec.Body.String() != invalidSignatureBody {
			t.Errorf("%s: %d %q", name, rec.Code, rec.Body)
		}
	}
	if len(cr.inputs) != 0 {
		t.Fatal("nothing may be created")
	}
}

func TestRequestWebhook_NonceReplayIsDuplicate200AndCreatesOnce(t *testing.T) {
	mux, cr, _ := replaySetup(t, true)
	body := goodBody("evt-n")
	ts := stamp(-time.Minute)
	sig := sign("topsecret", ts+"."+body)
	first := postTS(mux, ts, sig, body)
	second := postTS(mux, ts, sig, body)
	if first.Code != 200 || !strings.Contains(first.Body.String(), `"created":true`) {
		t.Fatalf("first %d %s", first.Code, first.Body)
	}
	if second.Code != 200 || !strings.Contains(second.Body.String(), `"duplicate":true`) || !strings.Contains(second.Body.String(), `"created":false`) {
		t.Fatalf("replay %d %s", second.Code, second.Body)
	}
	if len(cr.inputs) != 1 {
		t.Fatalf("CreateRequest must run once, ran %d", len(cr.inputs))
	}
}

func TestRequestWebhook_NonceStoreErrorIs503NotBypass(t *testing.T) {
	mux, cr, g := replaySetup(t, true)
	g.err = errors.New("db down")
	body := goodBody("evt-e")
	ts := stamp(0)
	rec := postTS(mux, ts, sign("topsecret", ts+"."+body), body)
	if rec.Code != 503 || len(cr.inputs) != 0 {
		t.Fatalf("%d creates=%d", rec.Code, len(cr.inputs))
	}
}

func TestRequestWebhook_UnauthenticatedCallsNeverReachTheNonceStore(t *testing.T) {
	mux, _, g := replaySetup(t, true)
	body := goodBody("evt-x")
	ts := stamp(0)
	postTS(mux, ts, sign("wrong", ts+"."+body), body)
	if len(g.seen) != 0 {
		t.Fatalf("store polluted by an unauthenticated call: %v", g.seen)
	}
}

func TestRequestWebhook_TransitionModeAcceptsBodyOnlySignature(t *testing.T) {
	mux, cr, _ := replaySetup(t, false)
	body := goodBody("evt-legacy")
	rec := postTS(mux, "", sign("topsecret", body), body)
	if rec.Code != 200 || len(cr.inputs) != 1 {
		t.Fatalf("legacy sender in the transition window: %d %s", rec.Code, rec.Body)
	}
	// A sender that does send a timestamp is held to it even in transition mode.
	body2 := goodBody("evt-new")
	old := stamp(-10 * time.Minute)
	if rec := postTS(mux, old, sign("topsecret", old+"."+body2), body2); rec.Code != 401 {
		t.Fatalf("stale timestamp in transition mode: %d", rec.Code)
	}
}
