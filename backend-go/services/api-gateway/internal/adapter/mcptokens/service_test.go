package mcptokens

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

type fakeAuth struct {
	tokens    []*authv1.McpTokenInfo
	issueErr  error
	revokeErr error
	issued    *authv1.IssueMcpTokenRequest
	md        metadata.MD
	revoked   string
}

func (f *fakeAuth) IssueMcpToken(ctx context.Context, in *authv1.IssueMcpTokenRequest, _ ...grpc.CallOption) (*authv1.IssueMcpTokenResponse, error) {
	f.issued = in
	f.md, _ = metadata.FromOutgoingContext(ctx)
	if f.issueErr != nil {
		return nil, f.issueErr
	}
	return &authv1.IssueMcpTokenResponse{
		Token:  &authv1.McpTokenInfo{Jti: "jti-1", Name: in.GetName(), Scopes: in.GetScopes(), CreatedAt: timestamppb.Now(), ExpiresAt: timestamppb.New(time.Now().Add(24 * time.Hour))},
		Secret: "omp_SECRET.SECRET.SECRET",
	}, nil
}
func (f *fakeAuth) ListMcpTokens(context.Context, *emptypb.Empty, ...grpc.CallOption) (*authv1.ListMcpTokensResponse, error) {
	return &authv1.ListMcpTokensResponse{Tokens: f.tokens}, nil
}
func (f *fakeAuth) RevokeMcpToken(_ context.Context, in *authv1.RevokeMcpTokenRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	f.revoked = in.GetJti()
	return &emptypb.Empty{}, f.revokeErr
}

type fakePolicy struct{ resp *mcpv1.GetServerInfoResponse }

func (f fakePolicy) GetServerInfo(context.Context, *mcpv1.GetServerInfoRequest, ...grpc.CallOption) (*mcpv1.GetServerInfoResponse, error) {
	return f.resp, nil
}

var id = usecase.Identity{TenantID: "t1", UserID: "u1", Role: "user"}

func svc(a *fakeAuth, info *mcpv1.GetServerInfoResponse) *Service {
	return &Service{Auth: a, Policy: fakePolicy{info}}
}

func okInfo() *mcpv1.GetServerInfoResponse {
	return &mcpv1.GetServerInfoResponse{Enabled: true, MaxTokenDays: 90}
}

func codeOf(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}

func TestCreate_TenantMaxDaysAndMessage(t *testing.T) {
	a := &fakeAuth{}
	info := okInfo()
	info.MaxTokenDays = 30
	_, err := svc(a, info).Create(context.Background(), id, CreateInput{Name: "n", Scopes: []string{"orca:read"}, ExpiresInDays: 31})
	if codeOf(err) != "MCP_TOKEN_TOO_LONG" || err.Error() != "MCP_TOKEN_TOO_LONG: maximum is 30 days for this organization" {
		t.Fatalf("err = %v", err)
	}
	if a.issued != nil {
		t.Fatal("auth-service must not be called when policy rejects")
	}
	if _, err := svc(a, info).Create(context.Background(), id, CreateInput{Name: "n", Scopes: []string{"orca:read"}, ExpiresInDays: 30}); err != nil {
		t.Fatal(err)
	}
	// 0 days and >90 even when the tenant allows more.
	info.MaxTokenDays = 365
	for _, d := range []int{0, 91} {
		if _, err := svc(a, info).Create(context.Background(), id, CreateInput{Name: "n", Scopes: []string{"orca:read"}, ExpiresInDays: d}); codeOf(err) != "MCP_TOKEN_TOO_LONG" {
			t.Errorf("days=%d: %v", d, err)
		}
	}
}

func TestCreate_PolicyGates(t *testing.T) {
	in := CreateInput{Name: "n", Scopes: []string{"orca:read"}, ExpiresInDays: 5}
	off := okInfo()
	off.Enabled = false
	if _, err := svc(&fakeAuth{}, off).Create(context.Background(), id, in); codeOf(err) != "MCP_DISABLED" {
		t.Errorf("disabled: %v", err)
	}
	ks := okInfo()
	ks.KillSwitch = &mcpv1.KillSwitch{Active: true}
	if _, err := svc(&fakeAuth{}, ks).Create(context.Background(), id, in); codeOf(err) != "MCP_KILL_SWITCH_ACTIVE" {
		t.Errorf("kill switch: %v", err)
	}
	for _, scopes := range [][]string{nil, {"orca:bogus"}} {
		if _, err := svc(&fakeAuth{}, okInfo()).Create(context.Background(), id, CreateInput{Name: "n", Scopes: scopes, ExpiresInDays: 5}); codeOf(err) != "MCP_SCOPE_INVALID" {
			t.Errorf("scopes %v: %v", scopes, err)
		}
	}
}

func TestCreate_PerUserLimitCountsOnlyActive(t *testing.T) {
	now := time.Now()
	var toks []*authv1.McpTokenInfo
	for i := 0; i < 3; i++ {
		toks = append(toks, &authv1.McpTokenInfo{Jti: "a", CreatedAt: timestamppb.New(now), ExpiresAt: timestamppb.New(now.Add(time.Hour))})
	}
	toks = append(toks,
		&authv1.McpTokenInfo{Jti: "r", ExpiresAt: timestamppb.New(now.Add(time.Hour)), RevokedAt: timestamppb.New(now)},
		&authv1.McpTokenInfo{Jti: "e", ExpiresAt: timestamppb.New(now.Add(-time.Hour))})
	s := svc(&fakeAuth{tokens: toks}, okInfo())
	s.MaxActivePerUser = 3
	in := CreateInput{Name: "n", Scopes: []string{"orca:read"}, ExpiresInDays: 5}
	if _, err := s.Create(context.Background(), id, in); codeOf(err) != "MCP_TOKEN_LIMIT" {
		t.Fatalf("at limit: %v", err)
	}
	s.MaxActivePerUser = 4 // revoked + expired tokens do not count
	if _, err := s.Create(context.Background(), id, in); err != nil {
		t.Fatalf("under limit: %v", err)
	}
}

func TestCreate_IdentityFromSessionOnlyAndErrorMapping(t *testing.T) {
	a := &fakeAuth{}
	out, err := svc(a, okInfo()).Create(context.Background(), id, CreateInput{Name: "ci", Scopes: []string{"orca:write", "orca:read"}, ExpiresInDays: 7})
	if err != nil {
		t.Fatal(err)
	}
	if out.Secret == "" || out.Token.Status != "active" || out.Token.ID != "jti-1" {
		t.Fatalf("out = %+v", out)
	}
	if got := a.md.Get("x-orca-user-id"); len(got) != 1 || got[0] != "u1" {
		t.Fatalf("identity metadata = %v", a.md)
	}
	if strings.Join(a.issued.GetScopes(), ",") != "orca:read,orca:write" || a.issued.GetExpiresInDays() != 7 {
		t.Fatalf("issued = %+v", a.issued)
	}

	cases := map[string]string{
		"AUTH_MCP_SCOPE_NOT_ALLOWED": "MCP_SCOPE_NOT_ALLOWED", "AUTH_MCP_TOKEN_TOO_LONG": "MCP_TOKEN_TOO_LONG",
		"AUTH_MCP_SCOPE_INVALID": "MCP_SCOPE_INVALID", "AUTH_MCP_NOT_CONFIGURED": "MCP_DISABLED",
	}
	for authCode, want := range cases {
		a := &fakeAuth{issueErr: status.Error(codes.PermissionDenied, authCode+": detail")}
		_, err := svc(a, okInfo()).Create(context.Background(), id, CreateInput{Name: "n", Scopes: []string{"orca:admin"}, ExpiresInDays: 5})
		if codeOf(err) != want {
			t.Errorf("%s -> %v", authCode, err)
		}
	}
	a = &fakeAuth{issueErr: status.Error(codes.Internal, "pq: password=hunter2")}
	if _, err := svc(a, okInfo()).Create(context.Background(), id, CreateInput{Name: "n", Scopes: []string{"orca:read"}, ExpiresInDays: 5}); err == nil || strings.Contains(err.Error(), "hunter2") {
		t.Errorf("internal detail leaked: %v", err)
	}
}

func TestRevoke_NotFoundAndInvalidates(t *testing.T) {
	a := &fakeAuth{}
	var dropped string
	s := svc(a, okInfo())
	s.OnRevoked = func(j string) { dropped = j }
	if err := s.Revoke(context.Background(), id, "jti-9"); err != nil || dropped != "jti-9" || a.revoked != "jti-9" {
		t.Fatalf("revoke: %v dropped=%q", err, dropped)
	}
	a.revokeErr = status.Error(codes.NotFound, "AUTH_MCP_TOKEN_NOT_FOUND: token not found")
	dropped = ""
	if err := s.Revoke(context.Background(), id, "other"); codeOf(err) != "MCP_NOT_FOUND" || dropped != "" {
		t.Fatalf("not found: %v dropped=%q", err, dropped)
	}
}

func TestList_StatusMapping(t *testing.T) {
	now := time.Now()
	a := &fakeAuth{tokens: []*authv1.McpTokenInfo{
		{Jti: "1", ExpiresAt: timestamppb.New(now.Add(time.Hour)), CreatedAt: timestamppb.New(now)},
		{Jti: "2", ExpiresAt: timestamppb.New(now.Add(time.Hour)), RevokedAt: timestamppb.New(now)},
		{Jti: "3", ExpiresAt: timestamppb.New(now.Add(-time.Hour)), LastUsedAt: timestamppb.New(now)},
	}}
	got, err := svc(a, okInfo()).List(context.Background(), id)
	if err != nil || got[0].Status != "active" || got[1].Status != "revoked" || got[2].Status != "expired" || got[2].LastUsedAt == "" {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestCreate_SecretNeverLogged(t *testing.T) {
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(old)
	a := &fakeAuth{}
	if _, err := svc(a, okInfo()).Create(context.Background(), id, CreateInput{Name: "n", Scopes: []string{"orca:read"}, ExpiresInDays: 5}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "omp_") || strings.Contains(buf.String(), "SECRET") {
		t.Fatalf("log output contains secret: %s", buf.String())
	}
}
