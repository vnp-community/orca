// Package aiproviderclient implements usecase.AIProviderClient against
// ai-provider-service's gRPC surface — used by ProviderResolver to run its
// explicit-pin-vs-priority-chain resolution.
package aiproviderclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	aiproviderv1 "github.com/stablyai/orca-go/proto/gen/go/orca/aiprovider/v1"
)

// Dial opens an insecure gRPC connection to ai-provider-service — same
// pattern as infrafleetclient.Dial/projectclient.Dial (this service's other
// outbound gRPC dependencies).
func Dial(addr string) (*grpc.ClientConn, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("aiproviderclient: dial ai-provider-service at %q: %w", addr, err)
	}
	return conn, nil
}

// Client wraps aiproviderv1.AiProviderServiceClient. Note the generated
// type's own casing: "AiProviderServiceClient", not "AIProviderServiceClient" —
// confirmed against the live generated code before using it.
type Client struct {
	client aiproviderv1.AiProviderServiceClient
}

func New(client aiproviderv1.AiProviderServiceClient) *Client {
	return &Client{client: client}
}

// ResolveForProject runs ai-provider-service's ResolveProvider RPC — the
// real user > project > server priority chain (see TASK-WF-002-02's
// Context: BE-SOL-002's sketch named a nonexistent "ResolveForContext" RPC;
// the real one is ResolveProvider).
func (c *Client) ResolveForProject(ctx context.Context, userID, projectID string) (string, error) {
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return "", fmt.Errorf("aiproviderclient: ResolveForProject: %w", err)
	}
	resp, err := c.client.ResolveProvider(ctx, &aiproviderv1.ResolveProviderRequest{
		UserId: userID, ProjectId: projectID,
	})
	if err != nil {
		return "", err
	}
	return resp.GetAccount().GetId(), nil
}

// GetAccountStatus validates an explicit provider pin. aiprovider.proto has
// no single-account-lookup RPC (confirmed live — the full RPC list is
// CreateAccount/ResolveProvider/RotateKey/GetUsageToday/ListAccounts/
// UpdateAccount/DeleteAccount/WriteCredential/TestConnection, no
// GetAccount), so this filters ListAccounts (tenant-scoped server-side,
// same "ignored request field, taken from ctx metadata" convention as
// ResolveProviderRequest) by id client-side.
func (c *Client) GetAccountStatus(ctx context.Context, accountID string) (string, error) {
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return "", fmt.Errorf("aiproviderclient: GetAccountStatus: %w", err)
	}
	resp, err := c.client.ListAccounts(ctx, &aiproviderv1.ListAccountsRequest{})
	if err != nil {
		return "", err
	}
	for _, account := range resp.GetAccounts() {
		if account.GetId() == accountID {
			return account.GetStatus(), nil
		}
	}
	return "", fmt.Errorf("aiproviderclient: GetAccountStatus: account %q not found", accountID)
}
