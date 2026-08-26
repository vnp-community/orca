// credentials.* channels (TASK-042) — relay to issue-tracking-service's
// SetIntegrationCredential/GetIntegrationCredentialStatus/
// ListIntegrationCredentials/RevokeAuth (TASK-041) for the jira/linear
// providers. scm-integration-service-backed providers (bitbucket,
// azure-devops, gitea — see frontend/src/preload/api-types.ts's
// RuntimeCredentialService union and runtime-credentials-client.ts) are
// deliberately out of scope for this pass — only jira/linear are wired
// here; an unrecognized `service` (including a valid-but-not-yet-wired scm
// one) returns CREDENTIALS_UNKNOWN_SERVICE rather than silently no-oping.
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
)

// issueCredentialProviders maps credentials.*'s `service` string onto
// issuetrackingv1.IssueProvider — the only 2 providers this file's fan-out
// target (issue-tracking-service) owns. See this file's package doc comment
// for why scm-integration-service's 3 providers aren't included here.
var issueCredentialProviders = map[string]issuetrackingv1.IssueProvider{
	"jira":   issuetrackingv1.IssueProvider_ISSUE_PROVIDER_JIRA,
	"linear": issuetrackingv1.IssueProvider_ISSUE_PROVIDER_LINEAR,
}

func issueCredentialProviderName(p issuetrackingv1.IssueProvider) string {
	for name, v := range issueCredentialProviders {
		if v == p {
			return name
		}
	}
	return ""
}

func unknownCredentialsServiceError(service string) error {
	return fmt.Errorf("CREDENTIALS_UNKNOWN_SERVICE: %q is not a recognized credentials.* service for this backend", service)
}

func registerCredentialsChannels(r *Registry, issueTrackingClient issuetrackingv1.IssueTrackingServiceClient) {
	r.Register("credentials.set", handleCredentialsSet(issueTrackingClient))
	r.Register("credentials.revoke", handleCredentialsRevoke(issueTrackingClient))
	r.Register("credentials.status", handleCredentialsStatus(issueTrackingClient))
	r.Register("credentials.list", handleCredentialsList(issueTrackingClient))
}

type credentialsServiceArgs struct {
	Service string `json:"service"`
}

type credentialsSetArgs struct {
	Service string            `json:"service"`
	Token   string            `json:"token"`
	Config  map[string]string `json:"config"`
}

func handleCredentialsSet(client issuetrackingv1.IssueTrackingServiceClient) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[credentialsSetArgs](args, 0)
		if err != nil {
			return nil, err
		}
		provider, ok := issueCredentialProviders[in.Service]
		if !ok {
			return nil, unknownCredentialsServiceError(in.Service)
		}
		configJSON, err := json.Marshal(in.Config)
		if err != nil {
			return nil, fmt.Errorf("encoding config: %w", err)
		}
		ctx = attachIdentity(ctx, id)
		rpcCtx, cancel := context.WithTimeout(ctx, groupRPCTimeout)
		defer cancel()

		_, err = client.SetIntegrationCredential(rpcCtx, &issuetrackingv1.SetIntegrationCredentialRequest{
			TenantId: id.TenantID, Provider: provider, Token: in.Token, ConfigJson: string(configJSON),
		})
		if err != nil {
			return nil, err
		}
		return map[string]bool{"success": true}, nil
	}
}

func handleCredentialsRevoke(client issuetrackingv1.IssueTrackingServiceClient) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[credentialsServiceArgs](args, 0)
		if err != nil {
			return nil, err
		}
		provider, ok := issueCredentialProviders[in.Service]
		if !ok {
			return nil, unknownCredentialsServiceError(in.Service)
		}
		ctx = attachIdentity(ctx, id)
		rpcCtx, cancel := context.WithTimeout(ctx, groupRPCTimeout)
		defer cancel()

		_, err = client.RevokeAuth(rpcCtx, &issuetrackingv1.RevokeAuthRequest{TenantId: id.TenantID, Provider: provider})
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

func handleCredentialsStatus(client issuetrackingv1.IssueTrackingServiceClient) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[credentialsServiceArgs](args, 0)
		if err != nil {
			return nil, err
		}
		provider, ok := issueCredentialProviders[in.Service]
		if !ok {
			return nil, unknownCredentialsServiceError(in.Service)
		}
		ctx = attachIdentity(ctx, id)
		rpcCtx, cancel := context.WithTimeout(ctx, groupRPCTimeout)
		defer cancel()

		resp, err := client.GetIntegrationCredentialStatus(rpcCtx, &issuetrackingv1.GetIntegrationCredentialStatusRequest{
			TenantId: id.TenantID, Provider: provider,
		})
		if err != nil {
			return nil, err
		}
		view := credentialsStatusView{Configured: resp.GetConfigured(), Mode: "server"}
		if cfg := resp.GetConfigJson(); cfg != "" {
			var decoded map[string]string
			// config_json is non-secret sidecar config (e.g. Jira's
			// baseUrl/email) — a decode failure here means unexpected shape,
			// not a fatal error; degrade to omitting Config rather than
			// failing the whole status check.
			if err := json.Unmarshal([]byte(cfg), &decoded); err == nil {
				view.Config = decoded
			}
		}
		return view, nil
	}
}

// credentialsListView mirrors frontend/src/preload/api-types.ts's
// `credentials.list` return shape — { services, mode }.
type credentialsListView struct {
	Services []string `json:"services"`
	Mode     string   `json:"mode"`
}

func handleCredentialsList(client issuetrackingv1.IssueTrackingServiceClient) ChannelHandler {
	return func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
		ctx = attachIdentity(ctx, id)
		rpcCtx, cancel := context.WithTimeout(ctx, groupRPCTimeout)
		defer cancel()

		resp, err := client.ListIntegrationCredentials(rpcCtx, &issuetrackingv1.ListIntegrationCredentialsRequest{TenantId: id.TenantID})
		if err != nil {
			return nil, err
		}
		services := make([]string, 0, len(resp.GetConfiguredProviders()))
		for _, p := range resp.GetConfiguredProviders() {
			if name := issueCredentialProviderName(p); name != "" {
				services = append(services, name)
			}
		}
		return credentialsListView{Services: services, Mode: "server"}, nil
	}
}
