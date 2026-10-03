package wscompat

import (
	"context"
	"encoding/json"
	"testing"

	"google.golang.org/grpc"

	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
)

type fakeTaskPromptClient struct {
	taskv1.TaskServiceClient
	got *taskv1.GenerateAgentPromptRequest
}

func (f *fakeTaskPromptClient) GenerateAgentPrompt(_ context.Context, in *taskv1.GenerateAgentPromptRequest, _ ...grpc.CallOption) (*taskv1.GenerateAgentPromptResponse, error) {
	f.got = in
	return &taskv1.GenerateAgentPromptResponse{Prompt: "do the thing"}, nil
}

func TestTaskGenerateAgentPromptChannel_ForwardsArgsAndReturnsCamelCase(t *testing.T) {
	fake := &fakeTaskPromptClient{}
	r := NewRegistry()
	registerTaskPromptChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "u1"}, "task.generateAgentPrompt",
		argsJSON(t, map[string]any{"taskId": "t1", "save": true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.got.GetTaskId() != "t1" || !fake.got.GetSave() {
		t.Errorf("args not forwarded: %+v", fake.got)
	}
	raw, _ := json.Marshal(result)
	if string(raw) != `{"prompt":"do the thing"}` {
		t.Errorf("want {\"prompt\":...} on the wire, got %s", raw)
	}
}

func TestTaskGenerateAgentPromptChannel_DefaultsToPreview(t *testing.T) {
	fake := &fakeTaskPromptClient{}
	r := NewRegistry()
	registerTaskPromptChannels(r, fake)

	if _, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "task.generateAgentPrompt", argsJSON(t, map[string]any{"taskId": "t1"})); err != nil {
		t.Fatal(err)
	}
	if fake.got.GetSave() {
		t.Error("omitting save must never persist a prompt")
	}
}
