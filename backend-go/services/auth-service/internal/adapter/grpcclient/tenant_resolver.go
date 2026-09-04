package grpcclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	tenantv1 "github.com/stablyai/orca-go/proto/gen/go/orca/tenant/v1"
)

// TenantResolver implements usecase.TenantResolver by dialing
// tenant-service's ListCompanies RPC directly, service-to-service — the
// same trust boundary TenantProvisioner's CreateCompany call already uses
// for first-boot bootstrap (see that type's doc comment): an internal,
// system-level provisioning call, not something exposed to end users
// through this service's own gRPC surface, so it needs no caller-side
// admin gate here (ListCompanies' own doc comment only requires the
// *user-facing* callers — e.g. wscompat's admin console — to admin-gate).
type TenantResolver struct {
	conn   *grpc.ClientConn
	client tenantv1.TenantServiceClient
}

func NewTenantResolver(addr string) (*TenantResolver, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("grpcclient: dial tenant-service at %q: %w", addr, err)
	}
	return &TenantResolver{conn: conn, client: tenantv1.NewTenantServiceClient(conn)}, nil
}

func (c *TenantResolver) Close() error {
	return c.conn.Close()
}

// ResolveDefaultTenant returns the sole existing company's id — see
// usecase.TenantResolver's doc comment for why "the one deployment-wide
// tenant" is the only resolvable answer today, and why zero or more than
// one company fails closed rather than guessing.
func (c *TenantResolver) ResolveDefaultTenant(ctx context.Context) (string, error) {
	resp, err := c.client.ListCompanies(ctx, &tenantv1.ListCompaniesRequest{})
	if err != nil {
		return "", fmt.Errorf("grpcclient: tenant-service ListCompanies: %w", err)
	}
	companies := resp.GetCompanies()
	if len(companies) != 1 {
		return "", fmt.Errorf("grpcclient: expected exactly one company, found %d", len(companies))
	}
	return companies[0].GetId(), nil
}
