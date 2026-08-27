package wscompat

import (
	"context"
	"encoding/json"

	orchestrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/orchestration/v1"
)

// dispatchView is the wire shape orchestration.dispatchShow returns —
// assignee_handle here is DispatchContext.handle under the name
// terminal-orchestration-task-links.ts actually reads. SOL-018 resolves
// this as a wire-naming gap, not a missing field — the translation
// happens here at the wscompat boundary rather than as a proto rename
// (DispatchContext.handle is used by 3 existing RPCs + 2 REST handlers).
type dispatchView struct {
	ID                  string `json:"id"`
	OrchestrationTaskID string `json:"orchestration_task_id"`
	AssigneeHandle      string `json:"assignee_handle"`
	Status              string `json:"status"`
}

func registerOrchestrationChannels(r *Registry, client orchestrationv1.OrchestrationServiceClient) {
	r.Register("orchestration.dispatchShow", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type dispatchShowArgs struct {
			Task string `json:"task"`
		}
		in, err := decodeArg[dispatchShowArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = attachIdentity(ctx, id)
		rpcCtx, cancel := context.WithTimeout(ctx, groupRPCTimeout)
		defer cancel()
		resp, err := client.GetDispatchContextForTask(rpcCtx, &orchestrationv1.GetDispatchContextForTaskRequest{
			OrchestrationTaskId: in.Task,
		})
		if err != nil {
			return nil, err
		}
		dc := resp.GetDispatch()
		if dc == nil {
			// No dispatch yet — matches focusRuntimeOrchestrationTask's own
			// null-safe `result.dispatch?.assignee_handle` read and its
			// client-side "No dispatched terminal for orchestration task"
			// error when absent.
			return map[string]any{"dispatch": nil}, nil
		}
		return map[string]any{"dispatch": dispatchView{
			ID:                  dc.GetId(),
			OrchestrationTaskID: dc.GetOrchestrationTaskId(),
			AssigneeHandle:      dc.GetHandle(),
			Status:              "", // DispatchContext proto has no status field yet — adjacent gap, out of BUG-018's scope; not fixed here
		}}, nil
	})
}
