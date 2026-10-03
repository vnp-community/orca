package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver/mcpservertest"
)

func TestSCMLimiterRefillAndBounds(t *testing.T) {
	now := time.Unix(1000, 0)
	l := newSCMLimiter(30, func() time.Time { return now })
	for i := 0; i < 30; i++ {
		if ok, _ := l.allow("t1", "github"); !ok {
			t.Fatalf("call %d refused", i)
		}
	}
	ok, wait := l.allow("t1", "github")
	if ok || wait <= 0 || wait > 3*time.Second {
		t.Fatalf("31st call: ok=%v wait=%v", ok, wait)
	}
	if ok, _ := l.allow("t2", "github"); !ok {
		t.Error("another tenant must have its own budget")
	}
	if ok, _ := l.allow("t1", "gitlab"); !ok {
		t.Error("another provider must have its own budget")
	}
	now = now.Add(2 * time.Second) // 0.5 token/s -> one token back
	if ok, _ := l.allow("t1", "github"); !ok {
		t.Error("token should have refilled")
	}
	for i := 0; i < maxRateBuckets*2; i++ {
		l.allow(string(rune(i))+"x"+time.Duration(i).String(), "github")
	}
	if len(l.buckets) > maxRateBuckets {
		t.Errorf("buckets unbounded: %d", len(l.buckets))
	}
	if ok, _ := newSCMLimiter(-1, time.Now).allow("t", "github"); !ok {
		t.Error("negative rate disables the limiter")
	}
}

func TestSCMRateLimitStopsBeforeDownstream(t *testing.T) {
	var hits atomic.Int32
	ex, _ := newExec(t, &mcpservertest.FakeGate{}, allPacks, nil, &hits)
	bob := mcpserver.Principal{TenantID: "t2", UserID: "bob", Scopes: allScopes}
	call := func(p mcpserver.Principal, n int) error {
		_, err := ex.CallTool(context.Background(), p, "github_issues", json.RawMessage(fmt.Sprintf(`{"repo":"r%d"}`, n)))
		return err
	}
	// distinct args: the 30s result cache must not hide calls from the limiter
	for i := 0; i < 30; i++ {
		if err := call(alice, i); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	before := hits.Load()
	err := call(alice, 40)
	if err == nil || !strings.HasPrefix(errText(err), "RATE_LIMITED") {
		t.Fatalf("want RATE_LIMITED, got %v", err)
	}
	if hits.Load() != before {
		t.Error("a rate-limited call reached the downstream")
	}
	if err := call(bob, 1); err != nil {
		t.Errorf("other tenant: %v", err)
	}
	// non-provider tools are never limited
	for i := 0; i < 40; i++ {
		if _, err := ex.CallTool(context.Background(), alice, "project_list", json.RawMessage(`{}`)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSCMRateLimitConfigFromEnv(t *testing.T) {
	env := map[string]string{"MCP_SCM_RATE_PER_MIN": "5", "MCP_PII_MASK": "ALL", "MCP_SENSITIVE_PATH_EXTRA": "a/*, b"}
	c, err := DefaultConfig().ApplyEnv(func(k string) string { return env[k] })
	if err != nil || c.SCMRatePerMin != 5 || c.PIIMask != "all" || len(c.SensitivePathExtra) != 2 {
		t.Fatalf("%+v %v", c, err)
	}
	for _, bad := range []map[string]string{{"MCP_SCM_RATE_PER_MIN": "x"}, {"MCP_PII_MASK": "maybe"}} {
		if _, err := DefaultConfig().ApplyEnv(func(k string) string { return bad[k] }); err == nil {
			t.Errorf("%v accepted", bad)
		}
	}
	d := DefaultConfig().withDefaults()
	if d.SCMRatePerMin != 30 || d.PIIMask != "directory" {
		t.Errorf("defaults: %+v", d)
	}
	off, _ := DefaultConfig().ApplyEnv(func(k string) string { return map[string]string{"MCP_SCM_RATE_PER_MIN": "0"}[k] })
	if off.withDefaults().SCMRatePerMin >= 0 {
		t.Error("0 must disable")
	}
}
