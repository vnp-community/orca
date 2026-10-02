package mcpmetrics_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	dto "github.com/prometheus/client_model/go"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpmetrics"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver/mcpservertest"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/originpolicy"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"
)

// forbiddenLabels must never appear on any orca_mcp_* series (cardinality and
// privacy: tenant/user/session ids are unbounded).
var forbiddenLabels = []string{"tenant", "tenant_id", "user", "user_id", "session", "session_id", "client_id", "trace_id", "token"}

func gather(t *testing.T, m *mcpmetrics.Metrics) []*dto.MetricFamily {
	t.Helper()
	fams, err := m.Registry().Gather()
	if err != nil {
		t.Fatal(err)
	}
	return fams
}

func counter(t *testing.T, m *mcpmetrics.Metrics, name string, labels map[string]string) float64 {
	t.Helper()
	for _, f := range gather(t, m) {
		if f.GetName() != name {
			continue
		}
		for _, mm := range f.GetMetric() {
			if labelsMatch(mm, labels) {
				switch {
				case mm.Counter != nil:
					return mm.Counter.GetValue()
				case mm.Gauge != nil:
					return mm.Gauge.GetValue()
				case mm.Histogram != nil:
					return float64(mm.Histogram.GetSampleCount())
				}
			}
		}
	}
	return 0
}

func labelsMatch(m *dto.Metric, want map[string]string) bool {
	got := map[string]string{}
	for _, l := range m.GetLabel() {
		got[l.GetName()] = l.GetValue()
	}
	for k, v := range want {
		if got[k] != v {
			return false
		}
	}
	return true
}

func TestRecorderMapsEventsAndBoundsLabels(t *testing.T) {
	m := mcpmetrics.New()
	m.AuthFailure("no_credentials")
	m.AuthFailure("attacker-controlled-" + strings.Repeat("x", 500))
	m.Request("tools/call", "ok")
	m.Request("made-up/method", "ok")
	m.Request("tools/call", "weird")
	m.SessionOpened()
	m.SessionOpened()
	m.SessionClosed("idle")
	m.StreamOpened()
	m.Resume("gap")
	m.ProgressCoalesced(3)
	m.ProgressCoalesced(0)
	m.IdentityMismatch()
	m.ObservePrincipalResolve("cache_hit")

	checks := []struct {
		name   string
		labels map[string]string
		want   float64
	}{
		{"orca_mcp_auth_failures_total", map[string]string{"reason": "no_credentials"}, 1},
		{"orca_mcp_auth_failures_total", map[string]string{"reason": "other"}, 1},
		{"orca_mcp_requests_total", map[string]string{"method": "tools/call", "result": "ok"}, 1},
		{"orca_mcp_requests_total", map[string]string{"method": "other", "result": "ok"}, 1},
		{"orca_mcp_requests_total", map[string]string{"method": "tools/call", "result": "rpc_error"}, 1},
		{"orca_mcp_sessions_active", nil, 1},
		{"orca_mcp_sessions_closed_total", map[string]string{"reason": "idle"}, 1},
		{"orca_mcp_sse_streams_active", nil, 1},
		{"orca_mcp_resume_total", map[string]string{"result": "gap"}, 1},
		{"orca_mcp_sse_events_dropped_total", map[string]string{"reason": "coalesced"}, 3},
		{"orca_mcp_session_identity_mismatch_total", nil, 1},
		{"orca_mcp_principal_resolve_total", map[string]string{"result": "cache_hit"}, 1},
	}
	for _, c := range checks {
		if got := counter(t, m, c.name, c.labels); got != c.want {
			t.Errorf("%s%v = %v, want %v", c.name, c.labels, got, c.want)
		}
	}
}

// TestCardinalityStaysBounded drives 50 tools x 20 users x 5 tenants plus
// hostile input and asserts the series count and the label names.
func TestCardinalityStaysBounded(t *testing.T) {
	m := mcpmetrics.New()
	catalog := map[string]bool{}
	for i := 0; i < 50; i++ {
		catalog[fmt.Sprintf("tool_%d", i)] = true
	}
	gate := mcpmetrics.InstrumentGate(&mcpservertest.FakeGate{}, m)
	exec := mcpmetrics.InstrumentExecutor(execFunc(func(ctx context.Context, p mcpserver.Principal, name string, _ json.RawMessage) (*mcp.CallToolResult, error) {
		_, _ = gate.Decide(ctx, p, mcpserver.ToolMeta{Name: name, Namespace: "ns" + name[len(name)-1:], Risk: mcpserver.RiskRead}, nil)
		return &mcp.CallToolResult{}, nil
	}), m, func(n string) bool { return catalog[n] })
	for tenant := 0; tenant < 5; tenant++ {
		for user := 0; user < 20; user++ {
			p := mcpserver.Principal{TenantID: fmt.Sprintf("t%d", tenant), UserID: fmt.Sprintf("u%d", user)}
			for i := 0; i < 50; i++ {
				_, _ = exec.CallTool(context.Background(), p, fmt.Sprintf("tool_%d", i), nil)
			}
			_, _ = exec.CallTool(context.Background(), p, fmt.Sprintf("hostile-%d-%d", tenant, user), nil)
		}
	}
	series := 0
	for _, f := range gather(t, m) {
		if !strings.HasPrefix(f.GetName(), "orca_mcp_") {
			continue
		}
		for _, mm := range f.GetMetric() {
			series++
			for _, l := range mm.GetLabel() {
				for _, bad := range forbiddenLabels {
					if l.GetName() == bad {
						t.Errorf("%s carries forbidden label %q", f.GetName(), bad)
					}
				}
				if strings.HasPrefix(l.GetValue(), "hostile-") {
					t.Errorf("unbounded client input leaked into label %s=%q", l.GetName(), l.GetValue())
				}
			}
		}
	}
	if series >= 2000 {
		t.Fatalf("series count %d >= 2000", series)
	}
	if got := counter(t, m, "orca_mcp_tool_calls_total", map[string]string{"tool": "unknown"}); got != 100 {
		t.Errorf("unknown-tool calls = %v, want 100", got)
	}
}

type execFunc func(ctx context.Context, p mcpserver.Principal, name string, args json.RawMessage) (*mcp.CallToolResult, error)

func (f execFunc) CallTool(ctx context.Context, p mcpserver.Principal, name string, args json.RawMessage) (*mcp.CallToolResult, error) {
	return f(ctx, p, name, args)
}

func TestExecutorClassifiesResultsAndDecisions(t *testing.T) {
	m := mcpmetrics.New()
	fg := &mcpservertest.FakeGate{Decisions: map[string]mcpserver.GateDecision{
		"deny_me":   {Outcome: mcpserver.OutcomeDeny, Reasons: []string{"hard_deny"}},
		"need_appr": {Outcome: mcpserver.OutcomeRequireApproval, ApprovalID: "a1"},
	}, Approved: true}
	gate := mcpmetrics.InstrumentGate(fg, m)
	ex := mcpmetrics.InstrumentExecutor(execFunc(func(ctx context.Context, p mcpserver.Principal, name string, _ json.RawMessage) (*mcp.CallToolResult, error) {
		meta := mcpserver.ToolMeta{Name: name, Namespace: "git", Risk: mcpserver.RiskExec}
		d, err := gate.Decide(ctx, p, meta, nil)
		if err != nil {
			return nil, err
		}
		switch d.Outcome {
		case mcpserver.OutcomeDeny:
			return nil, errors.New("MCP_POLICY_DENIED: no")
		case mcpserver.OutcomeRequireApproval:
			if ok, _ := gate.AwaitApproval(ctx, p, d.ApprovalID); !ok {
				return nil, errors.New("MCP_APPROVAL_DENIED: no")
			}
		}
		return &mcp.CallToolResult{}, nil
	}), m, func(string) bool { return true })

	p := mcpserver.Principal{TenantID: "t", UserID: "u"}
	_, _ = ex.CallTool(context.Background(), p, "deny_me", nil)
	_, _ = ex.CallTool(context.Background(), p, "need_appr", nil)
	_, _ = ex.CallTool(context.Background(), p, "plain", nil)
	fg.Approved = false
	_, _ = ex.CallTool(context.Background(), p, "need_appr", nil)

	for _, c := range []struct {
		name   string
		labels map[string]string
		want   float64
	}{
		{"orca_mcp_tool_calls_total", map[string]string{"tool": "deny_me", "decision": "deny", "result": "denied"}, 1},
		{"orca_mcp_tool_calls_total", map[string]string{"tool": "need_appr", "decision": "require_approval", "result": "ok"}, 1},
		{"orca_mcp_tool_calls_total", map[string]string{"tool": "need_appr", "decision": "require_approval", "result": "denied"}, 1},
		{"orca_mcp_tool_calls_total", map[string]string{"tool": "plain", "decision": "allow", "result": "ok"}, 1},
		{"orca_mcp_policy_denials_total", map[string]string{"reason": "hard_deny"}, 1},
		{"orca_mcp_approvals_total", map[string]string{"outcome": "approved"}, 1},
		{"orca_mcp_approvals_total", map[string]string{"outcome": "denied"}, 1},
		{"orca_mcp_tool_duration_seconds", map[string]string{"risk": "exec", "namespace": "git"}, 2}, // plain + approved
	} {
		if got := counter(t, m, c.name, c.labels); got != c.want {
			t.Errorf("%s%v = %v, want %v", c.name, c.labels, got, c.want)
		}
	}
}

type viewGate struct{ *mcpservertest.FakeGate }

func (viewGate) EffectiveDecision(context.Context, string, mcpserver.ToolMeta) (mcpserver.EffectiveDecision, error) {
	return mcpserver.EffectiveDecision{Decision: mcpserver.OutcomeAllow, Source: "default"}, nil
}

type deciderGate struct{ *mcpservertest.FakeGate }

func (deciderGate) DecideByElicitation(context.Context, mcpserver.Principal, string, bool) error {
	return nil
}

// The executor/catalog type-assert optional gate capabilities; the decorator
// must neither drop nor invent them.
func TestInstrumentGatePreservesOptionalCapabilities(t *testing.T) {
	m := mcpmetrics.New()
	plain := mcpmetrics.InstrumentGate(&mcpservertest.FakeGate{}, m)
	if _, ok := plain.(mcpserver.ToolPolicyView); ok {
		t.Error("plain gate must not gain ToolPolicyView")
	}
	if _, ok := plain.(mcpserver.ElicitationDecider); ok {
		t.Error("plain gate must not gain ElicitationDecider")
	}
	v := mcpmetrics.InstrumentGate(viewGate{&mcpservertest.FakeGate{}}, m)
	if _, ok := v.(mcpserver.ToolPolicyView); !ok {
		t.Error("ToolPolicyView lost")
	}
	if _, ok := v.(mcpserver.ElicitationDecider); ok {
		t.Error("ElicitationDecider invented")
	}
	d := mcpmetrics.InstrumentGate(deciderGate{&mcpservertest.FakeGate{}}, m)
	if _, ok := d.(mcpserver.ElicitationDecider); !ok {
		t.Error("ElicitationDecider lost")
	}
	if mcpmetrics.InstrumentGate(nil, m) != nil {
		t.Error("nil gate must stay nil (fail-closed default is chosen by the caller)")
	}
}

type fakeDispatcher struct{}

func (fakeDispatcher) Dispatch(context.Context, wscompat.Identity, string, []json.RawMessage) (any, error) {
	return map[string]any{"ok": true}, nil
}
func (fakeDispatcher) DispatchStreamChannel(context.Context, wscompat.Identity, string, []json.RawMessage) (any, <-chan wscompat.PushEvent, bool, error) {
	return nil, nil, false, nil
}

// TestEndToEndMetricsAndTraceChain serves the real /mcp handler behind
// otelhttp and checks (a) counters appear on the scrape output and (b) the
// spans mcp.request -> mcp.policy -> mcp.dispatch share ONE trace id with the
// HTTP root span and never carry argument/secret attributes.
func TestEndToEndMetricsAndTraceChain(t *testing.T) {
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })

	m := mcpmetrics.New()
	gate := mcpmetrics.InstrumentGate(&mcpservertest.FakeGate{}, m)
	disp := mcpmetrics.TraceDispatcher(fakeDispatcher{})
	exec := mcpmetrics.InstrumentExecutor(execFunc(func(ctx context.Context, p mcpserver.Principal, name string, _ json.RawMessage) (*mcp.CallToolResult, error) {
		if _, err := gate.Decide(ctx, p, mcpserver.ToolMeta{Name: name, Namespace: "demo", Risk: mcpserver.RiskRead}, json.RawMessage(`{"secret":"hunter2"}`)); err != nil {
			return nil, err
		}
		if _, err := disp.Dispatch(ctx, wscompat.Identity{TenantID: p.TenantID, UserID: p.UserID}, "demo.echo", nil); err != nil {
			return nil, err
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}}, nil
	}), m, func(n string) bool { return n == "echo" })

	srv := httptest.NewUnstartedServer(nil)
	base := "http://" + srv.Listener.Addr().String()
	origins, _ := originpolicy.Parse("https://app.example.com")
	r := chi.NewRouter()
	mcpserver.NewHandler(mcpserver.Deps{
		Config: mcpserver.Config{ResourceURL: base + "/mcp", IssuerURL: "https://auth.example.com", AllowedOrigins: origins,
			SessionIdleTTL: time.Minute, ScopesSupported: []string{"orca:read"}, ServerVersion: "test"},
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Verifier: mcpservertest.StaticVerifier{Tokens: map[string]mcpserver.Principal{"tok": {TenantID: "t1", UserID: "alice", Scopes: []string{"orca:read"}}}},
		Catalog:  mcpservertest.FakeCatalog{Names: []string{"echo"}}, Executor: exec, Recorder: m,
		CursorKeys: [][]byte{[]byte("k1-0123456789abcdef0123456789abcd")},
	}).Mount(r)
	srv.Config.Handler = otelhttp.NewHandler(r, "api-gateway")
	srv.Start()
	t.Cleanup(srv.Close)

	// Unauthenticated -> transport-level failure + auth failure series.
	resp, err := http.Post(base+"/mcp", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	cs, err := mcp.NewClient(&mcp.Implementation{Name: "c", Version: "1"}, nil).Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint: base + "/mcp", HTTPClient: &http.Client{Transport: bearer("tok")}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	if _, err := cs.ListTools(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "echo", Arguments: map[string]any{"token": "s3cr3t"}}); err != nil {
		t.Fatal(err)
	}

	scrape := httptest.NewRecorder()
	m.Handler().ServeHTTP(scrape, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := scrape.Body.String()
	for _, want := range []string{
		`orca_mcp_requests_total{method="initialize",result="ok"} 1`,
		`orca_mcp_requests_total{method="tools/list",result="ok"} 1`,
		`orca_mcp_requests_total{method="tools/call",result="ok"} 1`,
		`orca_mcp_requests_total{method="http",result="http_4xx"}`, // >=1: the unauthenticated POST (the SDK client may add more)
		`orca_mcp_auth_failures_total{reason="no_credentials"} 1`,
		`orca_mcp_tool_calls_total{decision="allow",result="ok",tool="echo"} 1`,
		`orca_mcp_sessions_active 1`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("scrape lacks %q", want)
		}
	}
	if strings.Contains(body, "hunter2") || strings.Contains(body, "s3cr3t") {
		t.Error("argument values leaked into metrics")
	}

	var callTrace string
	byName := map[string][]sdktrace.ReadOnlySpan{}
	for _, s := range rec.Ended() {
		byName[s.Name()] = append(byName[s.Name()], s)
		for _, kv := range s.Attributes() {
			k := strings.ToLower(string(kv.Key))
			for _, bad := range []string{"args", "arguments", "authorization", "token", "secret", "cookie"} {
				if strings.Contains(k, bad) {
					t.Errorf("span %s has forbidden attribute %q", s.Name(), kv.Key)
				}
			}
		}
	}
	for _, s := range byName["mcp.dispatch"] {
		callTrace = s.SpanContext().TraceID().String()
	}
	if callTrace == "" {
		t.Fatalf("no mcp.dispatch span; got %v", spanNames(rec.Ended()))
	}
	for _, name := range []string{"mcp.request", "mcp.policy", "mcp.dispatch"} {
		found := false
		for _, s := range byName[name] {
			if s.SpanContext().TraceID().String() == callTrace {
				found = true
			}
		}
		if !found {
			t.Errorf("%s not in the dispatch trace %s (spans: %v)", name, callTrace, spanNames(rec.Ended()))
		}
	}
	// mcp.policy and mcp.dispatch must descend from mcp.request.
	reqIDs := map[string]bool{}
	for _, s := range byName["mcp.request"] {
		reqIDs[s.SpanContext().SpanID().String()] = true
	}
	for _, name := range []string{"mcp.policy", "mcp.dispatch"} {
		for _, s := range byName[name] {
			if !reqIDs[s.Parent().SpanID().String()] && name == "mcp.policy" {
				t.Errorf("%s parent %s is not an mcp.request span", name, s.Parent().SpanID())
			}
		}
	}
}

func spanNames(ss []sdktrace.ReadOnlySpan) []string {
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		out = append(out, s.Name())
	}
	return out
}

type bearer string

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+string(b))
	return http.DefaultTransport.RoundTrip(r)
}
