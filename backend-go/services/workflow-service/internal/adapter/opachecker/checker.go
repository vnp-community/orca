// Package opachecker implements usecase.OPAChecker — "is this user an
// admin" is an auth-service fact (see that port's doc comment), not
// something workflow-service determines from its own tables. auth-service
// exposes no direct GetUser(id) RPC, only paginated ListUsers(tenant_id) —
// this checker pages through it looking for userID, matching the "no
// dedicated lookup RPC exists yet" constraint honestly rather than
// inventing one out of scope for BUG-WF-03.
package opachecker

import (
	"context"

	"google.golang.org/grpc/metadata"

	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/tenant"
	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
	"github.com/stablyai/orca-go/services/workflow-service/internal/usecase"
)

// listUsersPageSize bounds each ListUsers page — an admin check pages
// through at most a handful of tenant-sized batches, not the whole table
// in one call.
const listUsersPageSize = 100

type checker struct {
	auth authv1.AuthServiceClient
}

// New builds a usecase.OPAChecker against an already-dialed auth-service
// client — see cmd/server/main.go for the real dial, and this package's
// tests for a fake.
func New(auth authv1.AuthServiceClient) usecase.OPAChecker {
	return &checker{auth: auth}
}

// IsAdmin fails closed: any error reaching or paging through auth-service,
// or userID simply not being found, answers false — an approval gate must
// never accidentally grant admin power because of a transient RPC failure.
func (c *checker) IsAdmin(ctx context.Context, userID string) bool {
	tenantID, ok := tenant.TenantID(ctx)
	if !ok {
		return false
	}
	ctx = metadata.AppendToOutgoingContext(ctx, grpcmw.MetadataTenantID, tenantID)

	pageToken := ""
	for {
		resp, err := c.auth.ListUsers(ctx, &authv1.ListUsersRequest{TenantId: tenantID, PageToken: pageToken, PageSize: listUsersPageSize})
		if err != nil {
			return false
		}
		for _, u := range resp.GetUsers() {
			if u.GetId() == userID {
				return u.GetRole() == authv1.Role_ROLE_ADMIN
			}
		}
		pageToken = resp.GetNextPageToken()
		if pageToken == "" {
			return false
		}
	}
}
