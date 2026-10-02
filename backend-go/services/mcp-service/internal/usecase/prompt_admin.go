package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

// PromptRepository persists tenant custom prompts. Every write enqueues its
// events in the same transaction; implementations enforce the per-tenant cap
// and name uniqueness (domain.ErrPromptNameConflict / ErrPromptInvalid) and
// optimistic concurrency (domain.ErrPromptVersionConflict, domain.ErrNotFound).
type PromptRepository interface {
	ListPrompts(ctx context.Context, tenantID string) ([]domain.CustomPrompt, error)
	CreatePrompt(ctx context.Context, tenantID string, p domain.CustomPrompt, events []domain.OutboxRecord) (domain.CustomPrompt, error)
	// UpdatePrompt applies only when the stored version equals p.Version; the
	// stored version then becomes p.Version+1.
	UpdatePrompt(ctx context.Context, tenantID string, p domain.CustomPrompt, events []domain.OutboxRecord) (domain.CustomPrompt, error)
	DeletePrompt(ctx context.Context, tenantID, id string, events []domain.OutboxRecord) error
}

// PromptAdmin implements prompt list/upsert/delete.
type PromptAdmin struct {
	repo  PromptRepository
	clock Clock
}

func NewPromptAdmin(repo PromptRepository, clock Clock) *PromptAdmin {
	return &PromptAdmin{repo: repo, clock: clock}
}

// List is open to any authenticated role: /mcp prompts/list needs the
// tenant's prompts for ordinary users. The admin gate is the gateway channel.
func (uc *PromptAdmin) List(ctx context.Context) ([]domain.CustomPrompt, error) {
	id, err := userCaller(ctx)
	if err != nil {
		return nil, err
	}
	out, err := uc.repo.ListPrompts(ctx, id.TenantID)
	if err != nil {
		return nil, wrapRepoErr(err, "failed to list prompts")
	}
	return out, nil
}

// Upsert: empty ID creates (version 1); otherwise p.Version must match.
func (uc *PromptAdmin) Upsert(ctx context.Context, p domain.CustomPrompt) (domain.CustomPrompt, error) {
	id, err := userCaller(ctx)
	if err != nil {
		return domain.CustomPrompt{}, err
	}
	if err := id.requireAdmin(); err != nil {
		return domain.CustomPrompt{}, err
	}
	if err := p.Validate(); err != nil {
		return domain.CustomPrompt{}, err
	}
	now := uc.clock.Now()
	p.UpdatedBy, p.UpdatedAt = id.UserID, now
	if p.ID == "" {
		p.ID, p.Version, p.CreatedBy = uuid.NewString(), 1, id.UserID
		evs, err := uc.events(id, p.ID, p.Name, "upserted", p.Version, domain.AuditActionPromptUpsert, now)
		if err != nil {
			return domain.CustomPrompt{}, domain.ErrInternal("failed to build events", err)
		}
		out, err := uc.repo.CreatePrompt(ctx, id.TenantID, p, evs)
		if err != nil {
			return domain.CustomPrompt{}, wrapRepoErr(err, "failed to create prompt")
		}
		return out, nil
	}
	if _, err := uuid.Parse(p.ID); err != nil {
		return domain.CustomPrompt{}, domain.ErrNotFound()
	}
	if p.Version < 1 {
		return domain.CustomPrompt{}, domain.ErrPromptInvalid("version", "is required when updating")
	}
	evs, err := uc.events(id, p.ID, p.Name, "upserted", p.Version+1, domain.AuditActionPromptUpsert, now)
	if err != nil {
		return domain.CustomPrompt{}, domain.ErrInternal("failed to build events", err)
	}
	out, err := uc.repo.UpdatePrompt(ctx, id.TenantID, p, evs)
	if err != nil {
		return domain.CustomPrompt{}, wrapRepoErr(err, "failed to update prompt")
	}
	return out, nil
}

func (uc *PromptAdmin) Delete(ctx context.Context, promptID string) error {
	id, err := userCaller(ctx)
	if err != nil {
		return err
	}
	if err := id.requireAdmin(); err != nil {
		return err
	}
	if _, err := uuid.Parse(promptID); err != nil {
		return domain.ErrNotFound()
	}
	evs, err := uc.events(id, promptID, "", "deleted", 0, domain.AuditActionPromptDelete, uc.clock.Now())
	if err != nil {
		return domain.ErrInternal("failed to build events", err)
	}
	if err := uc.repo.DeletePrompt(ctx, id.TenantID, promptID, evs); err != nil {
		return wrapRepoErr(err, "failed to delete prompt")
	}
	return nil
}

// events returns the list_changed trigger and the admin audit event.
func (uc *PromptAdmin) events(id callerIdentity, promptID, name, action string, version int, auditAction string, now time.Time) ([]domain.OutboxRecord, error) {
	changed, err := domain.NewOutboxEvent(uuid.NewString(), domain.SubjectPromptChanged, id.TenantID, now,
		map[string]any{"prompt_id": promptID, "name": name, "action": action, "version": version})
	if err != nil {
		return nil, err
	}
	audit, err := domain.NewAdminAuditEvent(uuid.NewString(), uuid.NewString(), id.TenantID, id.UserID, auditAction, "mcp_prompt", promptID, "allowed", now,
		map[string]any{"name": name, "version": version})
	if err != nil {
		return nil, err
	}
	return []domain.OutboxRecord{changed, audit}, nil
}
