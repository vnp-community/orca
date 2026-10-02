package prompts_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
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
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver/prompts"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver/resources"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/originpolicy"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"
)

const taskID = "11111111-1111-4111-8111-111111111111"

type taskDispatcher struct{}

func (taskDispatcher) Dispatch(_ context.Context, id wscompat.Identity, channel string, _ []json.RawMessage) (any, error) {
	if id.TenantID == "" {
		return nil, errors.New("no identity")
	}
	switch channel {
	case "task.get":
		return map[string]any{"id": taskID, "title": "Ship it"}, nil
	case "task.getDependencies":
		return map[string]any{"edges": []any{}}, nil
	case "task.listComments":
		return map[string]any{"comments": []any{}}, nil
	}
	return nil, errors.New("unexpected channel " + channel)
}

type bearer string

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+string(b))
	return http.DefaultTransport.RoundTrip(r)
}

type store struct{ list []prompts.Custom }

func (s *store) ListCustom(context.Context, mcpserver.Principal) ([]prompts.Custom, error) {
	return s.list, nil
}

func TestConformance_PromptsOverSDKClient(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	res := resources.NewProvider(taskDispatcher{}, &mcpservertest.FakeGate{}, nil, resources.Config{}, log)
	st := &store{list: []prompts.Custom{{Name: "team_standup", Description: "Daily", Template: "Hello {{team}}", Arguments: []prompts.Argument{{Name: "team", Required: true}}}}}
	pr := prompts.NewProvider(st, res, log)

	srv := httptest.NewUnstartedServer(nil)
	base := "http://" + srv.Listener.Addr().String()
	origins, _ := originpolicy.Parse("https://app.example.com")
	h := mcpserver.NewHandler(mcpserver.Deps{
		Logger: log,
		Config: mcpserver.Config{ResourceURL: base + "/mcp", IssuerURL: "https://auth.example.com", AllowedOrigins: origins,
			PageSize: 2, SessionIdleTTL: time.Minute, ServerVersion: "test"},
		Verifier: mcpservertest.StaticVerifier{Tokens: map[string]mcpserver.Principal{
			"u": {TenantID: "t1", UserID: "u1", Role: "user", Scopes: []string{"orca:read"}},
		}},
		CursorKeys: [][]byte{[]byte("k1-0123456789abcdef0123456789abcd")},
		Resources:  res, Prompts: pr,
	})
	r := chi.NewRouter()
	h.Mount(r)
	srv.Config.Handler = r
	srv.Start()
	t.Cleanup(srv.Close)

	changed := make(chan struct{}, 4)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "conf", Version: "1"}, &mcp.ClientOptions{
		PromptListChangedHandler: func(context.Context, *mcp.PromptListChangedRequest) { changed <- struct{}{} },
	}).Connect(ctx, &mcp.StreamableClientTransport{Endpoint: base + "/mcp", HTTPClient: &http.Client{Transport: bearer("u")}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	if c := cs.InitializeResult().Capabilities; c.Prompts == nil || !c.Prompts.ListChanged {
		t.Fatalf("prompts capability: %+v", c.Prompts)
	}

	// Pagination with the signed cursor (page size 2): 5 built-ins + 1 custom.
	var names []string
	cursor := ""
	pages := 0
	for {
		lp, err := cs.ListPrompts(ctx, &mcp.ListPromptsParams{Cursor: cursor})
		if err != nil {
			t.Fatal(err)
		}
		pages++
		for _, p := range lp.Prompts {
			names = append(names, p.Name)
		}
		if cursor = lp.NextCursor; cursor == "" {
			break
		}
	}
	if pages != 3 || strings.Join(names, ",") != "review_pull_request,triage_issue,plan_task,summarize_worktree,handoff_to_agent,team_standup" {
		t.Fatalf("pages=%d names=%v", pages, names)
	}
	if _, err := cs.ListPrompts(ctx, &mcp.ListPromptsParams{Cursor: "forged"}); err == nil {
		t.Fatal("forged cursor must fail")
	}

	// prompts/get with an embedded resource read through the REAL resource provider.
	gp, err := cs.GetPrompt(ctx, &mcp.GetPromptParams{Name: "plan_task", Arguments: map[string]string{"task_id": taskID}})
	if err != nil || len(gp.Messages) != 2 {
		t.Fatalf("%+v %v", gp, err)
	}
	er, ok := gp.Messages[1].Content.(*mcp.EmbeddedResource)
	if !ok || er.Resource.URI != "orca://task/"+taskID || !strings.Contains(er.Resource.Text, "Ship it") || er.Resource.Meta["orca/untrusted"] != true {
		t.Fatalf("embedded resource: %+v", gp.Messages[1].Content)
	}
	// Locale from Accept-Language is exercised in unit tests; custom prompt over the wire:
	gp, err = cs.GetPrompt(ctx, &mcp.GetPromptParams{Name: "team_standup", Arguments: map[string]string{"team": "Core"}})
	if err != nil || !strings.HasPrefix(gp.Messages[0].Content.(*mcp.TextContent).Text, "Hello Core") {
		t.Fatalf("%+v %v", gp, err)
	}
	// Validation errors are protocol-level -32602.
	for _, params := range []*mcp.GetPromptParams{
		{Name: "plan_task"},
		{Name: "plan_task", Arguments: map[string]string{"task_id": "x"}},
		{Name: "plan_task", Arguments: map[string]string{"task_id": taskID, "z": "1"}},
		{Name: "no_such_prompt"},
	} {
		_, err := cs.GetPrompt(ctx, params)
		var e *jsonrpc.Error
		if !errors.As(err, &e) || e.Code != jsonrpc.CodeInvalidParams {
			t.Errorf("%+v: %v", params, err)
		}
	}

	// list_changed on prompt change.
	h.NotifyPromptsChanged()
	select {
	case <-changed:
	case <-time.After(3 * time.Second):
		t.Fatal("no notifications/prompts/list_changed")
	}
	// The sentinel never leaks into prompts/list.
	lp, _ := cs.ListPrompts(ctx, &mcp.ListPromptsParams{Cursor: ""})
	for _, p := range lp.Prompts {
		if strings.Contains(p.Name, "sentinel") {
			t.Fatal("sentinel prompt listed")
		}
	}
}
