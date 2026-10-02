// Package brokerclient is mcp-service's only path to secret material: it
// stores and reads external MCP server secrets in credential-broker-service
// (category mcp_external_secret). The service never talks to Vault itself.
package brokerclient

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"

	credentialbrokerv1 "github.com/stablyai/orca-go/proto/gen/go/orca/credentialbroker/v1"
)

// serviceID is the caller identity the broker's category allow-list admits.
const (
	serviceIDKey   = "x-orca-service-id"
	serviceID      = "mcp-service"
	DefaultTimeout = 5 * time.Second
	category       = credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_MCP_EXTERNAL_SECRET
)

func Dial(addr string) (*grpc.ClientConn, error) {
	return grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
}

type Client struct {
	api     credentialbrokerv1.CredentialBrokerServiceClient
	timeout time.Duration
}

var _ usecase.SecretBroker = (*Client)(nil)

func New(api credentialbrokerv1.CredentialBrokerServiceClient, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &Client{api: api, timeout: timeout}
}

func (c *Client) call(ctx context.Context, tenantID string) (context.Context, context.CancelFunc) {
	ctx = metadata.AppendToOutgoingContext(ctx, serviceIDKey, serviceID, grpcmw.MetadataTenantID, tenantID)
	return context.WithTimeout(ctx, c.timeout)
}

// failure keeps only the gRPC code: a transport or Vault error text must never
// carry request content into logs.
func failure(op string, err error) error {
	return fmt.Errorf("brokerclient: %s: %s", op, status.Code(err))
}

// Put writes a new credential, or rotates the existing one so there is only
// ever one live secret per (server, kind, name).
func (c *Client) Put(ctx context.Context, tenantID, ownerID string, value domain.SecretValue) error {
	ctx, cancel := c.call(ctx, tenantID)
	defer cancel()
	meta, err := c.api.GetCredentialMetadataByOwner(ctx, &credentialbrokerv1.GetCredentialMetadataByOwnerRequest{
		TenantId: tenantID, Category: category, OwnerId: ownerID})
	if err != nil {
		return failure("lookup", err)
	}
	if m := meta.GetMetadata(); m != nil {
		if _, err := c.api.RotateCredential(ctx, &credentialbrokerv1.RotateCredentialRequest{
			CredentialId: m.GetId(), NewEncryptedEnvelope: value.Reveal()}); err != nil {
			return failure("rotate", err)
		}
		return nil
	}
	// The field is named encrypted_envelope but the broker treats it as opaque
	// bytes and Transit-encrypts it (decision D1): plaintext crosses only this hop.
	if _, err := c.api.WriteCredential(ctx, &credentialbrokerv1.WriteCredentialRequest{
		TenantId: tenantID, OwnerId: ownerID, Category: category, EncryptedEnvelope: value.Reveal()}); err != nil {
		return failure("write", err)
	}
	return nil
}

func (c *Client) Get(ctx context.Context, tenantID, ownerID string) (domain.SecretValue, error) {
	ctx, cancel := c.call(ctx, tenantID)
	defer cancel()
	resp, err := c.api.ResolveCredentialByOwner(ctx, &credentialbrokerv1.ResolveCredentialByOwnerRequest{
		TenantId: tenantID, Category: category, OwnerId: ownerID})
	if err != nil {
		return domain.SecretValue{}, failure("resolve", err)
	}
	return domain.NewSecretValue(string(resp.GetValue())), nil
}

func (c *Client) Delete(ctx context.Context, tenantID, ownerID string) error {
	ctx, cancel := c.call(ctx, tenantID)
	defer cancel()
	_, err := c.api.RevokeCredentialByOwner(ctx, &credentialbrokerv1.RevokeCredentialByOwnerRequest{
		TenantId: tenantID, Category: category, OwnerId: ownerID})
	if err != nil && status.Code(err) != codes.NotFound {
		return failure("revoke", err)
	}
	return nil
}
