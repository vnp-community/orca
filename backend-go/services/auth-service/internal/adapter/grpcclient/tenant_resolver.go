package grpcclient

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	tenantv1 "github.com/stablyai/orca-go/proto/gen/go/orca/tenant/v1"
)

// TenantResolver implements usecase.TenantResolver by dialing
// tenant-service directly, service-to-service — the same trust boundary
// TenantProvisioner's CreateCompany call already uses for first-boot
// bootstrap (see that type's doc comment): an internal, system-level
// resolution call, not something exposed to end users through this
// service's own gRPC surface, so it needs no caller-side admin gate here.
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

// ResolveTenantForEmail resolves the domain half of email against
// tenant-service's ResolveCompanyByEmailDomain RPC first (the multi-tenant
// path — see usecase.TenantResolver's doc comment), falling back to "the
// sole existing company" when no domain is registered, for deployments
// that haven't set up per-domain SSO tenant routing yet. Fails closed
// (an error) when neither resolves.
func (c *TenantResolver) ResolveTenantForEmail(ctx context.Context, email string) (string, error) {
	domain := emailDomain(email)
	if domain == "" {
		return "", fmt.Errorf("grpcclient: email %q has no domain to resolve a tenant from", email)
	}

	resolveResp, err := c.client.ResolveCompanyByEmailDomain(ctx, &tenantv1.ResolveCompanyByEmailDomainRequest{EmailDomain: domain})
	if err != nil {
		return "", fmt.Errorf("grpcclient: tenant-service ResolveCompanyByEmailDomain: %w", err)
	}
	if resolveResp.GetFound() {
		return resolveResp.GetCompanyId(), nil
	}

	// Fallback: single-tenant deployments that haven't registered any
	// email domains yet — matches this resolver's original (pre-multi-
	// tenant) behavior exactly, so an existing single-company deployment
	// keeps working unmodified.
	listResp, err := c.client.ListCompanies(ctx, &tenantv1.ListCompaniesRequest{})
	if err != nil {
		return "", fmt.Errorf("grpcclient: tenant-service ListCompanies: %w", err)
	}
	companies := listResp.GetCompanies()
	if len(companies) == 1 {
		return companies[0].GetId(), nil
	}
	return "", fmt.Errorf("grpcclient: no company has registered email domain %q, and this deployment has %d companies (need exactly 1 for the single-tenant fallback)", domain, len(companies))
}

// emailDomain extracts the lowercased domain half of an email address —
// "" if addr has no "@". Mirrors tenant-service's own
// domain.EmailDomainFromAddress; duplicated rather than imported since
// auth-service and tenant-service share no internal package (each owns its
// own domain/ per specs/backend-go/architecture/03-clean-architecture-guidelines.md).
func emailDomain(addr string) string {
	i := strings.LastIndex(addr, "@")
	if i < 0 || i == len(addr)-1 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(addr[i+1:]))
}
