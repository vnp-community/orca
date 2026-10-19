package grpcclient

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/common/grpcmw"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/usecase"
)

// wireGoldenDir holds the JSON agentrelay froze from the params map this method used to send.
var wireGoldenDir = filepath.Join("..", "..", "..", "..", "..", "common", "agentrelay", "testdata", "wire")

func canonicalWire(t *testing.T, raw []byte) []byte {
	t.Helper()
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("unmarshal %s: %v", raw, err)
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(out, '\n')
}

// ctxRecordingClient keeps the context of the last Relay and RelayByDevServer call.
type ctxRecordingClient struct {
	fakeInfraFleetServiceClient
	ctx context.Context
}

func (c *ctxRecordingClient) Relay(ctx context.Context, in *infrafleetv1.RelayRequest, opts ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
	c.ctx = ctx
	return c.fakeInfraFleetServiceClient.Relay(ctx, in, opts...)
}

func (c *ctxRecordingClient) RelayByDevServer(ctx context.Context, in *infrafleetv1.RelayByDevServerRequest, opts ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
	c.ctx = ctx
	return c.fakeInfraFleetServiceClient.RelayByDevServer(ctx, in, opts...)
}

func completeReply(json string) *infrafleetv1.RelayResponse {
	return &infrafleetv1.RelayResponse{ResultJson: json}
}

func TestRelayExecutor_Complete_SendsOnlyThePromptAndReturnsTheContent(t *testing.T) {
	client := &ctxRecordingClient{fakeInfraFleetServiceClient: fakeInfraFleetServiceClient{
		relayResp: completeReply(`{"content":"feat: add x","model":"claude","provider":"anthropic","latencyMs":812,"usage":{"promptTokens":9,"completionTokens":4,"totalTokens":13}}`),
	}}
	got, err := NewRelayExecutor(client).Complete(ctxWithTenant(t), "conn-1", "Write a commit message")
	if err != nil || got != "feat: add x" {
		t.Fatalf("got %q err %v", got, err)
	}
	if client.gotRelay.GetMethod() != "ai.complete" || client.gotRelay.GetConnectionId() != "conn-1" {
		t.Fatalf("relay = %+v", client.gotRelay)
	}
	want, err := os.ReadFile(filepath.Join(wireGoldenDir, "complete_git.json"))
	if err != nil {
		t.Fatal(err)
	}
	if sent := canonicalWire(t, []byte(client.gotRelay.GetParamsJson())); !bytes.Equal(sent, want) {
		t.Errorf("ai.complete params changed:\n got %s\nwant %s", sent, want)
	}
	if _, has := client.ctx.Deadline(); has {
		t.Error("ai.complete had no deadline of ours before and must not get one now")
	}
	md, _ := metadata.FromOutgoingContext(client.ctx)
	if tenantIDs := md.Get(grpcmw.MetadataTenantID); len(tenantIDs) != 1 || tenantIDs[0] != "tenant-1" {
		t.Errorf("tenant metadata = %v", tenantIDs)
	}
}

func TestRelayExecutor_Complete_NoTenantFailsBeforeAnyCall(t *testing.T) {
	fake := &fakeInfraFleetServiceClient{relayResp: completeReply(`{"content":"x"}`)}
	if _, err := NewRelayExecutor(fake).Complete(context.Background(), "conn-1", "p"); err == nil {
		t.Fatal("expected an error without a tenant")
	}
	if fake.gotRelay != nil {
		t.Error("Relay must not be called without a tenant")
	}
}

// A repo whose dev server has no connection row is reached by the dev server id, as relay() does for git and fs calls;
// the hidden-target rewrite stays out of ai.complete.
func TestRelayExecutor_Complete_UsesTheDevServerFromContext(t *testing.T) {
	fake := &fakeInfraFleetServiceClient{relayByDevServerResp: completeReply(`{"content":"ok"}`)}
	ctx := usecase.WithHiddenTargetID(usecase.WithDevServerID(ctxWithTenant(t), "ds-1"), "ht-1")
	if got, err := NewRelayExecutor(fake).Complete(ctx, "conn-ignored", "p"); err != nil || got != "ok" {
		t.Fatalf("got %q err %v", got, err)
	}
	if fake.gotRelay != nil || fake.gotRelayByDevServer.GetDevServerId() != "ds-1" {
		t.Fatalf("relay=%v byDev=%v", fake.gotRelay, fake.gotRelayByDevServer)
	}
	if fake.gotRelayByDevServer.GetMethod() != "ai.complete" || strings.Contains(fake.gotRelayByDevServer.GetParamsJson(), "hiddenTargetId") {
		t.Errorf("ai.complete is not a per-repo method: %+v", fake.gotRelayByDevServer)
	}
}

func TestRelayExecutor_Complete_DevServerDroppedIsRetryableUnreachable(t *testing.T) {
	fake := &fakeInfraFleetServiceClient{
		relayByDevServerErr: status.Error(codes.FailedPrecondition, "INFRA_DEV_SERVER_NOT_CONNECTED: no live agent"),
	}
	_, err := NewRelayExecutor(fake).Complete(usecase.WithDevServerID(ctxWithTenant(t), "ds-1"), "c", "p")
	if ae := usecase.HostDispatchError(err); ae == nil || ae.Code != usecase.CodeDevServerUnreachable {
		t.Fatalf("expected %s, got %v", usecase.CodeDevServerUnreachable, err)
	}
}

// An agent refusal arrives as a normal relay answer; read as a completion it would be an empty message.
func TestRelayExecutor_Complete_AgentErrorEnvelopeIsAnError(t *testing.T) {
	for name, fake := range map[string]*fakeInfraFleetServiceClient{
		"by connection": {relayResp: completeReply(`{"error":{"code":-32000,"message":"no provider configured"}}`)},
		"by dev server": {relayByDevServerResp: completeReply(`{"error":{"code":-32000,"message":"no provider configured"}}`)},
	} {
		t.Run(name, func(t *testing.T) {
			ctx := ctxWithTenant(t)
			if name == "by dev server" {
				ctx = usecase.WithDevServerID(ctx, "ds-1")
			}
			got, err := NewRelayExecutor(fake).Complete(ctx, "c", "p")
			if err == nil || got != "" || !strings.Contains(err.Error(), "no provider configured") || !strings.Contains(err.Error(), "-32000") {
				t.Fatalf("got %q err %v", got, err)
			}
		})
	}
}

func TestRelayExecutor_Complete_FailuresAreErrors(t *testing.T) {
	cases := map[string]*fakeInfraFleetServiceClient{
		"transport":      {relayErr: status.Error(codes.Unavailable, "gone")},
		"typed refusal":  {relayErr: status.Error(codes.FailedPrecondition, "AI_COMPLETE_NO_ACCOUNT: add a provider")},
		"unreadable":     {relayResp: completeReply(`not json`)},
		"no answer text": {relayResp: completeReply(``)},
	}
	for name, fake := range cases {
		t.Run(name, func(t *testing.T) {
			if got, err := NewRelayExecutor(fake).Complete(ctxWithTenant(t), "c", "p"); err == nil || got != "" {
				t.Fatalf("got %q err %v", got, err)
			}
		})
	}
}
