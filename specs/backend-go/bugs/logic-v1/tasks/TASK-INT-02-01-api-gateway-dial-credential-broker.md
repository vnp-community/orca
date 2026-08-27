# TASK-INT-02-01: Dial `credential-broker-service` from `api-gateway`

**From Solution:** SOL-INT-02
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/cmd/server/main.go`, `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

`registry.go:79-81`'s doc comment states `credential-broker-service` is
"reached only indirectly via infra-fleet-service's credential path — no
client calls it through this gateway directly." `CREDENTIAL_BROKER_SERVICE_ADDR`
is already read into `config.go`'s `OtherServiceAddrs` map
(`config.go:100`) but nothing dials it. This task adds the dial and threads
the client through `RegisterRealChannels`'s signature; TASK-INT-02-02 adds
the channels that actually use it.

## Changes to make

In `backend-go/services/api-gateway/cmd/server/main.go`, add the dial
alongside the other `gatewaygrpc.Dial(cfg.OtherServiceAddrs[...])` calls
(`:146-151` is `infraFleetConn`/`infraFleetClient`'s pattern to mirror):

```go
credentialBrokerConn, err := gatewaygrpc.Dial(cfg.OtherServiceAddrs["credential-broker-service"])
if err != nil {
	return fmt.Errorf("dialing credential-broker-service: %w", err)
}
defer credentialBrokerConn.Close()
credentialBrokerClient := credentialbrokerv1.NewCredentialBrokerServiceClient(credentialBrokerConn)
```

(add `credentialbrokerv1 "github.com/stablyai/orca-go/proto/gen/go/orca/credentialbroker/v1"`
import.)

In `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`,
extend `RegisterRealChannels`'s signature with the new client:

```go
func RegisterRealChannels(
	r *Registry,
	annotationClient annotationv1.AnnotationServiceClient,
	taskClient taskv1.TaskServiceClient,
	gitClient gitgatewayv1.GitGatewayServiceClient,
	automationClient automationv1.AutomationServiceClient,
	infraFleetClient infrafleetv1.InfraFleetServiceClient,
	tenantClient tenantv1.TenantServiceClient,
	projectClient projectv1.ProjectServiceClient,
	issueTrackingClient issuetrackingv1.IssueTrackingServiceClient,
	orchestrationClient orchestrationv1.OrchestrationServiceClient,
	scmClient scmintegrationv1.ScmIntegrationServiceClient,
	workflowClient workflowv1.WorkflowServiceClient,
	aiProviderClient aiproviderv1.AiProviderServiceClient,
	credentialBrokerClient credentialbrokerv1.CredentialBrokerServiceClient, // NEW — SOL-INT-02
	rateLimits rateLimitReader,
) {
	// ... unchanged body ...
}
```

Update the call site in `main.go` (`:246-251`) to pass
`credentialBrokerClient` in the same position, and update `registry.go`'s
now-stale doc comment (`:79-81`) — see TASK-INT-02-02 for the exact text.

Leave the actual `registerCredentialsChannels(r, credentialBrokerClient)`
call out of `RegisterRealChannels`'s body for this task — TASK-INT-02-02
adds it once the channels file exists, so this task alone compiles with an
as-yet-unused parameter only if Go's compiler tolerates that (it does for
function parameters, unlike local variables) — no `_ = credentialBrokerClient`
needed.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/...
go vet ./services/api-gateway/...
```

Expected: clean build — `credentialBrokerClient` is dialed and threaded
through but not yet consumed by any channel (that lands in TASK-INT-02-02).
