package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ledgerlite/ledgerlite/internal/ledger"
)

func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	svc, err := ledger.NewService(ledger.NewMemoryStore(), ledger.Config{DefaultDailyLimit: 10_000})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(New(svc, slog.New(slog.NewTextHandler(io.Discard, nil))))
	t.Cleanup(srv.Close)
	return srv
}

func call(t *testing.T, method, url, body string, headers map[string]string) (*http.Response, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp, out
}

func errCode(m map[string]any) string {
	e, _ := m["error"].(map[string]any)
	code, _ := e["code"].(string)
	return code
}

func createAccount(t *testing.T, base, body string) string {
	t.Helper()
	resp, out := call(t, "POST", base+"/accounts", body, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create account: %d %v", resp.StatusCode, out)
	}
	return out["id"].(string)
}

// API-001, API-002 (FR-001, FR-002).
func TestCreateAndGetAccount(t *testing.T) {
	srv := newServer(t)
	id := createAccount(t, srv.URL, `{"owner":"alice","initial_deposit":2500}`)
	resp, out := call(t, "GET", srv.URL+"/accounts/"+id, "", nil)
	if resp.StatusCode != 200 || out["balance"].(float64) != 2500 {
		t.Fatalf("get account: %d %v", resp.StatusCode, out)
	}
	resp, out = call(t, "GET", srv.URL+"/accounts/missing", "", nil)
	if resp.StatusCode != 404 || errCode(out) != "account_not_found" {
		t.Fatalf("missing account: %d %v", resp.StatusCode, out)
	}
	resp, out = call(t, "POST", srv.URL+"/accounts", `{"owner":""}`, nil)
	if resp.StatusCode != 400 || errCode(out) != "invalid_account" {
		t.Fatalf("empty owner: %d %v", resp.StatusCode, out)
	}
}

// API-003 (FR-003, FR-004, BR-004): 201 first, 200 with a replay header, 409 on payload mismatch.
func TestTransferIdempotencyOverHTTP(t *testing.T) {
	srv := newServer(t)
	a := createAccount(t, srv.URL, `{"owner":"a","initial_deposit":1000}`)
	b := createAccount(t, srv.URL, `{"owner":"b"}`)
	body := `{"from":"` + a + `","to":"` + b + `","amount":250}`
	h := map[string]string{"Idempotency-Key": "http-1"}
	resp, first := call(t, "POST", srv.URL+"/transfers", body, h)
	if resp.StatusCode != 201 {
		t.Fatalf("first: %d %v", resp.StatusCode, first)
	}
	resp, again := call(t, "POST", srv.URL+"/transfers", body, h)
	if resp.StatusCode != 200 || resp.Header.Get("Idempotent-Replayed") != "true" || again["id"] != first["id"] {
		t.Fatalf("replay: %d %v header=%q", resp.StatusCode, again, resp.Header.Get("Idempotent-Replayed"))
	}
	resp, out := call(t, "POST", srv.URL+"/transfers", strings.Replace(body, "250", "251", 1), h)
	if resp.StatusCode != 409 || errCode(out) != "idempotency_conflict" {
		t.Fatalf("conflict: %d %v", resp.StatusCode, out)
	}
	resp, out = call(t, "POST", srv.URL+"/transfers", body, nil)
	if resp.StatusCode != 400 || errCode(out) != "idempotency_key_required" {
		t.Fatalf("missing key: %d %v", resp.StatusCode, out)
	}
}

// NFR-004, NFR-006: fractional amounts are rejected and business errors map to stable codes.
func TestTransferErrorMapping(t *testing.T) {
	srv := newServer(t)
	a := createAccount(t, srv.URL, `{"owner":"a","initial_deposit":100,"daily_limit":150}`)
	b := createAccount(t, srv.URL, `{"owner":"b"}`)
	cases := []struct {
		key, body string
		status    int
		code      string
	}{
		{"e1", `{"from":"` + a + `","to":"` + b + `","amount":10.5}`, 400, "invalid_json"},
		{"e2", `{"from":"` + a + `","to":"` + b + `","amount":0}`, 400, "invalid_amount"},
		{"e3", `{"from":"` + a + `","to":"` + b + `","amount":101}`, 422, "insufficient_funds"},
		{"e4", `{"from":"` + a + `","to":"` + b + `","amount":1,"note":"x"}`, 400, "invalid_json"},
	}
	for _, c := range cases {
		resp, out := call(t, "POST", srv.URL+"/transfers", c.body, map[string]string{"Idempotency-Key": c.key})
		if resp.StatusCode != c.status || errCode(out) != c.code {
			t.Errorf("%s: got %d %q, want %d %q", c.key, resp.StatusCode, errCode(out), c.status, c.code)
		}
	}
}

// API-003, BR-003: the daily limit surfaces as 422 daily_limit_exceeded.
func TestDailyLimitOverHTTP(t *testing.T) {
	srv := newServer(t)
	a := createAccount(t, srv.URL, `{"owner":"a","initial_deposit":1000,"daily_limit":300}`)
	b := createAccount(t, srv.URL, `{"owner":"b"}`)
	send := func(key string, amount string) (*http.Response, map[string]any) {
		return call(t, "POST", srv.URL+"/transfers", `{"from":"`+a+`","to":"`+b+`","amount":`+amount+`}`, map[string]string{"Idempotency-Key": key})
	}
	if resp, out := send("l1", "300"); resp.StatusCode != 201 {
		t.Fatalf("exactly the limit: %d %v", resp.StatusCode, out)
	}
	if resp, out := send("l2", "1"); resp.StatusCode != 422 || errCode(out) != "daily_limit_exceeded" {
		t.Fatalf("over the limit: %d %v", resp.StatusCode, out)
	}
}

// API-004, API-005, API-006 (FR-005, FR-006).
func TestTransferLookupJournalAndHealth(t *testing.T) {
	srv := newServer(t)
	a := createAccount(t, srv.URL, `{"owner":"a","initial_deposit":500}`)
	b := createAccount(t, srv.URL, `{"owner":"b"}`)
	_, tr := call(t, "POST", srv.URL+"/transfers", `{"from":"`+a+`","to":"`+b+`","amount":200}`, map[string]string{"Idempotency-Key": "j1"})
	if resp, got := call(t, "GET", srv.URL+"/transfers/"+tr["id"].(string), "", nil); resp.StatusCode != 200 || got["amount"].(float64) != 200 {
		t.Fatalf("get transfer: %d %v", resp.StatusCode, got)
	}
	if resp, got := call(t, "GET", srv.URL+"/transfers/none", "", nil); resp.StatusCode != 404 || errCode(got) != "transfer_not_found" {
		t.Fatalf("missing transfer: %d %v", resp.StatusCode, got)
	}
	_, j := call(t, "GET", srv.URL+"/journal?account="+b, "", nil)
	if entries := j["entries"].([]any); len(entries) != 1 || entries[0].(map[string]any)["delta"].(float64) != 200 {
		t.Fatalf("journal of b: %v", j)
	}
	_, empty := call(t, "GET", srv.URL+"/journal?account=unknown", "", nil)
	if entries, ok := empty["entries"].([]any); !ok || len(entries) != 0 {
		t.Fatalf("unknown account journal must be an empty list, got %v", empty)
	}
	if resp, h := call(t, "GET", srv.URL+"/healthz", "", nil); resp.StatusCode != 200 || h["status"] != "ok" {
		t.Fatalf("healthz: %d %v", resp.StatusCode, h)
	}
}
