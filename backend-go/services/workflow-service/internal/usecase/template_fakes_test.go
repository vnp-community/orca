package usecase

import (
	"context"
	"sort"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// fakeTemplateRepository is an in-memory TemplateRepository — never touches
// real Postgres, used by create_template_test.go, list_templates_test.go,
// and resolve_template_test.go.
type fakeTemplateRepository struct {
	templates  map[string]domain.WorkflowTemplate
	getErr     error
	createErr  error
	listErr    error
	resolveErr error
}

func newFakeTemplateRepository() *fakeTemplateRepository {
	return &fakeTemplateRepository{templates: make(map[string]domain.WorkflowTemplate)}
}

func (f *fakeTemplateRepository) CreateTemplate(ctx context.Context, tmpl domain.WorkflowTemplate) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.templates[tmpl.ID] = tmpl
	return nil
}

func (f *fakeTemplateRepository) GetTemplate(ctx context.Context, tenantID, id string) (domain.WorkflowTemplate, error) {
	if f.getErr != nil {
		return domain.WorkflowTemplate{}, f.getErr
	}
	t, ok := f.templates[id]
	if !ok || t.TenantID != tenantID {
		return domain.WorkflowTemplate{}, domain.ErrTemplateNotFound
	}
	return t, nil
}

// ListTemplates mirrors the real repository's keyset-pagination contract
// (id-ordered, cursor = last-seen id) closely enough to exercise
// usecase.ListTemplates without needing real Postgres.
func (f *fakeTemplateRepository) ListTemplates(ctx context.Context, tenantID, scope, pageToken string, pageSize int32) ([]domain.WorkflowTemplate, string, error) {
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	var matched []domain.WorkflowTemplate
	for _, t := range f.templates {
		if t.TenantID != tenantID {
			continue
		}
		if scope != "" && string(t.Scope) != scope {
			continue
		}
		if t.ID <= pageToken {
			continue
		}
		matched = append(matched, t)
	}
	sort.Slice(matched, func(i, j int) bool { return matched[i].ID < matched[j].ID })
	if int32(len(matched)) > pageSize {
		matched = matched[:pageSize]
	}
	next := ""
	if int32(len(matched)) == pageSize && len(matched) > 0 {
		next = matched[len(matched)-1].ID
	}
	return matched, next, nil
}

// ResolveChain mirrors the real repository's root-first ordering
// (index 0 = topmost ancestor, last = templateID itself), depth-capped.
func (f *fakeTemplateRepository) ResolveChain(ctx context.Context, tenantID, templateID string, maxDepth int) ([]domain.WorkflowTemplate, error) {
	if f.resolveErr != nil {
		return nil, f.resolveErr
	}
	var chain []domain.WorkflowTemplate
	cur, ok := f.templates[templateID]
	if !ok || cur.TenantID != tenantID {
		return nil, domain.ErrTemplateNotFound
	}
	for depth := 0; depth <= maxDepth; depth++ {
		// Prepend: each newly-visited node is an ancestor of everything
		// already in chain, so it belongs before them — this walk starts
		// at the requested template and moves outward, but the result
		// must read root-first (see the real repository's ORDER BY depth
		// DESC, which achieves the same order via SQL instead).
		chain = append([]domain.WorkflowTemplate{cur}, chain...)
		if cur.ParentTemplateID == "" {
			break
		}
		parent, ok := f.templates[cur.ParentTemplateID]
		if !ok {
			break
		}
		cur = parent
	}
	return chain, nil
}
