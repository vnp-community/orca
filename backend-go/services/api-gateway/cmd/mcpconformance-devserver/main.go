// Command mcpconformance-devserver serves the REAL /mcp handler (mcpserver)
// with test doubles behind it: a static bearer token, two harmless tools and
// the OAuth discovery documents. It exists so the Python reference-client tier
// (backend-go/ci/mcp-conformance) can run against a real HTTP listener on a
// machine that has no full dev stack. It is NOT a deployment artifact: it
// trusts a hard-coded token and is never built into the service image.
//
//	MCP_CONF_ADDR        listen address (default 127.0.0.1:8099)
//	MCP_CONF_PUBLIC_URL  public base URL advertised in metadata (default http://<addr>)
//	MCP_CONF_TOKEN       the one accepted bearer token (default conformance-token)
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/stablyai/orca-go/common/mcpscope"
	"github.com/stablyai/orca-go/common/oauthmetadata"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/originpolicy"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver/mcpservertest"
)

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// tools: echo is a plain read-only call; slow_progress streams progress
// notifications (so there are SSE events with ids to resume from) and then
// answers.
type tools struct{}

func (tools) ListTools(context.Context, mcpserver.Principal) ([]*mcp.Tool, error) {
	obj := func(props map[string]any) map[string]any {
		return map[string]any{"type": "object", "properties": props}
	}
	return []*mcp.Tool{
		{Name: "echo", Description: "Echo the text argument.", InputSchema: obj(map[string]any{"text": map[string]any{"type": "string"}}),
			Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true}},
		{Name: "slow_progress", Description: "Emit progress notifications, then answer.", InputSchema: obj(map[string]any{"steps": map[string]any{"type": "integer"}}),
			Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true}},
	}, nil
}

func (tools) CallTool(ctx context.Context, _ mcpserver.Principal, name string, args json.RawMessage) (*mcp.CallToolResult, error) {
	var in struct {
		Text  string `json:"text"`
		Steps int    `json:"steps"`
	}
	_ = json.Unmarshal(args, &in)
	switch name {
	case "echo":
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: in.Text}}}, nil
	case "slow_progress":
		if in.Steps <= 0 || in.Steps > 20 {
			in.Steps = 10
		}
		for i := 0; i < in.Steps; i++ {
			select {
			case <-time.After(260 * time.Millisecond): // > the 5/s progress throttle, so every step is delivered
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			mcpserver.ReportProgress(ctx, float64(i+1), float64(in.Steps), fmt.Sprintf("step %d", i+1))
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "done"}}}, nil
	}
	return nil, mcpserver.ErrUnknownTool
}

func main() {
	addr := env("MCP_CONF_ADDR", "127.0.0.1:8099")
	base := env("MCP_CONF_PUBLIC_URL", "http://"+addr)
	token := env("MCP_CONF_TOKEN", "conformance-token")
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	origins, _ := originpolicy.Parse("")
	scopes := mcpscope.All()
	asDoc, err := oauthmetadata.BuildAuthorizationServer(base, true, scopes)
	if err != nil {
		log.Error("authorization server metadata", slog.Any("error", err))
		os.Exit(1)
	}
	asBody, _ := json.Marshal(asDoc)

	h := mcpserver.NewHandler(mcpserver.Deps{
		Logger: log,
		Config: mcpserver.Config{ResourceURL: oauthmetadata.ResourceURL(base), IssuerURL: base, AllowedOrigins: origins,
			SessionIdleTTL: 5 * time.Minute, ScopesSupported: scopes, ServerVersion: "conformance-devserver",
			MaxStreamsPerUser: 5, MaxStreamsPerTenant: 20},
		Verifier: mcpservertest.StaticVerifier{Tokens: map[string]mcpserver.Principal{
			token: {TenantID: "conf-tenant", UserID: "conf-user", Scopes: scopes},
		}},
		Catalog: tools{}, Executor: tools{},
		CursorKeys: [][]byte{[]byte("conformance-devserver-cursor-key-0123456789")},
		AuthServerMetadata: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(asBody)
		}),
	})
	defer h.Close()
	r := chi.NewRouter()
	h.Mount(r)

	srv := &http.Server{Addr: addr, Handler: r, ReadHeaderTimeout: 5 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(sctx)
	}()
	log.Info("mcpconformance-devserver listening", slog.String("addr", addr), slog.String("resource", oauthmetadata.ResourceURL(base)))
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error("serve", slog.Any("error", err))
		os.Exit(1)
	}
}
