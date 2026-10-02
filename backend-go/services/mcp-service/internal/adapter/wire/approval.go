// Package wire maps domain values to the McpService protobuf messages used by
// more than one adapter (gRPC server and event hub).
package wire

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"

	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

// Approval maps an approval; status is the effective one (expiry applied).
func Approval(a domain.Approval, now time.Time) *mcpv1.Approval {
	out := &mcpv1.Approval{
		Id: a.ID, CreatedAt: timestamppb.New(a.CreatedAt), ExpiresAt: timestamppb.New(a.ExpiresAt), Status: a.EffectiveStatus(now),
		ToolName: a.ToolName, ToolTitle: a.ToolTitle, Risk: a.Risk, ClientName: a.ClientName, SessionId: a.SessionID,
		ArgsPreview: a.ArgsPreview, ArgsRedacted: a.ArgsRedacted, ParamsHash: a.ParamsHash, DecidedVia: a.DecidedVia,
		Reasons: a.Reasons, UserId: a.UserID,
	}
	if a.DecidedAt != nil {
		out.DecidedAt = timestamppb.New(*a.DecidedAt)
	}
	return out
}
