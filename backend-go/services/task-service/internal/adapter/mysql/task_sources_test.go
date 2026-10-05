//go:build integration

package mysql

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func newSourcedTask(t *testing.T, repo *Repository, tenantID, projectID string) string {
	t.Helper()
	task, err := domain.NewTask(uuid.NewString(), tenantID, "ENG-1", domain.StatusOpen, "", projectID)
	if err != nil {
		t.Fatalf("building task: %v", err)
	}
	if _, err := repo.Create(context.Background(), task); err != nil {
		t.Fatalf("creating task: %v", err)
	}
	return task.ID
}

func TestTaskSources_LinkFindGet_RoundTrips(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()
	projectID := uuid.NewString()
	taskID := newSourcedTask(t, repo, tenantID, projectID)

	src := domain.TaskSource{TaskID: taskID, TenantID: tenantID, ProjectID: projectID, Provider: domain.SourceProviderJira, Ref: "ENG-1", URL: "https://x/browse/ENG-1"}
	if err := repo.LinkSource(ctx, src); err != nil {
		t.Fatalf("LinkSource: %v", err)
	}

	got, ok, err := repo.FindTaskIDBySource(ctx, tenantID, projectID, domain.SourceProviderJira, "", "ENG-1")
	if err != nil || !ok || got != taskID {
		t.Fatalf("FindTaskIDBySource = %q, %v, %v; want %q", got, ok, err, taskID)
	}
	if _, ok, _ := repo.FindTaskIDBySource(ctx, tenantID, uuid.NewString(), domain.SourceProviderJira, "", "ENG-1"); ok {
		t.Error("same ref in a different project must not match")
	}

	gs, ok, err := repo.GetSource(ctx, tenantID, taskID)
	if err != nil || !ok || gs.Ref != "ENG-1" || gs.Provider != domain.SourceProviderJira || gs.ProjectID != projectID {
		t.Fatalf("GetSource = %+v, %v, %v", gs, ok, err)
	}
}

func TestTaskSources_SecondTaskSameIssue_ReturnsErrSourceAlreadyLinked(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()
	projectID := uuid.NewString()
	first := newSourcedTask(t, repo, tenantID, projectID)
	second := newSourcedTask(t, repo, tenantID, projectID)

	mk := func(taskID string) domain.TaskSource {
		return domain.TaskSource{TaskID: taskID, TenantID: tenantID, ProjectID: projectID, Provider: domain.SourceProviderJira, Ref: "ENG-1"}
	}
	if err := repo.LinkSource(ctx, mk(first)); err != nil {
		t.Fatalf("first LinkSource: %v", err)
	}
	if err := repo.LinkSource(ctx, mk(second)); !errors.Is(err, domain.ErrSourceAlreadyLinked) {
		t.Fatalf("want ErrSourceAlreadyLinked, got %v", err)
	}
}

func TestTaskSources_NoProject_StillDeduplicates(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()
	first := newSourcedTask(t, repo, tenantID, "")
	second := newSourcedTask(t, repo, tenantID, "")

	mk := func(taskID string) domain.TaskSource {
		return domain.TaskSource{TaskID: taskID, TenantID: tenantID, Provider: domain.SourceProviderLinear, Ref: "ENG-9"}
	}
	if err := repo.LinkSource(ctx, mk(first)); err != nil {
		t.Fatalf("first LinkSource: %v", err)
	}
	if err := repo.LinkSource(ctx, mk(second)); !errors.Is(err, domain.ErrSourceAlreadyLinked) {
		t.Fatalf("NULL project_id must still collide; got %v", err)
	}
	if got, ok, _ := repo.FindTaskIDBySource(ctx, tenantID, "", domain.SourceProviderLinear, "", "ENG-9"); !ok || got != first {
		t.Errorf("FindTaskIDBySource(no project) = %q, %v; want %q", got, ok, first)
	}
}

func TestTaskSources_GetSource_TaskWithoutSource(t *testing.T) {
	repo := setupRepository(t)
	tenantID := uuid.NewString()
	taskID := newSourcedTask(t, repo, tenantID, "")
	if _, ok, err := repo.GetSource(context.Background(), tenantID, taskID); err != nil || ok {
		t.Fatalf("want (not found, nil), got ok=%v err=%v", ok, err)
	}
}

func TestTaskSources_SameKeyDifferentSites_CoexistAndRoundTripSite(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()
	projectID := uuid.NewString()
	a := newSourcedTask(t, repo, tenantID, projectID)
	b := newSourcedTask(t, repo, tenantID, projectID)
	dup := newSourcedTask(t, repo, tenantID, projectID)

	mk := func(taskID, site string) domain.TaskSource {
		return domain.TaskSource{TaskID: taskID, TenantID: tenantID, ProjectID: projectID, Provider: domain.SourceProviderJira, Ref: "ENG-1", Site: site}
	}
	if err := repo.LinkSource(ctx, mk(a, "https://a.atlassian.net")); err != nil {
		t.Fatalf("site a: %v", err)
	}
	if err := repo.LinkSource(ctx, mk(b, "https://b.atlassian.net")); err != nil {
		t.Fatalf("site b must not collide with site a: %v", err)
	}
	if err := repo.LinkSource(ctx, mk(dup, "https://a.atlassian.net")); !errors.Is(err, domain.ErrSourceAlreadyLinked) {
		t.Fatalf("same site+ref must collide, got %v", err)
	}
	if got, ok, _ := repo.FindTaskIDBySource(ctx, tenantID, projectID, domain.SourceProviderJira, "https://b.atlassian.net", "ENG-1"); !ok || got != b {
		t.Errorf("find site b = %q, %v; want %q", got, ok, b)
	}
	if _, ok, _ := repo.FindTaskIDBySource(ctx, tenantID, projectID, domain.SourceProviderJira, "https://c.atlassian.net", "ENG-1"); ok {
		t.Error("unknown site must not match site-specific rows")
	}
	if gs, ok, _ := repo.GetSource(ctx, tenantID, a); !ok || gs.Site != "https://a.atlassian.net" {
		t.Errorf("GetSource site = %+v", gs)
	}
}

func TestTaskSources_LegacyEmptySiteRow_MatchedByAnySite(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()
	projectID := uuid.NewString()
	legacy := newSourcedTask(t, repo, tenantID, projectID)
	exact := newSourcedTask(t, repo, tenantID, projectID)

	mk := func(taskID, site string) domain.TaskSource {
		return domain.TaskSource{TaskID: taskID, TenantID: tenantID, ProjectID: projectID, Provider: domain.SourceProviderJira, Ref: "ENG-1", Site: site}
	}
	if err := repo.LinkSource(ctx, mk(legacy, "")); err != nil {
		t.Fatalf("legacy: %v", err)
	}
	if got, ok, _ := repo.FindTaskIDBySource(ctx, tenantID, projectID, domain.SourceProviderJira, "https://a.atlassian.net", "ENG-1"); !ok || got != legacy {
		t.Errorf("legacy row must match any site: %q, %v", got, ok)
	}
	if err := repo.LinkSource(ctx, mk(exact, "https://a.atlassian.net")); err != nil {
		t.Fatalf("exact: %v", err)
	}
	if got, _, _ := repo.FindTaskIDBySource(ctx, tenantID, projectID, domain.SourceProviderJira, "https://a.atlassian.net", "ENG-1"); got != exact {
		t.Errorf("exact site must win over legacy: got %q want %q", got, exact)
	}
}
