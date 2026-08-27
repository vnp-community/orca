package httpgateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

	aiproviderv1 "github.com/stablyai/orca-go/proto/gen/go/orca/aiprovider/v1"
)

// fakeAIProviderServiceClient implements aiproviderv1.AiProviderServiceClient
// entirely in-memory, configurable to return either a canned response or a
// gRPC status error per method, and records the last request/context it saw
// so tests can assert on what mountAIProviderRoutes actually sent upstream.
type fakeAIProviderServiceClient struct {
	createAccountResp    *aiproviderv1.CreateAccountResponse
	createAccountErr     error
	lastCreateAccountReq *aiproviderv1.CreateAccountRequest

	resolveProviderResp    *aiproviderv1.ResolveProviderResponse
	resolveProviderErr     error
	lastResolveProviderReq *aiproviderv1.ResolveProviderRequest

	rotateKeyResp    *aiproviderv1.RotateKeyResponse
	rotateKeyErr     error
	lastRotateKeyReq *aiproviderv1.RotateKeyRequest

	getUsageTodayResp    *aiproviderv1.GetUsageTodayResponse
	getUsageTodayErr     error
	lastGetUsageTodayReq *aiproviderv1.GetUsageTodayRequest

	lastCtx context.Context
}

func (f *fakeAIProviderServiceClient) CreateAccount(ctx context.Context, in *aiproviderv1.CreateAccountRequest, _ ...grpc.CallOption) (*aiproviderv1.CreateAccountResponse, error) {
	f.lastCtx = ctx
	f.lastCreateAccountReq = in
	if f.createAccountErr != nil {
		return nil, f.createAccountErr
	}
	return f.createAccountResp, nil
}

func (f *fakeAIProviderServiceClient) ResolveProvider(ctx context.Context, in *aiproviderv1.ResolveProviderRequest, _ ...grpc.CallOption) (*aiproviderv1.ResolveProviderResponse, error) {
	f.lastCtx = ctx
	f.lastResolveProviderReq = in
	if f.resolveProviderErr != nil {
		return nil, f.resolveProviderErr
	}
	return f.resolveProviderResp, nil
}

func (f *fakeAIProviderServiceClient) RotateKey(ctx context.Context, in *aiproviderv1.RotateKeyRequest, _ ...grpc.CallOption) (*aiproviderv1.RotateKeyResponse, error) {
	f.lastCtx = ctx
	f.lastRotateKeyReq = in
	if f.rotateKeyErr != nil {
		return nil, f.rotateKeyErr
	}
	return f.rotateKeyResp, nil
}

func (f *fakeAIProviderServiceClient) GetUsageToday(ctx context.Context, in *aiproviderv1.GetUsageTodayRequest, _ ...grpc.CallOption) (*aiproviderv1.GetUsageTodayResponse, error) {
	f.lastCtx = ctx
	f.lastGetUsageTodayReq = in
	if f.getUsageTodayErr != nil {
		return nil, f.getUsageTodayErr
	}
	return f.getUsageTodayResp, nil
}

// ListAccounts/UpdateAccount/DeleteAccount/WriteCredential/TestConnection:
// none of this file's tests exercise these — same unconditional
// Unimplemented-stub convention as infra_routes_test.go's terminal RPCs —
// they exist only to satisfy aiproviderv1.AiProviderServiceClient in full.
func (f *fakeAIProviderServiceClient) ListAccounts(context.Context, *aiproviderv1.ListAccountsRequest, ...grpc.CallOption) (*aiproviderv1.ListAccountsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by ai_provider_routes_test.go")
}

func (f *fakeAIProviderServiceClient) UpdateAccount(context.Context, *aiproviderv1.UpdateAccountRequest, ...grpc.CallOption) (*aiproviderv1.UpdateAccountResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by ai_provider_routes_test.go")
}

func (f *fakeAIProviderServiceClient) DeleteAccount(context.Context, *aiproviderv1.DeleteAccountRequest, ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "not used by ai_provider_routes_test.go")
}

func (f *fakeAIProviderServiceClient) WriteCredential(context.Context, *aiproviderv1.WriteCredentialRequest, ...grpc.CallOption) (*aiproviderv1.WriteCredentialResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by ai_provider_routes_test.go")
}

func (f *fakeAIProviderServiceClient) TestConnection(context.Context, *aiproviderv1.TestConnectionRequest, ...grpc.CallOption) (*aiproviderv1.TestConnectionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by ai_provider_routes_test.go")
}

// RecordTokenUsage is service-to-service only (TASK-AIP-03-08) — never
// routed through api-gateway, so this fake never expects a call either.
func (f *fakeAIProviderServiceClient) RecordTokenUsage(context.Context, *aiproviderv1.RecordTokenUsageRequest, ...grpc.CallOption) (*aiproviderv1.RecordTokenUsageResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by ai_provider_routes_test.go")
}

// testAIProviderRouter mounts mountAIProviderRoutes alone on a fresh chi
// router — no auth middleware — since these tests inject identity directly
// into the request context, the same way authMiddleware would have (see
// requestWithIdentity in automation_routes_test.go).
func testAIProviderRouter(client aiproviderv1.AiProviderServiceClient) http.Handler {
	r := chi.NewRouter()
	mountAIProviderRoutes(r, client)
	return r
}

func TestHandleCreateAccount_SuccessRoundTrip(t *testing.T) {
	client := &fakeAIProviderServiceClient{
		createAccountResp: &aiproviderv1.CreateAccountResponse{
			Account: &aiproviderv1.ProviderAccount{
				Id:            "acct-1",
				TenantId:      "tenant-1",
				Type:          aiproviderv1.ProviderType_PROVIDER_TYPE_ANTHROPIC,
				Status:        "active",
				CredentialRef: "cred-ref-1",
			},
		},
	}
	router := testAIProviderRouter(client)

	body, _ := json.Marshal(createAccountRequestBody{Type: "anthropic"})
	req := requestWithIdentity(http.MethodPost, "/v1/ai-providers/accounts", body, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var got aiproviderv1.ProviderAccount
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v; body=%s", err, rec.Body.String())
	}
	if got.Id != "acct-1" || got.CredentialRef != "cred-ref-1" {
		t.Fatalf("unexpected response: id=%q credentialRef=%q", got.Id, got.CredentialRef)
	}

	if client.lastCreateAccountReq == nil {
		t.Fatal("expected CreateAccount to be called")
	}
	if client.lastCreateAccountReq.TenantId != "tenant-1" {
		t.Fatalf("TenantId = %q, want %q (must come from identity, not body)", client.lastCreateAccountReq.TenantId, "tenant-1")
	}
	if client.lastCreateAccountReq.Type != aiproviderv1.ProviderType_PROVIDER_TYPE_ANTHROPIC {
		t.Fatalf("Type = %v, want PROVIDER_TYPE_ANTHROPIC", client.lastCreateAccountReq.Type)
	}
}

func TestHandleCreateAccount_TenantIDComesFromIdentityNotBody(t *testing.T) {
	client := &fakeAIProviderServiceClient{
		createAccountResp: &aiproviderv1.CreateAccountResponse{
			Account: &aiproviderv1.ProviderAccount{Id: "acct-2"},
		},
	}
	router := testAIProviderRouter(client)

	// The request body carries no tenant_id field at all (the wire type
	// intentionally has none) — but craft a raw JSON body with an attempted
	// tenant_id key to prove it's ignored even if a client tries to smuggle
	// one in.
	rawBody := []byte(`{"type":"anthropic","tenant_id":"attacker-tenant"}`)
	req := requestWithIdentity(http.MethodPost, "/v1/ai-providers/accounts", rawBody, usecase.Identity{TenantID: "real-tenant", UserID: "user-1"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if client.lastCreateAccountReq == nil {
		t.Fatal("expected CreateAccount to be called")
	}
	if client.lastCreateAccountReq.TenantId != "real-tenant" {
		t.Fatalf("TenantId = %q, want %q (must come from identity, not body)", client.lastCreateAccountReq.TenantId, "real-tenant")
	}
}

// TestHandleResolveProvider_ResponseNeverLeaksSecretBeyondProto is the
// SECURITY-CRITICAL check: ResolveProvider's REST response must expose only
// what ResolveProviderResponse's proto message itself carries (a
// credential_ref, never a plaintext secret) — this asserts the JSON
// response's field set is exactly ProviderAccount's fields, nothing added.
func TestHandleResolveProvider_ResponseNeverLeaksSecretBeyondProto(t *testing.T) {
	client := &fakeAIProviderServiceClient{
		resolveProviderResp: &aiproviderv1.ResolveProviderResponse{
			Account: &aiproviderv1.ProviderAccount{
				Id:            "acct-3",
				TenantId:      "tenant-1",
				Type:          aiproviderv1.ProviderType_PROVIDER_TYPE_OPENAI,
				Status:        "active",
				CredentialRef: "cred-ref-3",
			},
		},
	}
	router := testAIProviderRouter(client)

	req := requestWithIdentity(http.MethodGet, "/v1/ai-providers/resolve?project_id=proj-1", nil, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal response: %v; body=%s", err, rec.Body.String())
	}

	// ProviderAccount's json tags per aiprovider.pb.go: id, tenant_id, type,
	// status, credential_ref. Anything outside this set (e.g. a raw "key",
	// "secret", "api_key", "credential" field) would be a leak.
	allowed := map[string]bool{
		"id": true, "tenant_id": true, "type": true, "status": true, "credential_ref": true,
	}
	for k, v := range raw {
		if !allowed[k] {
			t.Fatalf("response has unexpected field %q (possible credential leak): %v; full body=%s", k, v, rec.Body.String())
		}
		lower := strings.ToLower(k)
		if lower != "credential_ref" && (strings.Contains(lower, "secret") || strings.Contains(lower, "key") || strings.Contains(lower, "password")) {
			t.Fatalf("response field %q looks like it could carry raw credential material: %v", k, v)
		}
	}
	if raw["credential_ref"] != "cred-ref-3" {
		t.Fatalf("credential_ref = %v, want %q", raw["credential_ref"], "cred-ref-3")
	}

	if client.lastResolveProviderReq == nil {
		t.Fatal("expected ResolveProvider to be called")
	}
	if client.lastResolveProviderReq.TenantId != "tenant-1" || client.lastResolveProviderReq.UserId != "user-1" || client.lastResolveProviderReq.ProjectId != "proj-1" {
		t.Fatalf("unexpected ResolveProvider request: %+v", client.lastResolveProviderReq)
	}
}

func TestHandleRotateKey_GRPCErrorMapsToHTTPStatus(t *testing.T) {
	client := &fakeAIProviderServiceClient{
		rotateKeyErr: status.Error(codes.NotFound, "account not found"),
	}
	router := testAIProviderRouter(client)

	req := requestWithIdentity(http.MethodPost, "/v1/ai-providers/accounts/acct-404/rotate-key", nil, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}

	var body errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal error body: %v; body=%s", err, rec.Body.String())
	}
	if body.Error.Code != codes.NotFound.String() {
		t.Fatalf("error.code = %q, want %q", body.Error.Code, codes.NotFound.String())
	}
	if body.Error.Message != "account not found" {
		t.Fatalf("error.message = %q, want %q", body.Error.Message, "account not found")
	}

	if client.lastRotateKeyReq == nil || client.lastRotateKeyReq.AccountId != "acct-404" {
		t.Fatalf("expected RotateKey called with account id acct-404, got %+v", client.lastRotateKeyReq)
	}
}

func TestHandleGetUsageToday_SuccessRoundTrip(t *testing.T) {
	client := &fakeAIProviderServiceClient{
		getUsageTodayResp: &aiproviderv1.GetUsageTodayResponse{
			CostUsd:      1.23,
			RequestCount: 42,
		},
	}
	router := testAIProviderRouter(client)

	req := requestWithIdentity(http.MethodGet, "/v1/ai-providers/usage-today?account_id=acct-1", nil, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got aiproviderv1.GetUsageTodayResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v; body=%s", err, rec.Body.String())
	}
	if got.CostUsd != 1.23 || got.RequestCount != 42 {
		t.Fatalf("unexpected response: costUsd=%v requestCount=%v", got.CostUsd, got.RequestCount)
	}

	if client.lastGetUsageTodayReq == nil || client.lastGetUsageTodayReq.AccountId != "acct-1" {
		t.Fatalf("expected GetUsageToday called with account id acct-1, got %+v", client.lastGetUsageTodayReq)
	}
}
