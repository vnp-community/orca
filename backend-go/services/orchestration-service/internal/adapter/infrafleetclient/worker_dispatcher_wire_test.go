package infrafleetclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/stablyai/orca-go/common/agentrelay"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/orchestration-service/internal/domain"
)

// wireGoldenDir holds the JSON agentrelay froze from the struct this service used to send.
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

// legacyDispatchParams is agentExecPromptParams as it stood before the move to agentrelay, tags unchanged.
type legacyDispatchParams struct {
	Prompt       string `json:"prompt"`
	WorktreePath string `json:"worktreePath"`
	StepID       string `json:"stepId,omitempty"`
}

// ctxRecordingClient keeps the context Relay was called with.
type ctxRecordingClient struct {
	fakeInfraFleetServiceClient
	ctx context.Context
}

func (c *ctxRecordingClient) Relay(ctx context.Context, in *infrafleetv1.RelayRequest, opts ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
	c.ctx = ctx
	return c.fakeInfraFleetServiceClient.Relay(ctx, in, opts...)
}

func dispatchTask() domain.OrchestrationTask {
	return domain.OrchestrationTask{ID: "t-9", TaskTitle: "Fix the login bug", Spec: []byte(`{"connectionId":"conn-1","worktreePath":"/srv/wt/t-9"}`)}
}

func TestWorkerDispatcherWire_SendsTheThreeFieldsItAlwaysSent(t *testing.T) {
	client := &ctxRecordingClient{fakeInfraFleetServiceClient: fakeInfraFleetServiceClient{
		relayResp: &infrafleetv1.RelayResponse{ResultJson: `{"stdout":"ok","exitCode":0,"timedOut":false}`},
	}}
	if err := NewWorkerDispatcher(client).Dispatch(context.Background(), "tenant-1", dispatchTask(), "handle-9"); err != nil {
		t.Fatal(err)
	}
	sent := []byte(client.gotRelay.GetParamsJson())

	golden, err := os.ReadFile(filepath.Join(wireGoldenDir, "orchestration.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := canonicalWire(t, sent); !bytes.Equal(got, golden) {
		t.Errorf("dispatch changed on the wire:\n got %s\nwant %s", got, golden)
	}
	old, _ := json.Marshal(legacyDispatchParams{Prompt: "Fix the login bug", WorktreePath: "/srv/wt/t-9", StepID: "handle-9"})
	if !bytes.Equal(canonicalWire(t, sent), canonicalWire(t, old)) {
		t.Errorf("differs from what the old struct sent:\n got %s\nold %s", sent, old)
	}

	if _, has := client.ctx.Deadline(); has {
		t.Error("a dispatch had no RPC deadline of ours before and must not get one now")
	}
	if md, ok := metadata.FromOutgoingContext(client.ctx); ok && len(md) != 0 {
		t.Errorf("this service sent no outgoing metadata of its own, got %v", md)
	}
}

func TestWorkerDispatcherWire_TheOutcomeIsReadFromTimedOutAndExitCode(t *testing.T) {
	cases := []struct {
		name, result, wantErr string
	}{
		{"exit 0", `{"exitCode":0}`, ""},
		{"output cut by the agent still succeeds", `{"exitCode":0,"truncated":{"stdout":true,"stderr":false}}`, ""},
		{"timed out", `{"exitCode":null,"timedOut":true}`, "timed out"},
		{"non-zero exit carries stderr", `{"stderr":"boom","exitCode":3}`, "boom"},
		{"no exit code and no timeout", `{"stdout":"x"}`, "exited non-zero"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeInfraFleetServiceClient{relayResp: &infrafleetv1.RelayResponse{ResultJson: tc.result}}
			err := NewWorkerDispatcher(fake).Dispatch(context.Background(), "tenant-1", dispatchTask(), "h")
			switch {
			case tc.wantErr == "" && err != nil:
				t.Fatalf("unexpected error: %v", err)
			case tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)):
				t.Fatalf("err = %v, want it to mention %q", err, tc.wantErr)
			}
		})
	}
}

func TestWorkerDispatcherWire_RelayAndDecodeFailuresAreErrors(t *testing.T) {
	fake := &fakeInfraFleetServiceClient{relayErr: errors.New("dev server unreachable")}
	if err := NewWorkerDispatcher(fake).Dispatch(context.Background(), "t", dispatchTask(), "h"); err == nil || !strings.Contains(err.Error(), "dev server unreachable") {
		t.Fatalf("relay failure: %v", err)
	}
	fake = &fakeInfraFleetServiceClient{relayResp: &infrafleetv1.RelayResponse{ResultJson: `not json`}}
	if err := NewWorkerDispatcher(fake).Dispatch(context.Background(), "t", dispatchTask(), "h"); !errors.Is(err, agentrelay.ErrMalformedResult) {
		t.Fatalf("decode failure: %v", err)
	}
}
