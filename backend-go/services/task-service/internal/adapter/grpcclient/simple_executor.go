package grpcclient

import (
	"context"
	"fmt"
)

// StubSimpleExecutor implements usecase.SimpleExecutor as a stub that
// returns a synthesized execution reference without calling
// infra-fleet-service. Real wiring needs: an
// infrafleetv1.InfraFleetServiceClient (gRPC) dialed to
// infra-fleet-service, calling its relay-to-Dev-Server-Agent `agent.exec`
// path (task-service.md §3.1) and returning infra-fleet-service's own
// execution/run ID as the ref task-service stores in
// active_execution_id-equivalent bookkeeping (not persisted in this
// scaffold — see this service's README).
type StubSimpleExecutor struct{}

func NewStubSimpleExecutor() *StubSimpleExecutor {
	return &StubSimpleExecutor{}
}

func (s *StubSimpleExecutor) Execute(ctx context.Context, tenantID, taskID, requestID string) (string, error) {
	return fmt.Sprintf("stub-infra-fleet-exec:%s:%s", taskID, requestID), nil
}
