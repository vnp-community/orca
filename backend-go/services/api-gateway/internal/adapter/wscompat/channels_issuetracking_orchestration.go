// registerIssueTrackingOrchestrationChannels is this pass's single wiring
// entry point for the jira.*/linear.*/nativeChat.readSession/
// orchestration.dispatchShow channels (TASK-096..112) — kept out of
// channels.go on purpose: other task groups add their own channel
// registrations to channels.go's RegisterRealChannels in parallel
// worktrees, so this file adds a new call site instead of editing that
// shared function directly. The final integration pass wires this one
// call into RegisterRealChannels (see the one-line call documented below).
package wscompat

import (
	"time"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	issuetrackingv1 "github.com/stablyai/orca-go/proto/gen/go/orca/issuetracking/v1"
	orchestrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/orchestration/v1"
)

// groupRPCTimeout is the per-RPC deadline this task group's channel
// handlers (jira.*/linear.*/nativeChat.readSession/
// orchestration.dispatchShow) apply to their outbound gRPC calls — this
// worktree's channels.go baseline predates the shared `rpcTimeout` const
// other parallel task groups may add there, so this is deliberately its
// own name/const rather than assuming that symbol exists; a future
// integration pass can collapse the two once channels.go actually defines
// one.
const groupRPCTimeout = 8 * time.Second

// registerIssueTrackingOrchestrationChannels registers every channel this
// task group owns:
//   - jira.*      (19 channels, TASK-100)
//   - linear.*    (19 channels, TASK-106)
//   - nativeChat.readSession (1 channel, TASK-108)
//   - orchestration.dispatchShow (1 channel, TASK-111)
//
// issueTrackingClient/orchestrationClient/infraFleetClient are all
// already dialed in cmd/server/main.go for their respective REST routes
// (see main.go's issueTrackingClient/orchestrationClient/infraFleetClient
// local vars) — no new dial needed here, same convention
// registerDevServerChannels/registerFleetChannels already established for
// reusing infraFleetClient across multiple register*Channels calls.
//
// Wiring this into the running server needs exactly one addition at the
// call site in channels.go's RegisterRealChannels (not made here per this
// task's "do not edit channels.go" instruction):
//
//	registerIssueTrackingOrchestrationChannels(r, issueTrackingClient, orchestrationClient, infraFleetClient)
//
// and main.go's existing wscompat.RegisterRealChannels(...) call already
// has issueTrackingClient/orchestrationClient/infraFleetClient in scope to
// pass through once RegisterRealChannels's own parameter list is extended
// by whichever integration pass merges every task group's channels.go
// changes together.
func registerIssueTrackingOrchestrationChannels(
	r *Registry,
	issueTrackingClient issuetrackingv1.IssueTrackingServiceClient,
	orchestrationClient orchestrationv1.OrchestrationServiceClient,
	infraFleetClient infrafleetv1.InfraFleetServiceClient,
) {
	registerJiraChannels(r, issueTrackingClient)
	registerLinearChannels(r, issueTrackingClient)
	registerNativeChatChannels(r, infraFleetClient)
	registerOrchestrationChannels(r, orchestrationClient)
}
