package usecase

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type memProfiles struct {
	mu   sync.Mutex
	rows map[string]StoredDrawingProfile
}

func (m *memProfiles) Get(_ context.Context, key string) (StoredDrawingProfile, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.rows[key]
	return p, ok, nil
}
func (m *memProfiles) Save(_ context.Context, p StoredDrawingProfile, expected int64) (StoredDrawingProfile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.rows[p.ProjectKey]
	switch {
	case expected == 0 && ok, expected != 0 && (!ok || cur.Version != expected):
		return StoredDrawingProfile{}, domain.ErrReviewStaleRevision()
	}
	p.Version, p.UpdatedAt, p.ID = expected+1, time.Now().UTC(), uuid.NewString()
	m.rows[p.ProjectKey] = p
	return p, nil
}
func (m *memProfiles) Delete(_ context.Context, key string, expected int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cur, ok := m.rows[key]; !ok || cur.Version != expected {
		return domain.ErrReviewStaleRevision()
	}
	delete(m.rows, key)
	return nil
}

func TestDrawingProfiles_ProjectOverTenantOverDefault(t *testing.T) {
	ctx := tenant.WithTenantID(context.Background(), uuid.NewString())
	audit := &reviewAuditLog{}
	svc := &ReviewDrawingProfiles{Repo: &memProfiles{rows: map[string]StoredDrawingProfile{}}, Audit: audit}
	project, person := uuid.NewString(), uuid.NewString()

	v, err := svc.Get(ctx, project)
	if err != nil || v.Tenant != nil || v.Project != nil || v.Effective.Style.Detail != "balanced" || len(v.Effective.Sources) != 0 {
		t.Fatalf("no layers is orca-default: %+v %v", v, err)
	}
	tenantLayer, err := svc.Save(ctx, "", []byte(`{"schema":"review-drawing-profile/1","style":{"detail":"faithful","audience":"engineer"},"language":"vi"}`), 0, person)
	if err != nil || tenantLayer.Version != 1 {
		t.Fatalf("tenant layer: %+v %v", tenantLayer, err)
	}
	if _, err := svc.Save(ctx, project, []byte(`{"schema":"review-drawing-profile/1","style":{"detail":"simplified"},"sections":{"flow":{"maxLanes":4}}}`), 0, person); err != nil {
		t.Fatal(err)
	}
	v, _ = svc.Get(ctx, project)
	if v.Effective.Style.Detail != "simplified" || v.Effective.Sources["style.detail"] != domain.SourceProject ||
		v.Effective.Style.Audience != "engineer" || v.Effective.Sources["style.audience"] != domain.SourceTenant || v.Effective.Language != "vi" {
		t.Errorf("effective: %+v %v", v.Effective, v.Effective.Sources)
	}
	other, _ := svc.Resolve(ctx, uuid.NewString())
	if other.Style.Detail != "faithful" || other.Sections[domain.SectionFlow].MaxNodes != domain.MaxDiagramLanes {
		t.Errorf("another project follows the tenant: %+v", other.Style)
	}

	if _, err := svc.Save(ctx, project, []byte(`{"schema":"review-drawing-profile/1","style":{"roles":{"accent":"#ff0000"}}}`), 1, person); err == nil || !strings.Contains(err.Error(), "style.roles.accent") {
		t.Errorf("an invalid profile names the field: %v", err)
	}
	if _, err := svc.Save(ctx, project, []byte(`{"schema":"review-drawing-profile/1"}`), 1, person); err != nil {
		t.Fatalf("replace under the version: %v", err)
	}
	if _, err := svc.Save(ctx, project, []byte(`{"schema":"review-drawing-profile/1"}`), 1, person); !codeIs(err, domain.CodeReviewStaleRevision) {
		t.Errorf("the second writer on the old version loses: %v", err)
	}
	if err := svc.Reset(ctx, project, 2); err != nil {
		t.Fatal(err)
	}
	if v, _ = svc.Get(ctx, project); v.Project != nil {
		t.Error("reset removes the layer")
	}

	for _, ev := range audit.events {
		for k, val := range ev.Metadata {
			if s, ok := val.(string); ok && (strings.Contains(s, "faithful") || strings.Contains(s, "simplified") || strings.Contains(s, "#ff0000")) {
				t.Errorf("the audit holds the profile: %s=%s", k, s)
			}
		}
	}
	if len(audit.events) != 4 || audit.events[0].Action != domain.AuditReviewProfile || audit.events[0].Metadata["new_digest"] == "" {
		t.Errorf("audit: %+v", audit.events)
	}
	if _, err := svc.Get(context.Background(), project); err == nil {
		t.Error("a call without a tenant fails")
	}
}

func TestDrawingProfiles_AStoredLayerThatNoLongerPassesIsIgnoredNotFatal(t *testing.T) {
	ctx := tenant.WithTenantID(context.Background(), uuid.NewString())
	repo := &memProfiles{rows: map[string]StoredDrawingProfile{"": {ProjectKey: "", Spec: []byte(`{"schema":"review-drawing-profile/1","style":{"variant":"full"}}`), Version: 3}}}
	svc := &ReviewDrawingProfiles{Repo: repo}
	v, err := svc.Get(ctx, "")
	if err != nil || v.Tenant == nil || v.Effective.Style.Variant != "minimal" {
		t.Errorf("drawing keeps working with the default: %+v %v", v.Effective.Style, err)
	}
}
