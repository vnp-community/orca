package resources

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver/mcpservertest"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/originpolicy"
)

type bearer string

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+string(b))
	return http.DefaultTransport.RoundTrip(r)
}

// serve mounts the real adapter (chi + SDK handler) with the given providers.
func serve(t *testing.T, res mcpserver.ResourceProvider, prm mcpserver.PromptProvider) (url string, h *mcpserver.Handler) {
	t.Helper()
	srv := httptest.NewUnstartedServer(nil)
	base := "http://" + srv.Listener.Addr().String()
	origins, _ := originpolicy.Parse("https://app.example.com")
	h = mcpserver.NewHandler(mcpserver.Deps{
		Logger: quietLog(),
		Config: mcpserver.Config{ResourceURL: base + "/mcp", IssuerURL: "https://auth.example.com", AllowedOrigins: origins,
			PageSize: 2, SessionIdleTTL: time.Minute, ServerVersion: "test"},
		Verifier: mcpservertest.StaticVerifier{Tokens: map[string]mcpserver.Principal{
			"alice":  alice,
			"noread": {TenantID: "t1", UserID: "nr", Scopes: []string{"orca:write"}},
		}},
		CursorKeys: [][]byte{[]byte("k1-0123456789abcdef0123456789abcd")},
		Resources:  res, Prompts: prm,
	})
	r := chi.NewRouter()
	h.Mount(r)
	srv.Config.Handler = r
	srv.Start()
	t.Cleanup(srv.Close)
	return base + "/mcp", h
}

func connect(t *testing.T, url, token string, opts *mcp.ClientOptions) *mcp.ClientSession {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "conformance", Version: "1"}, opts).Connect(ctx,
		&mcp.StreamableClientTransport{Endpoint: url, HTTPClient: &http.Client{Transport: bearer(token)}}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func rpcCode(err error) int64 {
	var e *jsonrpc.Error
	if errors.As(err, &e) {
		return e.Code
	}
	return 0
}

func TestConformance_ResourcesOverSDKClient(t *testing.T) {
	gate := &mcpservertest.FakeGate{}
	src := newFakeSource()
	p := NewProvider(scripted(), gate, src, Config{Debounce: 20 * time.Millisecond}, quietLog())
	t.Cleanup(p.Close)
	url, _ := serve(t, p, nil)

	updated := make(chan string, 4)
	cs := connect(t, url, "alice", &mcp.ClientOptions{
		ResourceUpdatedHandler: func(_ context.Context, r *mcp.ResourceUpdatedNotificationRequest) { updated <- r.Params.URI },
	})
	ctx := context.Background()

	caps := cs.InitializeResult().Capabilities
	if caps.Resources == nil || !caps.Resources.Subscribe || caps.Resources.ListChanged {
		t.Fatalf("resources capability must be {subscribe:true, listChanged:false}: %+v", caps.Resources)
	}
	if caps.Prompts != nil {
		t.Fatalf("prompts must not be declared without a provider: %+v", caps.Prompts)
	}

	lr, err := cs.ListResources(ctx, nil)
	if err != nil || len(lr.Resources) != 2 || lr.Resources[0].URI != "orca://projects" || lr.Resources[1].URI != "orca://project/"+idA {
		t.Fatalf("resources/list: %+v %v", lr, err)
	}
	lt, err := cs.ListResourceTemplates(ctx, nil)
	if err != nil || len(lt.ResourceTemplates) < 5 {
		t.Fatalf("resources/templates/list: %+v %v", lt, err)
	}
	for _, tpl := range lt.ResourceTemplates {
		if !strings.HasPrefix(tpl.URITemplate, "orca://") || tpl.MIMEType == "" {
			t.Errorf("bad template %+v", tpl)
		}
		if strings.Contains(tpl.URITemplate, "/file/") {
			t.Error("file template must be hidden by default")
		}
	}

	rr, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: "orca://task/" + idA})
	if err != nil || len(rr.Contents) != 1 || !strings.Contains(rr.Contents[0].Text, "Fix bug") || rr.Contents[0].Meta["orca/untrusted"] != true {
		t.Fatalf("resources/read: %+v %v", rr, err)
	}
	if got := gate.Decided[len(gate.Decided)-1]; got.Name != "resource:task" {
		t.Fatalf("policy name = %q", got.Name)
	}

	// Malformed, unknown scheme, disabled file, sensitive path and denied: one identical error.
	var firstMsg string
	for _, u := range []string{"orca://task/" + "44444444-4444-4444-8444-444444444444x", "http://evil/x", "orca://worktree/" + idA + "/file/a.txt", "orca://worktree/" + idA + "/diff?path=.env"} {
		_, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: u})
		if err == nil || rpcCode(err) != jsonrpc.CodeInvalidParams || !strings.Contains(err.Error(), "Resource not found") {
			t.Fatalf("%s: %v", u, err)
		}
		msg := strings.Replace(err.Error(), u, "<uri>", -1)
		if firstMsg == "" {
			firstMsg = msg
		}
	}

	// Subscribe -> event -> URI-only notification.
	if err := cs.Subscribe(ctx, &mcp.SubscribeParams{URI: "orca://task/" + idA}); err != nil {
		t.Fatal(err)
	}
	<-src.attached
	src.emit("t1", idA)
	select {
	case u := <-updated:
		if u != "orca://task/"+idA {
			t.Fatalf("updated uri %q", u)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no notifications/resources/updated")
	}
	// Not subscribable -> -32602 with the spec wording.
	err = cs.Subscribe(ctx, &mcp.SubscribeParams{URI: "orca://project/" + idA})
	if rpcCode(err) != jsonrpc.CodeInvalidParams || !strings.Contains(err.Error(), "subscription not supported") {
		t.Fatalf("subscribe project: %v", err)
	}
	// Unsubscribe releases the source; re-subscribe then end the session.
	if err := cs.Unsubscribe(ctx, &mcp.UnsubscribeParams{URI: "orca://task/" + idA}); err != nil {
		t.Fatal(err)
	}
	eventually(t, "source stopped after unsubscribe", func() bool { return src.running.Load() == 0 })

	if err := cs.Subscribe(ctx, &mcp.SubscribeParams{URI: "orca://task/" + idA}); err != nil {
		t.Fatal(err)
	}
	<-src.attached
	_ = cs.Close() // session ends: DELETE + connection loss
	eventually(t, "subscriptions cleaned on session end", func() bool {
		p.subs.mu.Lock()
		defer p.subs.mu.Unlock()
		return len(p.subs.bySession) == 0 && src.running.Load() == 0
	})
}

func TestConformance_ResourcesScopeAndNoProvider(t *testing.T) {
	p := NewProvider(scripted(), &mcpservertest.FakeGate{}, nil, Config{}, quietLog())
	url, _ := serve(t, p, nil)
	cs := connect(t, url, "noread", nil)
	lr, err := cs.ListResources(context.Background(), nil)
	if err != nil || len(lr.Resources) != 0 {
		t.Fatalf("token without orca:read must list nothing: %+v %v", lr, err)
	}
	if _, err := cs.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: "orca://projects"}); err == nil {
		t.Fatal("read without scope must fail")
	}
	// No event source -> subscribe is not declared and the method is unknown.
	if c := cs.InitializeResult().Capabilities; c.Resources == nil || c.Resources.Subscribe {
		t.Fatalf("subscribe must not be declared without an event source: %+v", c.Resources)
	}
	if err := cs.Subscribe(context.Background(), &mcp.SubscribeParams{URI: "orca://task/" + idA}); err == nil {
		t.Fatal("subscribe must fail")
	}

	// No providers at all: capabilities stay tools+logging only.
	url2, _ := serve(t, nil, nil)
	cs2 := connect(t, url2, "alice", nil)
	if c := cs2.InitializeResult().Capabilities; c.Resources != nil || c.Prompts != nil {
		t.Fatalf("unimplemented capabilities declared: %+v", c)
	}
}
