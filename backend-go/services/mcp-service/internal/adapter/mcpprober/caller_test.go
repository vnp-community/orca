package mcpprober

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/netip"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"
)

// serveCall answers tools/call with result; other methods use the default flow.
func serveCall(result any) func(http.ResponseWriter, *http.Request, map[string]any) bool {
	return func(w http.ResponseWriter, _ *http.Request, msg map[string]any) bool {
		if msg["method"] != "tools/call" {
			return false
		}
		reply(w, msg, result)
		return true
	}
}

func callTarget(e env) usecase.CallTarget { return usecase.CallTarget{URL: e.url} }

func TestCallTool_TextOnly(t *testing.T) {
	e := newEnv(t)
	var gotParams map[string]any
	e.srv.handler = func(w http.ResponseWriter, r *http.Request, msg map[string]any) bool {
		if msg["method"] == "tools/call" {
			gotParams, _ = msg["params"].(map[string]any)
		}
		return serveCall(map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": "alpha"},
				{"type": "image", "data": "AAAA", "mimeType": "image/png"},
				{"type": "resource", "resource": map[string]any{"uri": "x://y", "text": "gamma"}},
				{"type": "text", "text": "beta"},
			},
			"isError": true,
		})(w, r, msg)
	}
	p := e.prober(fixed("127.0.0.1"), allowOnlyLoopback)
	res, err := p.CallTool(context.Background(), callTarget(e), "echo", []byte(`{"q":"hi"}`), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "alpha\ngamma\nbeta" || !res.IsError || res.Truncated || res.SizeBytes != len(res.Text) {
		t.Fatalf("%+v", res)
	}
	if gotParams["name"] != "echo" || fmt.Sprint(gotParams["arguments"]) != "map[q:hi]" {
		t.Fatalf("params: %v", gotParams)
	}
	// initialize, initialized, tools/call, then session close (DELETE).
	if e.srv.hits.Load() != 4 {
		t.Fatalf("hits=%d", e.srv.hits.Load())
	}
}

func TestCallTool_TruncatesAtRuneBoundary(t *testing.T) {
	e := newEnv(t)
	text := strings.Repeat("Tiếng Việt có dấu. ", 20)
	e.srv.handler = serveCall(map[string]any{"content": []map[string]any{{"type": "text", "text": text}}})
	p := e.prober(fixed("127.0.0.1"), allowOnlyLoopback)
	for max := 1; max < 40; max++ {
		res, err := p.CallTool(context.Background(), callTarget(e), "echo", nil, max)
		if err != nil {
			t.Fatal(err)
		}
		if !utf8.ValidString(res.Text) || len(res.Text) > max || !res.Truncated || !strings.HasPrefix(text, res.Text) {
			t.Fatalf("max=%d: %q truncated=%v", max, res.Text, res.Truncated)
		}
	}
}

func TestCallTool_OversizedBodyFails(t *testing.T) {
	e := newEnv(t)
	e.srv.handler = serveCall(map[string]any{"content": []map[string]any{{"type": "text", "text": strings.Repeat("a", MaxBodyBytes+10)}}})
	p := e.prober(fixed("127.0.0.1"), allowOnlyLoopback)
	if _, err := p.CallTool(context.Background(), callTarget(e), "echo", nil, MaxBodyBytes); err == nil || !strings.Contains(err.Error(), "bad_response") {
		t.Fatalf("got %v", err)
	}
}

func TestCallTool_RedirectRefused(t *testing.T) {
	e := newEnv(t)
	e.srv.handler = func(w http.ResponseWriter, r *http.Request, msg map[string]any) bool {
		http.Redirect(w, r, "https://169.254.169.254/latest/meta-data", http.StatusFound)
		return true
	}
	p := e.prober(fixed("127.0.0.1"), allowOnlyLoopback)
	if _, err := p.CallTool(context.Background(), callTarget(e), "echo", nil, 100); err == nil || !strings.Contains(err.Error(), "redirect") {
		t.Fatalf("got %v", err)
	}
	if e.srv.hits.Load() != 1 {
		t.Fatalf("hits=%d", e.srv.hits.Load())
	}
}

func TestCallTool_SSRFBlocked(t *testing.T) {
	for _, ip := range []string{"127.0.0.1", "169.254.169.254", "10.0.0.5", "::1"} {
		e := newEnv(t)
		p := e.prober(fixed(ip), nil)
		if _, err := p.CallTool(context.Background(), callTarget(e), "echo", nil, 100); !isCode(err, domain.CodeServerSSRFBlocked) {
			t.Errorf("%s: %v", ip, err)
		}
		if _, err := p.ReadResource(context.Background(), callTarget(e), "file:///x", 100); !isCode(err, domain.CodeServerSSRFBlocked) {
			t.Errorf("%s read: %v", ip, err)
		}
		if e.srv.hits.Load() != 0 {
			t.Errorf("%s: server contacted", ip)
		}
	}
	p := New(Config{Resolver: fixed("8.8.8.8")})
	for _, u := range []string{"http://x.example.com/", "https://10.0.0.1/", "https://u:p@x.example.com/"} {
		if _, err := p.CallTool(context.Background(), usecase.CallTarget{URL: u}, "echo", nil, 100); err == nil {
			t.Errorf("%s must be rejected before any dial", u)
		}
	}
}

func TestCallTool_TimeoutIs20s(t *testing.T) {
	if CallTimeout != 20*time.Second || CallTimeout == TotalTimeout {
		t.Fatalf("CallTimeout=%v", CallTimeout)
	}
	if got := New(Config{}).callTimeout(); got != 20*time.Second {
		t.Fatalf("default call timeout %v", got)
	}
	// A server slower than the (overridden) budget makes the call fail as a timeout.
	e := newEnv(t)
	e.srv.handler = func(w http.ResponseWriter, r *http.Request, msg map[string]any) bool {
		if msg["method"] == "tools/call" {
			select {
			case <-time.After(3 * time.Second):
			case <-r.Context().Done():
			}
		}
		return false
	}
	p := New(Config{
		Policy: domain.ExternalURLPolicy{AllowedPorts: []int{e.port}}, Resolver: fixed("127.0.0.1"),
		Blocked: func(ip netip.Addr) bool { return !ip.Unmap().IsLoopback() }, CallTimeout: 300 * time.Millisecond,
		TLSConfig: &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}, //nolint:gosec // test server cert
	})
	start := time.Now()
	_, err := p.CallTool(context.Background(), callTarget(e), "echo", nil, 100)
	if err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("got %v", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("call outlived its budget: %v", time.Since(start))
	}
}

func TestCallTool_RPCErrorAndSSE(t *testing.T) {
	e := newEnv(t)
	e.srv.handler = func(w http.ResponseWriter, r *http.Request, msg map[string]any) bool {
		if msg["method"] != "tools/call" {
			return false
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: {\"jsonrpc\":\"2.0\",\"id\":%v,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"streamed\"}]}}\n\n", msg["id"])
		return true
	}
	p := e.prober(fixed("127.0.0.1"), allowOnlyLoopback)
	res, err := p.CallTool(context.Background(), callTarget(e), "echo", nil, 100)
	if err != nil || res.Text != "streamed" {
		t.Fatalf("%+v %v", res, err)
	}
	e.srv.handler = func(w http.ResponseWriter, r *http.Request, msg map[string]any) bool {
		if msg["method"] != "tools/call" {
			return false
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": msg["id"], "error": map[string]any{"code": -32602, "message": "unknown tool secret-detail"}})
		return true
	}
	if _, err := p.CallTool(context.Background(), callTarget(e), "nope", nil, 100); err == nil || !strings.Contains(err.Error(), "rpc_error") || strings.Contains(err.Error(), "secret-detail") {
		t.Fatalf("got %v", err)
	}
}

func TestCallTool_OnlyConfiguredHeadersAreSent(t *testing.T) {
	e := newEnv(t)
	e.srv.handler = serveCall(map[string]any{"content": []map[string]any{{"type": "text", "text": "ok"}}})
	p := e.prober(fixed("127.0.0.1"), allowOnlyLoopback)
	tg := usecase.CallTarget{URL: e.url, Headers: map[string]domain.SecretValue{"X-Api-Key": domain.NewSecretValue("k-9"), "Cookie": domain.NewSecretValue("c")}}
	if _, err := p.CallTool(context.Background(), tg, "echo", nil, 100); err != nil {
		t.Fatal(err)
	}
	for _, h := range e.srv.headers {
		if h.Get("X-Api-Key") != "k-9" || h.Get("Cookie") != "" || h.Get("Authorization") != "" {
			t.Fatalf("headers: %v", h)
		}
	}
}

func TestReadResource_TextBlobAndTruncation(t *testing.T) {
	e := newEnv(t)
	var gotURI any
	e.srv.handler = func(w http.ResponseWriter, r *http.Request, msg map[string]any) bool {
		if msg["method"] != "resources/read" {
			return false
		}
		gotURI = msg["params"].(map[string]any)["uri"]
		reply(w, msg, map[string]any{"contents": []map[string]any{
			{"uri": "file:///a", "mimeType": "text/markdown", "text": "Xin chào thế giới"},
			{"uri": "file:///b", "mimeType": "image/png", "blob": "AAAA"},
		}})
		return true
	}
	p := e.prober(fixed("127.0.0.1"), allowOnlyLoopback)
	res, err := p.ReadResource(context.Background(), callTarget(e), "file:///a", 1024)
	if err != nil || res.Text != "Xin chào thế giới" || res.MimeType != "text/markdown" || res.Truncated {
		t.Fatalf("%+v %v", res, err)
	}
	if gotURI != "file:///a" {
		t.Fatalf("uri %v", gotURI)
	}
	res, err = p.ReadResource(context.Background(), callTarget(e), "file:///a", 9) // cuts inside "à"
	if err != nil || !utf8.ValidString(res.Text) || !res.Truncated || len(res.Text) > 9 {
		t.Fatalf("%+v %v", res, err)
	}
}
