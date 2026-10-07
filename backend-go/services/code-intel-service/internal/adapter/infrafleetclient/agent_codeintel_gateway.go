package infrafleetclient

import (
	"github.com/stablyai/orca-go/services/code-intel-service/internal/usecase"
)

// NewAgentCodeIntelGateway creates an AgentCodeIntelGateway wrapping the provided AgentRPCCaller.
func NewAgentCodeIntelGateway(caller usecase.AgentRPCCaller) usecase.AgentCodeIntelGateway {
	return usecase.NewAgentCodeIntelGateway(caller)
}
