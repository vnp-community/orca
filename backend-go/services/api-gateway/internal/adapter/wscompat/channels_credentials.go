// credentials.* channels (TASK-042) — relay to issue-tracking-service's
// SetIntegrationCredential/GetIntegrationCredentialStatus/
// ListIntegrationCredentials/RevokeAuth (TASK-041) for the jira/linear
// providers, and to scm-integration-service's identically-shaped RPCs for
// the bitbucket/azure-devops/gitea providers (frontend/src/preload/
// api-types.ts's RuntimeCredentialService union and
// runtime-credentials-client.ts list all 5). An unrecognized `service`
// returns CREDENTIALS_UNKNOWN_SERVICE rather than silently no-oping.
//
// mode: "server" is hardcoded on every response — frontend/src/preload/api-types.ts's
// credentials.* only exists in Web Server mode (Electron mode answers these
// calls locally, never over this RPC path), so a request reaching this
// handler at all is, by construction, never Electron's local IPC path.
//
// Response field names matter here: the frontend
// (renderer/src/runtime/runtime-credentials-client.ts) reads `success` on
// set/revoke — NOT `ok` (a naming mismatch other channel groups in this
// package use for their own ad hoc "{ok: bool}" acks; do not copy that
// convention into this file).
package wscompat

import (
	"context"
	"encoding/json"
	"fmt"

	issuetrackingv1 "github.com/stablyai/orca-go/proto/gen/go/orca/issuetracking/v1"
	scmintegrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/scmintegration/v1"
)

// issueCredentialProviders maps credentials.*'s `service` string onto
// issuetrackingv1.IssueProvider — the 2 providers issue-tracking-service
// owns. scmCredentialProviders (below) covers the other 3.
var issueCredentialProviders = map[string]issuetrackingv1.IssueProvider{
	"jira":   issuetrackingv1.IssueProvider_ISSUE_PROVIDER_JIRA,
	"linear": issuetrackingv1.IssueProvider_ISSUE_PROVIDER_LINEAR,
}

// scmCredentialProviders maps credentials.*'s `service` string onto
// scmintegrationv1.ScmProvider — the 3 providers scm-integration-service
// owns for this channel group. Uses the hyphenated "azure-devops" spelling
// from runtime-credentials-client.ts's RuntimeCredentialService union, NOT
// parseWSProvider's (channels_scm.go) underscored "azure_devops" — that
// function serves a different channel group (scm.* provider routing) with
// its own, separately-established string convention; credentials.* has its
// own contract with the frontend and must not silently drift if one changes.
var scmCredentialProviders = map[string]scmintegrationv1.ScmProvider{
	"bitbucket":    scmintegrationv1.ScmProvider_SCM_PROVIDER_BITBUCKET,
	"azure-devops": scmintegrationv1.ScmProvider_SCM_PROVIDER_AZURE_DEVOPS,
	"gitea":        scmintegrationv1.ScmProvider_SCM_PROVIDER_GITEA,
}

func issueCredentialProviderName(p issuetrackingv1.IssueProvider) string {
	for name, v := range issueCredentialProviders {
		if v == p {
			return name
		}
	}
	return ""
}

func scmCredentialProviderName(p scmintegrationv1.ScmProvider) string {
	for name, v := range scmCredentialProviders {
		if v == p {
			return name
		}
	}
	return ""
}

func unknownCredentialsServiceError(service string) error {
	return fmt.Errorf("CREDENTIALS_UNKNOWN_SERVICE: %q is not a recognized credentials.* service for this backend", service)
}

func registerCredentialsChannels(r *Registry, scmClient scmintegrationv1.ScmIntegrationServiceClient, issueTrackingClient issuetrackingv1.IssueTrackingServiceClient) {
	r.Register("credentials.set", handleCredentialsSet(scmClient, issueTrackingClient))
	r.Register("credentials.revoke", handleCredentialsRevoke(scmClient, issueTrackingClient))
	r.Register("credentials.status", handleCredentialsStatus(scmClient, issueTrackingClient))
	r.Register("credentials.list", handleCredentialsList(scmClient, issueTrackingClient))
}

type credentialsServiceArgs struct {
	Service string `json:"service"`
}

type credentialsSetArgs struct {
	Service string            `json:"service"`
	Token   string            `json:"token"`
	Config  map[string]string `json:"config"`
}

func handleCredentialsSet(scmClient scmintegrationv1.ScmIntegrationServiceClient, issueClient issuetrackingv1.IssueTrackingServiceClient) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[credentialsSetArgs](args, 0)
		if err != nil {
			return nil, err
		}
		configJSON, err := json.Marshal(in.Config)
		if err != nil {
			return nil, fmt.Errorf("encoding config: %w", err)
		}
		ctx = attachIdentity(ctx, id)
		rpcCtx, cancel := context.WithTimeout(ctx, groupRPCTimeout)
		defer cancel()

		if provider, ok := scmCredentialProviders[in.Service]; ok {
			_, err := scmClient.SetIntegrationCredential(rpcCtx, &scmintegrationv1.SetIntegrationCredentialRequest{
				TenantId: id.TenantID, Provider: provider, Token: in.Token, ConfigJson: string(configJSON),
			})
			if err != nil {
				return nil, err
			}
			return map[string]bool{"success": true}, nil
		}
		provider, ok := issueCredentialProviders[in.Service]
		if !ok {
			return nil, unknownCredentialsServiceError(in.Service)
		}
		_, err = issueClient.SetIntegrationCredential(rpcCtx, &issuetrackingv1.SetIntegrationCredentialRequest{
			TenantId: id.TenantID, Provider: provider, Token: in.Token, ConfigJson: string(configJSON),
		})
		if err != nil {
			return nil, err
		}
		return map[string]bool{"success": true}, nil
	}
}

func handleCredentialsRevoke(scmClient scmintegrationv1.ScmIntegrationServiceClient, issueClient issuetrackingv1.IssueTrackingServiceClient) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[credentialsServiceArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = attachIdentity(ctx, id)
		rpcCtx, cancel := context.WithTimeout(ctx, groupRPCTimeout)
		defer cancel()

		if provider, ok := scmCredentialProviders[in.Service]; ok {
			_, err := scmClient.RevokeAuth(rpcCtx, &scmintegrationv1.RevokeAuthRequest{TenantId: id.TenantID, Provider: provider})
			if err != nil {
				return nil, err
			}
			return map[string]bool{"success": true}, nil
		}
		provider, ok := issueCredentialProviders[in.Service]
		if !ok {
			return nil, unknownCredentialsServiceError(in.Service)
		}
		_, err = issueClient.RevokeAuth(rpcCtx, &issuetrackingv1.RevokeAuthRequest{TenantId: id.TenantID, Provider: provider})
		if err != nil {
			return nil, err
		}
		return map[string]bool{"success": true}, nil
	}
}

// credentialsStatusView mirrors frontend/src/preload/api-types.ts's
// `credentials.status` return shape — { configured, mode, config? }.
type credentialsStatusView struct {
	Configured bool              `json:"configured"`
	Mode       string            `json:"mode"`
	Config     map[string]string `json:"config,omitempty"`
}

func handleCredentialsStatus(scmClient scmintegrationv1.ScmIntegrationServiceClient, issueClient issuetrackingv1.IssueTrackingServiceClient) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[credentialsServiceArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = attachIdentity(ctx, id)
		rpcCtx, cancel := context.WithTimeout(ctx, groupRPCTimeout)
		defer cancel()

		if provider, ok := scmCredentialProviders[in.Service]; ok {
			resp, err := scmClient.GetIntegrationCredentialStatus(rpcCtx, &scmintegrationv1.GetIntegrationCredentialStatusRequest{
				TenantId: id.TenantID, Provider: provider,
			})
			if err != nil {
				return nil, err
			}
			return newCredentialsStatusView(resp.GetConfigured(), resp.GetConfigJson()), nil
		}
		provider, ok := issueCredentialProviders[in.Service]
		if !ok {
			return nil, unknownCredentialsServiceError(in.Service)
		}
		resp, err := issueClient.GetIntegrationCredentialStatus(rpcCtx, &issuetrackingv1.GetIntegrationCredentialStatusRequest{
			TenantId: id.TenantID, Provider: provider,
		})
		if err != nil {
			return nil, err
		}
		return newCredentialsStatusView(resp.GetConfigured(), resp.GetConfigJson()), nil
	}
}

// newCredentialsStatusView builds credentials.status's response shape from
// the (configured, config_json) pair both backing services return
// identically-shaped.
func newCredentialsStatusView(configured bool, configJSON string) credentialsStatusView {
	view := credentialsStatusView{Configured: configured, Mode: "server"}
	if configJSON != "" {
		// config_json is non-secret sidecar config (e.g. Jira's
		// baseUrl/email) — a decode failure here means unexpected shape,
		// not a fatal error; degrade to omitting Config rather than
		// failing the whole status check.
		var decoded map[string]string
		if err := json.Unmarshal([]byte(configJSON), &decoded); err == nil {
			view.Config = decoded
		}
	}
	return view
}

// credentialsListView mirrors frontend/src/preload/api-types.ts's
// `credentials.list` return shape — { services, mode }.
type credentialsListView struct {
	Services []string `json:"services"`
	Mode     string   `json:"mode"`
}

// handleCredentialsList fans out to BOTH services and merges — the
// frontend's { services, mode } spans all 5 providers in one call.
func handleCredentialsList(scmClient scmintegrationv1.ScmIntegrationServiceClient, issueClient issuetrackingv1.IssueTrackingServiceClient) ChannelHandler {
	return func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
		ctx = attachIdentity(ctx, id)
		rpcCtx, cancel := context.WithTimeout(ctx, groupRPCTimeout)
		defer cancel()

		var services []string

		scmResp, err := scmClient.ListIntegrationCredentials(rpcCtx, &scmintegrationv1.ListIntegrationCredentialsRequest{TenantId: id.TenantID})
		if err != nil {
			return nil, err
		}
		for _, p := range scmResp.GetConfiguredProviders() {
			if name := scmCredentialProviderName(p); name != "" {
				services = append(services, name)
			}
		}

		issueResp, err := issueClient.ListIntegrationCredentials(rpcCtx, &issuetrackingv1.ListIntegrationCredentialsRequest{TenantId: id.TenantID})
		if err != nil {
			return nil, err
		}
		for _, p := range issueResp.GetConfiguredProviders() {
			if name := issueCredentialProviderName(p); name != "" {
				services = append(services, name)
			}
		}

		return credentialsListView{Services: services, Mode: "server"}, nil
	}
}
