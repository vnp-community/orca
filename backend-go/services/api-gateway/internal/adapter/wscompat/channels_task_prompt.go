package wscompat

import (
	"context"
	"encoding/json"

	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
)

// registerTaskPromptChannels exposes "Generate Agent Prompt": the AI writes a
// ready-to-run prompt for one task. save=false previews it; save=true also
// stores it as the task's promptTemplate. task-service enforces the caller's
// grant (read to preview, write to save), so nothing is checked here.
func registerTaskPromptChannels(r *Registry, client taskv1.TaskServiceClient) {
	r.Register("task.generateAgentPrompt", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type generateArgs struct {
			TaskID string `json:"taskId"`
			Save   bool   `json:"save"`
		}
		in, err := decodeArg[generateArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.GenerateAgentPrompt(ctx, &taskv1.GenerateAgentPromptRequest{TaskId: in.TaskID, Save: in.Save})
		if err != nil {
			return nil, err
		}
		return struct {
			Prompt string `json:"prompt"`
		}{Prompt: resp.GetPrompt()}, nil
	})
}
