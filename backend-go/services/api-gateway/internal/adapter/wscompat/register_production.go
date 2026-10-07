package wscompat

import (
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

	aiproviderv1 "github.com/stablyai/orca-go/proto/gen/go/orca/aiprovider/v1"
	annotationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/annotation/v1"
	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
	automationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/automation/v1"
	credentialbrokerv1 "github.com/stablyai/orca-go/proto/gen/go/orca/credentialbroker/v1"
	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	issuetrackingv1 "github.com/stablyai/orca-go/proto/gen/go/orca/issuetracking/v1"
	orchestrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/orchestration/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	scmintegrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/scmintegration/v1"
	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
	tenantv1 "github.com/stablyai/orca-go/proto/gen/go/orca/tenant/v1"
	workflowv1 "github.com/stablyai/orca-go/proto/gen/go/orca/workflow/v1"

	codeintelv1 "github.com/stablyai/orca-go/proto/gen/go/orca/codeintel/v1"
	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
)

// ChannelDeps is everything the non-MCP channel registrations need. Every
// field may be nil: registration only builds closures, so tests and CI can
// build the real channel inventory without any downstream.
type ChannelDeps struct {
	Annotation       annotationv1.AnnotationServiceClient
	Task             taskv1.TaskServiceClient
	Git              gitgatewayv1.GitGatewayServiceClient
	Automation       automationv1.AutomationServiceClient
	InfraFleet       infrafleetv1.InfraFleetServiceClient
	Tenant           tenantv1.TenantServiceClient
	Project          projectv1.ProjectServiceClient
	IssueTracking    issuetrackingv1.IssueTrackingServiceClient
	Orchestration    orchestrationv1.OrchestrationServiceClient
	Scm              scmintegrationv1.ScmIntegrationServiceClient
	Workflow         workflowv1.WorkflowServiceClient
	AIProvider       aiproviderv1.AiProviderServiceClient
	CredentialBroker credentialbrokerv1.CredentialBrokerServiceClient
	Auth             authv1.AuthServiceClient
	RateLimits       rateLimitReader
	FanOut           *usecase.FanOutCreateWorktrees
	EventBus         *commoneventbus.Consumer

	NotificationStream NotificationStreamOpener
	ClientEvents       *ClientEventBus
	WorkspaceEvents    *WorkspaceEventBus
	DeviceSecrets      DeviceSecretResolver

	// TaskActivityEnabled registers task.activity.subscribe; production sets it
	// only when NATS connected (the consumer itself is then non-nil).
	TaskActivityEnabled bool
	TaskActivityBus     *commoneventbus.Consumer

	CodeIntel       codeintelv1.CodeIntelServiceClient
	QualityGate     codeintelv1.QualityGateServiceClient
	CodeIntelLimits CodeIntelLimits
}

// RegisterProductionChannels registers every non-mcp.* channel in the order
// cmd/server used to, so production, tests and the MCP parity check share one
// inventory. Later registrations overwrite earlier ones (git.diff).
func RegisterProductionChannels(r *Registry, d ChannelDeps) {
	RegisterRealChannels(r, d.Annotation, d.Task, d.Git, d.Automation, d.InfraFleet, d.Tenant, d.Project,
		d.IssueTracking, d.Orchestration, d.Scm, d.Workflow, d.AIProvider, d.CredentialBroker, d.Auth,
		d.RateLimits, d.FanOut, d.EventBus)
	clientEvents := d.ClientEvents
	if clientEvents == nil {
		clientEvents = NewClientEventBus()
	}
	RegisterPushChannels(r, d.NotificationStream, clientEvents, d.InfraFleet)
	RegisterClientStateChannels(r, d.Tenant)
	if d.TaskActivityEnabled {
		RegisterTaskActivityStreamChannel(r, d.TaskActivityBus)
	}
	workspaceEvents := d.WorkspaceEvents
	if workspaceEvents == nil {
		workspaceEvents = NewWorkspaceEventBus()
	}
	RegisterWorkspaceSubscribeChannel(r, workspaceEvents)
	RegisterMobileChannels(r, d.InfraFleet, d.Project, d.DeviceSecrets)
	registerCodeIntelChannels(r, d)
}
