package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

type CreatePublicLink struct {
	links             ShareLinkRepository
	resolvePermission *ResolvePermission
}

func NewCreatePublicLink(links ShareLinkRepository, resolvePermission *ResolvePermission) *CreatePublicLink {
	return &CreatePublicLink{links: links, resolvePermission: resolvePermission}
}

// Execute requires 'manage' on taskID, generates a random 256-bit token,
// stores only its SHA-256 hash, and returns the plaintext exactly once —
// same posture as the Dev Server Agent's own bearer token
// (07-security-architecture.md: "hashed at rest... not plaintext").
func (uc *CreatePublicLink) Execute(ctx context.Context, taskID string) (id, token string, err error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return "", "", apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	callerID, _ := tenant.UserID(ctx)
	if _, err := uc.resolvePermission.Execute(ctx, ResolvePermissionInput{TaskID: taskID, UserID: callerID, Action: "manage"}); err != nil {
		return "", "", err
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", apperrors.New(apperrors.KindInternal, "TASK_PUBLIC_LINK_TOKEN_GEN_FAILED", "failed to generate share token", err)
	}
	token = hex.EncodeToString(raw)
	sum := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(sum[:])

	id, err = uc.links.Create(ctx, tenantID, taskID, tokenHash, callerID)
	if err != nil {
		return "", "", apperrors.New(apperrors.KindInternal, "TASK_PUBLIC_LINK_CREATE_FAILED", "failed to persist share link", err)
	}
	return id, token, nil
}

type ResolvePublicLink struct {
	links ShareLinkRepository
}

func NewResolvePublicLink(links ShareLinkRepository) *ResolvePublicLink {
	return &ResolvePublicLink{links: links}
}

// Execute hashes the incoming token and looks up by token_hash, checking
// revoked_at IS NULL AND (expires_at IS NULL OR expires_at > now()) — this
// does NOT go through domain.ResolveGrant (no subject_id exists for an
// anonymous caller); it's a distinct, deliberately narrower code path that
// never touches the BFS walk.
func (uc *ResolvePublicLink) Execute(ctx context.Context, tenantID, token string) (taskID string, err error) {
	sum := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(sum[:])
	taskID, err = uc.links.ResolveActive(ctx, tenantID, tokenHash)
	if err != nil {
		return "", apperrors.New(apperrors.KindNotFound, "TASK_PUBLIC_LINK_NOT_FOUND", "share link not found, expired, or revoked", err)
	}
	return taskID, nil
}

type RevokePublicLink struct {
	links             ShareLinkRepository
	resolvePermission *ResolvePermission
	tasks             TaskRepository
}

func NewRevokePublicLink(links ShareLinkRepository, resolvePermission *ResolvePermission, tasks TaskRepository) *RevokePublicLink {
	return &RevokePublicLink{links: links, resolvePermission: resolvePermission, tasks: tasks}
}

func (uc *RevokePublicLink) Execute(ctx context.Context, linkID string) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	taskID, err := uc.links.TaskIDFor(ctx, tenantID, linkID)
	if err != nil {
		return apperrors.New(apperrors.KindNotFound, "TASK_PUBLIC_LINK_NOT_FOUND", "share link not found", err)
	}
	callerID, _ := tenant.UserID(ctx)
	if _, err := uc.resolvePermission.Execute(ctx, ResolvePermissionInput{TaskID: taskID, UserID: callerID, Action: "manage"}); err != nil {
		return err
	}
	return uc.links.Revoke(ctx, tenantID, linkID)
}
