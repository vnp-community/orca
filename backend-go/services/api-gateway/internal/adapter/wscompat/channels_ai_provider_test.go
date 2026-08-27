package wscompat

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	aiproviderv1 "github.com/stablyai/orca-go/proto/gen/go/orca/aiprovider/v1"
)

// fakeAiProviderClient embeds the generated interface so only the methods a
// given test actually exercises need a func field — same convention as
// fakeInfraFleetClient (channels_test.go) and fakeIssueTrackingClient
// (channels_jira_test.go).
type fakeAiProviderClient struct {
	aiproviderv1.AiProviderServiceClient

	createAccountFunc   func(ctx context.Context, in *aiproviderv1.CreateAccountRequest) (*aiproviderv1.CreateAccountResponse, error)
	listAccountsFunc    func(ctx context.Context, in *aiproviderv1.ListAccountsRequest) (*aiproviderv1.ListAccountsResponse, error)
	updateAccountFunc   func(ctx context.Context, in *aiproviderv1.UpdateAccountRequest) (*aiproviderv1.UpdateAccountResponse, error)
	deleteAccountFunc   func(ctx context.Context, in *aiproviderv1.DeleteAccountRequest) (*emptypb.Empty, error)
	writeCredentialFunc func(ctx context.Context, in *aiproviderv1.WriteCredentialRequest) (*aiproviderv1.WriteCredentialResponse, error)
	testConnectionFunc  func(ctx context.Context, in *aiproviderv1.TestConnectionRequest) (*aiproviderv1.TestConnectionResponse, error)
}

func (f *fakeAiProviderClient) CreateAccount(ctx context.Context, in *aiproviderv1.CreateAccountRequest, _ ...grpc.CallOption) (*aiproviderv1.CreateAccountResponse, error) {
	return f.createAccountFunc(ctx, in)
}

func (f *fakeAiProviderClient) ListAccounts(ctx context.Context, in *aiproviderv1.ListAccountsRequest, _ ...grpc.CallOption) (*aiproviderv1.ListAccountsResponse, error) {
	return f.listAccountsFunc(ctx, in)
}

func (f *fakeAiProviderClient) UpdateAccount(ctx context.Context, in *aiproviderv1.UpdateAccountRequest, _ ...grpc.CallOption) (*aiproviderv1.UpdateAccountResponse, error) {
	return f.updateAccountFunc(ctx, in)
}

func (f *fakeAiProviderClient) DeleteAccount(ctx context.Context, in *aiproviderv1.DeleteAccountRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	return f.deleteAccountFunc(ctx, in)
}

func (f *fakeAiProviderClient) WriteCredential(ctx context.Context, in *aiproviderv1.WriteCredentialRequest, _ ...grpc.CallOption) (*aiproviderv1.WriteCredentialResponse, error) {
	return f.writeCredentialFunc(ctx, in)
}

func (f *fakeAiProviderClient) TestConnection(ctx context.Context, in *aiproviderv1.TestConnectionRequest, _ ...grpc.CallOption) (*aiproviderv1.TestConnectionResponse, error) {
	return f.testConnectionFunc(ctx, in)
}

func mustArgs(t *testing.T, v any) []json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	return []json.RawMessage{b}
}

func TestAiProviderCreateChannel_Success(t *testing.T) {
	var gotCtx context.Context
	var gotReq *aiproviderv1.CreateAccountRequest
	client := &fakeAiProviderClient{
		createAccountFunc: func(ctx context.Context, in *aiproviderv1.CreateAccountRequest) (*aiproviderv1.CreateAccountResponse, error) {
			gotCtx, gotReq = ctx, in
			return &aiproviderv1.CreateAccountResponse{
				Account: &aiproviderv1.ProviderAccount{Id: "acct-1", TenantId: "t1", Type: aiproviderv1.ProviderType_PROVIDER_TYPE_ANTHROPIC},
			}, nil
		},
	}
	r := NewRegistry()
	registerAiProviderChannels(r, client)

	args := mustArgs(t, map[string]any{"type": "PROVIDER_TYPE_ANTHROPIC"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "aiProvider.create", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	account, ok := result.(*aiproviderv1.ProviderAccount)
	if !ok || account.GetId() != "acct-1" {
		t.Fatalf("result = %#v, want ProviderAccount{Id: acct-1}", result)
	}
	if gotReq.GetTenantId() != "t1" {
		t.Errorf("TenantId = %q, want t1", gotReq.GetTenantId())
	}
	if gotReq.GetType() != aiproviderv1.ProviderType_PROVIDER_TYPE_ANTHROPIC {
		t.Errorf("Type = %v, want PROVIDER_TYPE_ANTHROPIC", gotReq.GetType())
	}
	tenant, user := outgoingTenantUser(gotCtx)
	if tenant != "t1" || user != "u1" {
		t.Errorf("AttachIdentity metadata = (%q,%q), want (t1,u1)", tenant, user)
	}
}

func TestAiProviderCreateChannel_PropagatesError(t *testing.T) {
	client := &fakeAiProviderClient{
		createAccountFunc: func(context.Context, *aiproviderv1.CreateAccountRequest) (*aiproviderv1.CreateAccountResponse, error) {
			return nil, errors.New("boom")
		},
	}
	r := NewRegistry()
	registerAiProviderChannels(r, client)

	args := mustArgs(t, map[string]any{"type": "PROVIDER_TYPE_OPENAI"})
	if _, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "aiProvider.create", args); err == nil {
		t.Fatal("expected error to propagate")
	}
}

func TestAiProviderListChannel_Success(t *testing.T) {
	var gotReq *aiproviderv1.ListAccountsRequest
	client := &fakeAiProviderClient{
		listAccountsFunc: func(ctx context.Context, in *aiproviderv1.ListAccountsRequest) (*aiproviderv1.ListAccountsResponse, error) {
			gotReq = in
			return &aiproviderv1.ListAccountsResponse{Accounts: []*aiproviderv1.ProviderAccount{{Id: "a1"}, {Id: "a2"}}}, nil
		},
	}
	r := NewRegistry()
	registerAiProviderChannels(r, client)

	args := mustArgs(t, map[string]any{"devServerId": "ds-1"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "aiProvider.list", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	accounts, ok := result.([]*aiproviderv1.ProviderAccount)
	if !ok || len(accounts) != 2 {
		t.Fatalf("result = %#v, want 2 accounts", result)
	}
	if gotReq.GetDevServerId() != "ds-1" {
		t.Errorf("DevServerId = %q, want ds-1", gotReq.GetDevServerId())
	}
}

func TestAiProviderListChannel_PropagatesError(t *testing.T) {
	client := &fakeAiProviderClient{
		listAccountsFunc: func(context.Context, *aiproviderv1.ListAccountsRequest) (*aiproviderv1.ListAccountsResponse, error) {
			return nil, errors.New("unavailable")
		},
	}
	r := NewRegistry()
	registerAiProviderChannels(r, client)

	if _, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "aiProvider.list", nil); err == nil {
		t.Fatal("expected error to propagate")
	}
}

func TestAiProviderUpdateChannel_Success(t *testing.T) {
	var gotCtx context.Context
	var gotReq *aiproviderv1.UpdateAccountRequest
	client := &fakeAiProviderClient{
		updateAccountFunc: func(ctx context.Context, in *aiproviderv1.UpdateAccountRequest) (*aiproviderv1.UpdateAccountResponse, error) {
			gotCtx, gotReq = ctx, in
			return &aiproviderv1.UpdateAccountResponse{Account: &aiproviderv1.ProviderAccount{Id: in.GetAccountId()}}, nil
		},
	}
	r := NewRegistry()
	registerAiProviderChannels(r, client)

	args := mustArgs(t, map[string]any{
		"accountId": "acct-1", "label": "My Key", "modelHint": "claude-opus", "baseUrl": "http://localhost:11434",
	})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "aiProvider.update", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	account, ok := result.(*aiproviderv1.ProviderAccount)
	if !ok || account.GetId() != "acct-1" {
		t.Fatalf("result = %#v, want ProviderAccount{Id: acct-1}", result)
	}
	if gotReq.GetLabel() != "My Key" || gotReq.GetModelHint() != "claude-opus" || gotReq.GetBaseUrl() != "http://localhost:11434" {
		t.Errorf("req fields = %#v, want label/modelHint/baseUrl mapped 1:1", gotReq)
	}
	tenant, user := outgoingTenantUser(gotCtx)
	if tenant != "t1" || user != "u1" {
		t.Errorf("AttachIdentity metadata = (%q,%q), want (t1,u1)", tenant, user)
	}
}

func TestAiProviderUpdateChannel_PropagatesError(t *testing.T) {
	client := &fakeAiProviderClient{
		updateAccountFunc: func(context.Context, *aiproviderv1.UpdateAccountRequest) (*aiproviderv1.UpdateAccountResponse, error) {
			return nil, errors.New("not found")
		},
	}
	r := NewRegistry()
	registerAiProviderChannels(r, client)

	args := mustArgs(t, map[string]any{"accountId": "acct-missing"})
	if _, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "aiProvider.update", args); err == nil {
		t.Fatal("expected error to propagate")
	}
}

func TestAiProviderDeleteChannel_Success(t *testing.T) {
	var gotReq *aiproviderv1.DeleteAccountRequest
	client := &fakeAiProviderClient{
		deleteAccountFunc: func(ctx context.Context, in *aiproviderv1.DeleteAccountRequest) (*emptypb.Empty, error) {
			gotReq = in
			return &emptypb.Empty{}, nil
		},
	}
	r := NewRegistry()
	registerAiProviderChannels(r, client)

	args := mustArgs(t, map[string]any{"accountId": "acct-1"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "aiProvider.delete", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ok, _ := result.(map[string]bool)
	if !ok["ok"] {
		t.Fatalf("result = %#v, want {ok: true}", result)
	}
	if gotReq.GetAccountId() != "acct-1" {
		t.Errorf("AccountId = %q, want acct-1", gotReq.GetAccountId())
	}
}

func TestAiProviderDeleteChannel_PropagatesError(t *testing.T) {
	client := &fakeAiProviderClient{
		deleteAccountFunc: func(context.Context, *aiproviderv1.DeleteAccountRequest) (*emptypb.Empty, error) {
			return nil, errors.New("denied")
		},
	}
	r := NewRegistry()
	registerAiProviderChannels(r, client)

	args := mustArgs(t, map[string]any{"accountId": "acct-1"})
	if _, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "aiProvider.delete", args); err == nil {
		t.Fatal("expected error to propagate")
	}
}

func TestAiProviderWriteCredentialChannel_Success(t *testing.T) {
	var gotReq *aiproviderv1.WriteCredentialRequest
	client := &fakeAiProviderClient{
		writeCredentialFunc: func(ctx context.Context, in *aiproviderv1.WriteCredentialRequest) (*aiproviderv1.WriteCredentialResponse, error) {
			gotReq = in
			return &aiproviderv1.WriteCredentialResponse{Account: &aiproviderv1.ProviderAccount{Id: in.GetAccountId()}}, nil
		},
	}
	r := NewRegistry()
	registerAiProviderChannels(r, client)

	// "aGVsbG8=" == base64("hello"), "d29ybGQ=" == base64("world")
	args := mustArgs(t, map[string]any{"accountId": "acct-1", "encryptedBlob": "aGVsbG8=", "iv": "d29ybGQ="})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "aiProvider.writeCredential", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	account, ok := result.(*aiproviderv1.ProviderAccount)
	if !ok || account.GetId() != "acct-1" {
		t.Fatalf("result = %#v, want ProviderAccount{Id: acct-1}", result)
	}
	if string(gotReq.GetEncryptedBlob()) != "hello" {
		t.Errorf("EncryptedBlob = %q, want decoded 'hello'", gotReq.GetEncryptedBlob())
	}
	if string(gotReq.GetIv()) != "world" {
		t.Errorf("Iv = %q, want decoded 'world'", gotReq.GetIv())
	}
}

func TestAiProviderWriteCredentialChannel_InvalidBase64_DoesNotCallRPC(t *testing.T) {
	called := false
	client := &fakeAiProviderClient{
		writeCredentialFunc: func(context.Context, *aiproviderv1.WriteCredentialRequest) (*aiproviderv1.WriteCredentialResponse, error) {
			called = true
			return nil, nil
		},
	}
	r := NewRegistry()
	registerAiProviderChannels(r, client)

	args := mustArgs(t, map[string]any{"accountId": "acct-1", "encryptedBlob": "not-valid-base64!!", "iv": "d29ybGQ="})
	if _, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "aiProvider.writeCredential", args); err == nil {
		t.Fatal("expected a base64 decode error")
	}
	if called {
		t.Error("WriteCredential RPC must not be called when encryptedBlob fails to decode")
	}
}

func TestAiProviderWriteCredentialChannel_PropagatesError(t *testing.T) {
	client := &fakeAiProviderClient{
		writeCredentialFunc: func(context.Context, *aiproviderv1.WriteCredentialRequest) (*aiproviderv1.WriteCredentialResponse, error) {
			return nil, errors.New("broker unavailable")
		},
	}
	r := NewRegistry()
	registerAiProviderChannels(r, client)

	args := mustArgs(t, map[string]any{"accountId": "acct-1", "encryptedBlob": "aGVsbG8=", "iv": "d29ybGQ="})
	if _, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "aiProvider.writeCredential", args); err == nil {
		t.Fatal("expected error to propagate")
	}
}

func TestAiProviderTestConnectionChannel_Success(t *testing.T) {
	var gotReq *aiproviderv1.TestConnectionRequest
	client := &fakeAiProviderClient{
		testConnectionFunc: func(ctx context.Context, in *aiproviderv1.TestConnectionRequest) (*aiproviderv1.TestConnectionResponse, error) {
			gotReq = in
			return &aiproviderv1.TestConnectionResponse{Success: true, Message: "ok"}, nil
		},
	}
	r := NewRegistry()
	registerAiProviderChannels(r, client)

	args := mustArgs(t, map[string]any{"accountId": "acct-1", "traceId": "trace-1"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "aiProvider.testConnection", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok || m["success"] != true || m["message"] != "ok" {
		t.Fatalf("result = %#v, want {success: true, message: ok}", result)
	}
	if gotReq.GetAccountId() != "acct-1" || gotReq.GetTraceId() != "trace-1" {
		t.Errorf("req = %#v, want AccountId=acct-1 TraceId=trace-1", gotReq)
	}
}

func TestAiProviderTestConnectionChannel_PropagatesError(t *testing.T) {
	client := &fakeAiProviderClient{
		testConnectionFunc: func(context.Context, *aiproviderv1.TestConnectionRequest) (*aiproviderv1.TestConnectionResponse, error) {
			return nil, errors.New("agent unreachable")
		},
	}
	r := NewRegistry()
	registerAiProviderChannels(r, client)

	args := mustArgs(t, map[string]any{"accountId": "acct-1"})
	if _, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "aiProvider.testConnection", args); err == nil {
		t.Fatal("expected error to propagate")
	}
}
