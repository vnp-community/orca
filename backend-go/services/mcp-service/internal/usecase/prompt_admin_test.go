package usecase

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

type memPrompts struct {
	mu     sync.Mutex
	byID   map[string]domain.CustomPrompt
	events []domain.OutboxRecord
}

func newMemPrompts() *memPrompts { return &memPrompts{byID: map[string]domain.CustomPrompt{}} }

func (m *memPrompts) ListPrompts(_ context.Context, tenantID string) ([]domain.CustomPrompt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []domain.CustomPrompt{}
	for _, p := range m.byID {
		out = append(out, p)
	}
	return out, nil
}

func (m *memPrompts) CreatePrompt(_ context.Context, _ string, p domain.CustomPrompt, ev []domain.OutboxRecord) (domain.CustomPrompt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, q := range m.byID {
		if q.Name == p.Name {
			return domain.CustomPrompt{}, domain.ErrPromptNameConflict(p.Name)
		}
	}
	m.byID[p.ID] = p
	m.events = append(m.events, ev...)
	return p, nil
}

func (m *memPrompts) UpdatePrompt(_ context.Context, _ string, p domain.CustomPrompt, ev []domain.OutboxRecord) (domain.CustomPrompt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.byID[p.ID]
	if !ok {
		return domain.CustomPrompt{}, domain.ErrNotFound()
	}
	if cur.Version != p.Version {
		return domain.CustomPrompt{}, domain.ErrPromptVersionConflict(cur.Version)
	}
	p.Version++
	m.byID[p.ID] = p
	m.events = append(m.events, ev...)
	return p, nil
}

func (m *memPrompts) DeletePrompt(_ context.Context, _, id string, ev []domain.OutboxRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.byID[id]; !ok {
		return domain.ErrNotFound()
	}
	delete(m.byID, id)
	m.events = append(m.events, ev...)
	return nil
}

const (
	promptTenant = "11111111-1111-4111-8111-111111111111"
	promptAdmin  = "22222222-2222-4222-8222-222222222222"
)

func validPrompt() domain.CustomPrompt {
	return domain.CustomPrompt{Name: "team_standup", Template: "Summarize {{team}}.",
		Arguments: []domain.PromptArgument{{Name: "team", Required: true}}}
}

func TestPromptAdmin_RolesAndLifecycle(t *testing.T) {
	repo := newMemPrompts()
	uc := NewPromptAdmin(repo, SystemClock{})

	if _, err := uc.Upsert(as(promptTenant, promptAdmin, "user"), validPrompt()); err == nil || !strings.Contains(err.Error(), domain.CodeNotAdmin) {
		t.Fatalf("non-admin upsert must be MCP_NOT_ADMIN, got %v", err)
	}
	if err := uc.Delete(as(promptTenant, promptAdmin, ""), uuid.NewString()); err == nil || !strings.Contains(err.Error(), domain.CodeNotAdmin) {
		t.Fatalf("role-less delete must fail closed, got %v", err)
	}
	admin := as(promptTenant, promptAdmin, domain.RoleAdmin)
	created, err := uc.Upsert(admin, validPrompt())
	if err != nil || created.Version != 1 || created.ID == "" || created.CreatedBy != promptAdmin {
		t.Fatalf("%+v %v", created, err)
	}
	// Ordinary users may list (prompts/list on /mcp).
	if ps, err := uc.List(as(promptTenant, promptAdmin, "user")); err != nil || len(ps) != 1 {
		t.Fatalf("%v %v", ps, err)
	}
	// Optimistic concurrency: stale version loses.
	created.Description = "v2"
	updated, err := uc.Upsert(admin, created)
	if err != nil || updated.Version != 2 {
		t.Fatalf("%+v %v", updated, err)
	}
	if _, err := uc.Upsert(admin, created); err == nil || !strings.Contains(err.Error(), domain.CodePromptVersionConflict) {
		t.Fatalf("stale update must conflict, got %v", err)
	}
	// Name conflict on create.
	if _, err := uc.Upsert(admin, validPrompt()); err == nil || !strings.Contains(err.Error(), domain.CodePromptNameConflict) {
		t.Fatalf("duplicate name must conflict, got %v", err)
	}
	// Validation errors keep the contract format.
	bad := validPrompt()
	bad.Name = "plan_task"
	if _, err := uc.Upsert(admin, bad); err == nil || !strings.HasPrefix(err.Error(), "MCP_PROMPT_INVALID: name: ") {
		t.Fatalf("got %v", err)
	}
	// Update without version, unknown id.
	noVer := updated
	noVer.Version = 0
	if _, err := uc.Upsert(admin, noVer); err == nil || !strings.Contains(err.Error(), "version") {
		t.Fatalf("got %v", err)
	}
	// Events: list_changed trigger + audit for create, update.
	subjects := map[string]int{}
	for _, e := range repo.events {
		subjects[e.Subject]++
	}
	if subjects[domain.SubjectPromptChanged] != 2 || subjects[domain.SubjectAuditAppended] != 2 {
		t.Fatalf("events: %v", subjects)
	}
	if err := uc.Delete(admin, "not-a-uuid"); err == nil || !strings.Contains(err.Error(), domain.CodeNotFound) {
		t.Fatalf("got %v", err)
	}
	if err := uc.Delete(admin, created.ID); err != nil {
		t.Fatal(err)
	}
	if err := uc.Delete(admin, created.ID); err == nil || !strings.Contains(err.Error(), domain.CodeNotFound) {
		t.Fatalf("second delete: %v", err)
	}
}
