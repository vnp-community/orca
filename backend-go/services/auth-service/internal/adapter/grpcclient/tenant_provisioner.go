// Package grpcclient holds thin typed-client wrappers dialing peer
// services — mirrors project-service's internal/adapter/grpcclient package
// shape (e.g. its InfraFleetDevServerLister).
package grpcclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	tenantv1 "github.com/stablyai/orca-go/proto/gen/go/orca/tenant/v1"
)

// TenantProvisioner implements usecase.TenantProvisioner by dialing
// tenant-service's CreateCompany RPC — used only by the first-boot
// bootstrap (bootstrap.go), which needs to originate a tenant, not join
// an existing one. See specs/backend-go/bugs/missing-v2/BUG-002.
type TenantProvisioner struct {
	conn   *grpc.ClientConn
	client tenantv1.TenantServiceClient
}

func NewTenantProvisioner(addr string) (*TenantProvisioner, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("grpcclient: dial tenant-service at %q: %w", addr, err)
	}
	return &TenantProvisioner{conn: conn, client: tenantv1.NewTenantServiceClient(conn)}, nil
}

func (c *TenantProvisioner) Close() error {
	return c.conn.Close()
}

// CreateCompany returns the newly-originated tenant ID (== the created
// Company's id) — bootstrap.go uses this as the admin User's tenant_id.
func (c *TenantProvisioner) CreateCompany(ctx context.Context, name string) (string, error) {
	resp, err := c.client.CreateCompany(ctx, &tenantv1.CreateCompanyRequest{Name: name})
	if err != nil {
		return "", fmt.Errorf("grpcclient: tenant-service CreateCompany: %w", err)
	}
	return resp.GetCompany().GetId(), nil
}
