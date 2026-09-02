// channels_auth_directory.go wires auth.listTenantMemberDirectory — the
// non-admin counterpart to admin.listUsers (channels_admin_users.go). Every
// channel in that file is deliberately admin-gated (its own package doc
// comment); this one is NOT — any authenticated tenant member may call it,
// since it backs member-picker UIs (project/repo MemberManager) that
// previously had only a raw userId to display/collect, with no way for an
// ordinary project owner (not necessarily a tenant admin) to resolve who
// that id belongs to, or look someone up by name/email in the first place.
package wscompat

import (
	"context"
	"encoding/json"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

type tenantMemberDirectoryEntryView struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func registerAuthDirectoryChannels(r *Registry, client authv1.AuthServiceClient) {
	r.Register("auth.listTenantMemberDirectory", func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID, Role: id.Role})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		// Why no request args at all: tenant/actor come from the identity
		// just attached above (the authenticated caller's own), never a
		// request field — see ListTenantMemberDirectory's doc comment in
		// auth.proto for why a caller-supplied tenantId would let any
		// member enumerate an arbitrary tenant's directory.
		resp, err := client.ListTenantMemberDirectory(rpcCtx, &authv1.ListTenantMemberDirectoryRequest{})
		if err != nil {
			return nil, err
		}
		views := make([]tenantMemberDirectoryEntryView, 0, len(resp.GetMembers()))
		for _, m := range resp.GetMembers() {
			views = append(views, tenantMemberDirectoryEntryView{ID: m.GetId(), Name: m.GetName(), Email: m.GetEmail()})
		}
		return views, nil
	})
}
