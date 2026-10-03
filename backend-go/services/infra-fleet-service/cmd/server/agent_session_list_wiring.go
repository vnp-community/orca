package main

import (
	infragrpc "github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// withAgentSessionList adds ListAgentSessions when the agent-session store can
// list by origin (both the postgres and mysql stores can); otherwise the RPC
// stays Unimplemented and the gateway falls back to in-process reaping.
func withAgentSessionList(base infrafleetv1.InfraFleetServiceServer, store usecase.AgentSessionRepository) infrafleetv1.InfraFleetServiceServer {
	lister, ok := store.(usecase.AgentSessionOriginLister)
	if !ok {
		return base
	}
	return infragrpc.WithAgentSessionList(base, usecase.NewListAgentSessionsByOrigin(lister))
}
