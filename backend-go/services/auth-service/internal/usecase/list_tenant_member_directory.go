package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

type TenantMemberDirectoryEntry struct {
	ID    string
	Name  string
	Email string
}

// ListTenantMemberDirectory is the non-admin counterpart to ListUsers: any
// authenticated user may look up their OWN tenant's other members by
// name/email — needed by member-picker UIs (project/repo MemberManager,
// which previously only had a raw userId to work with and no way to
// resolve who it belonged to; ListUsers itself is admin-console-only, per
// its own doc comment, so it can't back a picker every project owner
// needs).
//
// Deliberately minimal projection (id/name/email only — not role/isActive/
// createdAt, which ListUsers's admin view exposes) and deliberately
// tenant-scoped from the authenticated actor's OWN context, never a
// caller-supplied tenantId — that would let any member enumerate an
// arbitrary tenant's directory.
type ListTenantMemberDirectory struct {
	users UserRepository
}

func NewListTenantMemberDirectory(users UserRepository) *ListTenantMemberDirectory {
	return &ListTenantMemberDirectory{users: users}
}

func (uc *ListTenantMemberDirectory) Execute(ctx context.Context) ([]TenantMemberDirectoryEntry, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "AUTH_NO_TENANT", "no tenant in request context", err)
	}
	if _, ok := tenant.UserID(ctx); !ok {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "AUTH_NO_ACTOR", "no authenticated user in request context", nil)
	}

	// Why a large fixed page size, no pagination surfaced to callers: this
	// backs a member-picker dropdown, not an admin list view — a single
	// page comfortably covers realistic tenant sizes for v1.
	users, _, err := uc.users.ListUsers(ctx, tenantID, "", 200)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "AUTH_LIST_TENANT_MEMBERS_FAILED", "failed to list tenant members", err)
	}

	entries := make([]TenantMemberDirectoryEntry, 0, len(users))
	for _, u := range users {
		entries = append(entries, TenantMemberDirectoryEntry{ID: u.ID, Name: u.Name, Email: u.Email})
	}
	return entries, nil
}
