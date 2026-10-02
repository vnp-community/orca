package wscompat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

const secretSentinel = "SENTINEL-plaintext-secret-8d41"

type fakeRegistry struct {
	mcpv1.McpRegistryServiceClient
	upsert  *mcpv1.UpsertExternalServerRequest
	secret  *mcpv1.SetExternalServerSecretRequest
	review  *mcpv1.ReviewExternalServerRequest
	listReq *mcpv1.ListExternalServersRequest
	err     error
	calls   int
}

var sample = &mcpv1.ExternalServer{
	Id: "s1", Scope: "tenant", ScopeId: "t", Name: "gh", Transport: "http", Url: "https://x.example.com/mcp", Status: "pending_review",
	HeaderRefs: []*mcpv1.ExternalSecretRef{{Name: "X-Api-Key", HasSecret: true}}, ToolsDigest: "d", ToolsChanged: true, CreatedBy: "u",
	HasHealth: true, HealthOk: true, HealthCheckedAt: timestamppb.New(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)),
	UpdatedAt: timestamppb.New(time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)),
}

func (f *fakeRegistry) ListExternalServers(_ context.Context, in *mcpv1.ListExternalServersRequest, _ ...grpc.CallOption) (*mcpv1.ListExternalServersResponse, error) {
	f.calls++
	f.listReq = in
	return &mcpv1.ListExternalServersResponse{Servers: []*mcpv1.ExternalServer{sample}}, f.err
}
func (f *fakeRegistry) UpsertExternalServer(_ context.Context, in *mcpv1.UpsertExternalServerRequest, _ ...grpc.CallOption) (*mcpv1.UpsertExternalServerResponse, error) {
	f.calls++
	f.upsert = in
	if f.err != nil {
		return nil, f.err
	}
	return &mcpv1.UpsertExternalServerResponse{Server: sample}, nil
}
func (f *fakeRegistry) SetExternalServerSecret(_ context.Context, in *mcpv1.SetExternalServerSecretRequest, _ ...grpc.CallOption) (*mcpv1.SetExternalServerSecretResponse, error) {
	f.calls++
	f.secret = in
	if f.err != nil {
		return nil, f.err
	}
	return &mcpv1.SetExternalServerSecretResponse{HasSecret: true}, nil
}
func (f *fakeRegistry) ProbeExternalServer(_ context.Context, _ *mcpv1.ProbeExternalServerRequest, _ ...grpc.CallOption) (*mcpv1.ProbeExternalServerResponse, error) {
	f.calls++
	return &mcpv1.ProbeExternalServerResponse{Transport: "http", Digest: "dg",
		Tools:         []*mcpv1.ExternalToolInfo{{Name: "echo", Description: "e"}},
		ApprovedTools: []*mcpv1.ExternalToolInfo{{Name: "echo", Description: "old"}}}, f.err
}
func (f *fakeRegistry) ReviewExternalServer(_ context.Context, in *mcpv1.ReviewExternalServerRequest, _ ...grpc.CallOption) (*mcpv1.ReviewExternalServerResponse, error) {
	f.calls++
	f.review = in
	if f.err != nil {
		return nil, f.err
	}
	return &mcpv1.ReviewExternalServerResponse{Server: sample}, nil
}
func (f *fakeRegistry) DeleteExternalServer(_ context.Context, _ *mcpv1.DeleteExternalServerRequest, _ ...grpc.CallOption) (*mcpv1.DeleteExternalServerResponse, error) {
	f.calls++
	return &mcpv1.DeleteExternalServerResponse{Ok: true}, f.err
}

func extDeps(f *fakeRegistry) McpChannelDeps { return McpChannelDeps{Enabled: true, Registry: f} }

func dispatchExt(t *testing.T, d McpChannelDeps, role, channel, args string) (string, error) {
	t.Helper()
	r := NewRegistry()
	RegisterMcpChannels(r, d)
	var a []json.RawMessage
	if args != "" {
		a = []json.RawMessage{json.RawMessage(args)}
	}
	out, err := r.Dispatch(context.Background(), Identity{TenantID: "t", UserID: "u", Role: role}, channel, a)
	if err != nil {
		return "", err
	}
	b, _ := json.Marshal(out)
	return string(b), nil
}

func TestMcpExternalServer_ListShapesContractType(t *testing.T) {
	f := &fakeRegistry{}
	got, err := dispatchExt(t, extDeps(f), "user", "mcp.externalServer.list", `{"scope":"user"}`)
	if err != nil {
		t.Fatal(err)
	}
	if f.listReq.GetScope() != "user" {
		t.Fatal("scope filter must be forwarded")
	}
	for _, want := range []string{`"envRefs":[]`, `"headerRefs":[{"name":"X-Api-Key","hasSecret":true}]`, `"toolsChanged":true`,
		`"health":{"ok":true,"checkedAt":"2026-10-01T00:00:00Z"}`, `"status":"pending_review"`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in %s", want, got)
		}
	}
	if _, err := dispatchExt(t, extDeps(&fakeRegistry{}), "user", "mcp.externalServer.list", ""); err != nil {
		t.Errorf("list without args must work: %v", err)
	}
}

func TestMcpExternalServer_UpsertForwardsOnlySpecFields(t *testing.T) {
	f := &fakeRegistry{}
	in := `{"id":"","scope":"user","name":"gh","transport":"http","url":"https://x.example.com/mcp","status":"approved","toolsDigest":"forged",
		"toolsChanged":false,"createdBy":"someone-else","headerRefs":[{"name":"X-Api-Key","hasSecret":true}],"envRefs":[]}`
	if _, err := dispatchExt(t, extDeps(f), "user", "mcp.externalServer.upsert", in); err != nil {
		t.Fatal(err)
	}
	u := f.upsert
	if u.GetName() != "gh" || len(u.GetHeaderRefNames()) != 1 || u.GetHeaderRefNames()[0] != "X-Api-Key" {
		t.Fatalf("%+v", u)
	}
	// the request type has no field for status/digest/actor, so a forged value cannot travel
	b, _ := json.Marshal(u)
	for _, banned := range []string{"approved", "forged", "someone-else"} {
		if strings.Contains(string(b), banned) {
			t.Errorf("forged field %q reached mcp-service: %s", banned, b)
		}
	}
}

func TestMcpExternalServer_ReviewIsAdminOnlyAtGateway(t *testing.T) {
	f := &fakeRegistry{}
	args := `{"serverId":"s1","decision":"approve","toolsDigest":"d"}`
	if _, err := dispatchExt(t, extDeps(f), "user", "mcp.externalServer.review", args); err == nil || err.Error() != "MCP_NOT_ADMIN: admin role required" {
		t.Fatalf("%v", err)
	}
	if f.calls != 0 {
		t.Fatal("non-admin review must not reach mcp-service")
	}
	if _, err := dispatchExt(t, extDeps(f), "admin", "mcp.externalServer.review", args); err != nil {
		t.Fatal(err)
	}
	if f.review.GetToolsDigest() != "d" || f.review.GetDecision() != "approve" {
		t.Fatalf("%+v", f.review)
	}
}

func TestMcpExternalServer_ProbeAndDeleteAndErrorCodes(t *testing.T) {
	f := &fakeRegistry{}
	got, err := dispatchExt(t, extDeps(f), "user", "mcp.externalServer.probe", `{"serverId":"s1"}`)
	if err != nil || !strings.Contains(got, `"approvedTools":[{"name":"echo","description":"old"}]`) || !strings.Contains(got, `"digest":"dg"`) {
		t.Fatalf("%s %v", got, err)
	}
	if got, _ := dispatchExt(t, extDeps(f), "user", "mcp.externalServer.delete", `{"serverId":"s1"}`); got != `{"ok":true}` {
		t.Fatalf("%s", got)
	}
	for code, want := range map[string]string{
		"MCP_SERVER_DIGEST_MISMATCH: tools changed since the last probe; probe again before reviewing": "MCP_SERVER_DIGEST_MISMATCH: tools changed since the last probe; probe again before reviewing",
		"MCP_SERVER_SSRF_BLOCKED: url must use https":                                                  "MCP_SERVER_SSRF_BLOCKED: url must use https",
		"MCP_SERVER_STDIO_NOT_ALLOWED: stdio servers are disabled on this deployment":                  "MCP_SERVER_STDIO_NOT_ALLOWED: stdio servers are disabled on this deployment",
	} {
		_, err := dispatchExt(t, extDeps(&fakeRegistry{err: status.Error(codes.FailedPrecondition, code)}), "admin", "mcp.externalServer.review", `{"serverId":"s1","decision":"approve","toolsDigest":"x"}`)
		if err == nil || err.Error() != want {
			t.Errorf("got %v want %s", err, want)
		}
	}
	if _, err := dispatchExt(t, McpChannelDeps{Enabled: true}, "admin", "mcp.externalServer.list", ""); err == nil || !strings.HasPrefix(err.Error(), "MCP_UNAVAILABLE") {
		t.Fatalf("no registry: %v", err)
	}
	if _, err := dispatchExt(t, McpChannelDeps{Enabled: false, Registry: f}, "admin", "mcp.externalServer.list", ""); err == nil || !strings.HasPrefix(err.Error(), "MCP_DISABLED") {
		t.Fatalf("disabled: %v", err)
	}
}

func setSecretArgs() string {
	return `{"serverId":"s1","kind":"header","name":"X-Api-Key","value":"` + secretSentinel + `"}`
}

func TestSetSecret_ResponseHasNoValueAndForwardsPlaintextOnce(t *testing.T) {
	f := &fakeRegistry{}
	got, err := dispatchExt(t, extDeps(f), "user", "mcp.externalServer.setSecret", setSecretArgs())
	if err != nil || got != `{"hasSecret":true}` {
		t.Fatalf("%s %v", got, err)
	}
	if f.secret.GetValue() != secretSentinel || f.secret.GetServerId() != "s1" {
		t.Fatalf("the plaintext must reach mcp-service exactly once: %+v", f.secret)
	}
}

func TestSetSecret_ErrorDoesNotEchoValue(t *testing.T) {
	errs := []error{
		status.Error(codes.Internal, "vault write failed for "+secretSentinel),
		errors.New("MCP_INVALID_ARGUMENT: bad value " + secretSentinel),
		errors.New("rpc error: code = Unknown desc = " + secretSentinel),
		status.Error(codes.InvalidArgument, "MCP_INVALID_ARGUMENT: value is not acceptable"),
	}
	for i, e := range errs {
		_, err := dispatchExt(t, extDeps(&fakeRegistry{err: e}), "user", "mcp.externalServer.setSecret", setSecretArgs())
		if err == nil {
			t.Fatalf("case %d must fail", i)
		}
		if strings.Contains(err.Error(), secretSentinel) {
			t.Errorf("case %d echoes the value: %v", i, err)
		}
	}
	for _, bad := range []string{`{"serverId":1,"value":"` + secretSentinel + `"}`, `not json ` + secretSentinel, `{"serverId":"s","kind":"env","name":"n","value":""}`} {
		_, err := dispatchExt(t, extDeps(&fakeRegistry{}), "user", "mcp.externalServer.setSecret", bad)
		if err == nil || strings.Contains(err.Error(), secretSentinel) || !strings.HasPrefix(err.Error(), "MCP_INVALID_ARGUMENT") {
			t.Errorf("decode failure must be a static coded error: %v", err)
		}
	}
}

func TestSensitiveChannelArgsRedacted(t *testing.T) {
	if !IsSensitiveChannel("mcp.externalServer.setSecret") || IsSensitiveChannel("mcp.externalServer.upsert") || IsSensitiveChannel("mcp.externalServer.list") {
		t.Fatal("only setSecret is sensitive")
	}
	args := []json.RawMessage{json.RawMessage(setSecretArgs())}
	red, _ := json.Marshal(RedactChannelArgs("mcp.externalServer.setSecret", args))
	if strings.Contains(string(red), secretSentinel) || !strings.Contains(string(red), "[redacted]") {
		t.Fatalf("redacted args: %s", red)
	}
	if string(RedactChannelArgs("mcp.externalServer.list", args)[0]) != string(args[0]) {
		t.Fatal("ordinary channels are not altered")
	}

	// A buggy handler that echoes the value is neutralised by Registry.Dispatch.
	r := NewRegistry()
	r.Register("mcp.externalServer.setSecret", func(context.Context, Identity, []json.RawMessage) (any, error) {
		return nil, errors.New("MCP_INTERNAL: boom " + secretSentinel)
	})
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t", UserID: "u"}, "mcp.externalServer.setSecret", args)
	if err == nil || strings.Contains(err.Error(), secretSentinel) {
		t.Fatalf("Dispatch must scrub: %v", err)
	}
	r.Register("other.channel", func(context.Context, Identity, []json.RawMessage) (any, error) {
		return nil, errors.New("plain error")
	})
	if _, err := r.Dispatch(context.Background(), Identity{}, "other.channel", args); err == nil || err.Error() != "plain error" {
		t.Fatalf("non-sensitive channels keep their errors: %v", err)
	}
}

// End to end through the real WebSocket handler, both dialects: neither the
// frames the client receives nor anything the gateway logs may contain the value.
func TestSetSecret_WebSocketPathNeverLogsOrEchoesValue(t *testing.T) {
	for _, tc := range []struct {
		name string
		reg  *fakeRegistry
	}{
		{"success", &fakeRegistry{}},
		{"downstream failure echoing value", &fakeRegistry{err: status.Error(codes.Internal, "boom "+secretSentinel)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
			registry := NewRegistry()
			RegisterMcpChannels(registry, extDeps(tc.reg))
			h := New(logger, fakeSessionValidator{identity: Identity{TenantID: "tenant-1", UserID: "user-1"}}, nil, registry)
			ts := httptest.NewServer(http.HandlerFunc(h.ServeHTTP))
			defer ts.Close()
			client := dialTestClient(t, ts)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			native := `{"id":"n1","type":"invoke","channel":"mcp.externalServer.setSecret","args":[` + setSecretArgs() + `]}`
			session := `{"id":"s1","authToken":"x","method":"mcp.externalServer.setSecret","params":` + setSecretArgs() + `}`
			for _, frame := range []string{native, session} {
				if err := writeRaw(ctx, client, frame); err != nil {
					t.Fatal(err)
				}
				got := readRawFrame(t, ctx, client)
				if strings.Contains(got, secretSentinel) {
					t.Fatalf("frame echoes the value: %s", got)
				}
			}
			if strings.Contains(logs.String(), secretSentinel) {
				t.Fatalf("gateway logs contain the value: %s", logs.String())
			}
		})
	}
}
