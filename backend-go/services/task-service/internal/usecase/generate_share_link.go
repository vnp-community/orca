package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// GenerateShareLinkInput mirrors the GenerateShareLink RPC request — UserID
// is the acting caller's own identity, sent explicitly on the wire rather
// than extracted from context, matching ResolvePermissionInput.UserID's
// existing convention (this usecase calls ResolvePermission internally,
// which already expects that shape).
type GenerateShareLinkInput struct {
	TaskID string
	UserID string
}

// GenerateShareLink mints a public share-link token for a task
// (TASK-TG-003-05 — SECURITY REVIEW REQUIRED before merge, see
// domain.TaskShareView's doc comment). Requires the caller to already hold
// at least admin-level permission on the task — "who can create a public
// link to this task" is itself a security decision, defaulted to the same
// level task_grant.rego's level_actions map reserves for the most sensitive
// actions (backend-go/policy/orca-authz/task_grant.rego:25-31).
type GenerateShareLink struct {
	tasks             TaskRepository
	resolvePermission *ResolvePermission
}

func NewGenerateShareLink(tasks TaskRepository, resolvePermission *ResolvePermission) *GenerateShareLink {
	return &GenerateShareLink{tasks: tasks, resolvePermission: resolvePermission}
}

func (uc *GenerateShareLink) Execute(ctx context.Context, in GenerateShareLinkInput) (string, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return "", apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	if in.TaskID == "" {
		return "", apperrors.New(apperrors.KindInvalidArgument, "TASK_MISSING_ID", "task_id is required", nil)
	}

	// Precondition: the caller must already hold admin-level permission on
	// the task. ResolvePermission.Execute returns a PermissionDenied-shaped
	// error on any deny — propagated as-is, not re-wrapped, so the caller
	// sees the identical error shape every other permission check in this
	// service returns.
	if _, err := uc.resolvePermission.Execute(ctx, ResolvePermissionInput{TaskID: in.TaskID, UserID: in.UserID, Action: "admin"}); err != nil {
		return "", err
	}

	task, err := uc.tasks.Get(ctx, tenantID, in.TaskID)
	if err != nil {
		return "", apperrors.New(apperrors.KindNotFound, "TASK_NOT_FOUND", "task not found", err)
	}

	token, err := generateShareToken()
	if err != nil {
		return "", apperrors.New(apperrors.KindInternal, "TASK_SHARE_LINK_TOKEN_GEN_FAILED", "failed to generate a share token", err)
	}
	task.ShareToken = &token
	if err := uc.tasks.Update(ctx, tenantID, task); err != nil {
		return "", apperrors.New(apperrors.KindInternal, "TASK_SHARE_LINK_SAVE_FAILED", "failed to persist the share token", err)
	}
	return token, nil
}

// generateShareToken returns a cryptographically random, URL-safe token —
// deliberately crypto/rand (a CSPRNG), NOT math/rand and NOT a UUID derived
// from the task ID or any other guessable value. github.com/google/uuid's
// v4 IS itself CSPRNG-backed in Go's implementation, so it would also be a
// safe choice here, but this uses crypto/rand directly and explicitly so a
// future reader can see the unguessability property at the call site
// without having to know that library-internal detail — a share token must
// never be "simplified" into anything derived from the task's own ID, an
// incrementing counter, or a lower-entropy source, since it IS the entire
// authorization for the public GetTaskByShareToken path (TASK-TG-003-05).
// 32 random bytes hex-encoded = 256 bits of entropy, 64 URL-safe characters.
func generateShareToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
