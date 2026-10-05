package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

type fakeTaskSourceRepository struct {
	byKey      map[string]string // key -> task id
	bySource   map[string]domain.TaskSource
	linkErr    error
	raceTaskID string // when set, LinkSource pretends this task won the key first
}

func newFakeTaskSourceRepository() *fakeTaskSourceRepository {
	return &fakeTaskSourceRepository{byKey: map[string]string{}, bySource: map[string]domain.TaskSource{}}
}

func sourceKey(tenantID, projectID string, p domain.SourceProvider, site, ref string) string {
	return tenantID + "|" + projectID + "|" + string(p) + "|" + site + "|" + ref
}

func (f *fakeTaskSourceRepository) LinkSource(_ context.Context, src domain.TaskSource) error {
	if f.linkErr != nil {
		return f.linkErr
	}
	k := sourceKey(src.TenantID, src.ProjectID, src.Provider, src.Site, src.Ref)
	if f.raceTaskID != "" {
		f.byKey[k] = f.raceTaskID
		return domain.ErrSourceAlreadyLinked
	}
	if _, taken := f.byKey[k]; taken {
		return domain.ErrSourceAlreadyLinked
	}
	f.byKey[k] = src.TaskID
	f.bySource[src.TaskID] = src
	return nil
}

func (f *fakeTaskSourceRepository) FindTaskIDBySource(_ context.Context, tenantID, projectID string, p domain.SourceProvider, site, ref string) (string, bool, error) {
	if id, ok := f.byKey[sourceKey(tenantID, projectID, p, site, ref)]; ok {
		return id, true, nil
	}
	// Legacy rows stored with site "" match any site.
	id, ok := f.byKey[sourceKey(tenantID, projectID, p, "", ref)]
	return id, ok, nil
}

func (f *fakeTaskSourceRepository) GetSource(_ context.Context, _ string, taskID string) (domain.TaskSource, bool, error) {
	s, ok := f.bySource[taskID]
	return s, ok, nil
}

func newCreateFromSourceUC() (*CreateTaskFromSource, *fakeTaskRepository, *fakeTaskSourceRepository) {
	repo := newFakeTaskRepository()
	sources := newFakeTaskSourceRepository()
	return NewCreateTaskFromSource(repo, sources, NewCreateTask(repo, nil)), repo, sources
}

func sourceInput(ref string) CreateTaskFromSourceInput {
	return CreateTaskFromSourceInput{
		CreateTaskInput: CreateTaskInput{Title: "ENG-1 fix login", ProjectID: "11111111-1111-1111-1111-111111111111"},
		Provider:        "jira", Ref: ref, URL: "https://x.atlassian.net/browse/" + ref,
	}
}

func TestCreateTaskFromSource_CreatesAndLinks(t *testing.T) {
	uc, repo, sources := newCreateFromSourceUC()
	ctx := tenant.WithTenantID(context.Background(), "t1")

	res, err := uc.Execute(ctx, sourceInput("ENG-1"))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !res.Created || res.Task.ID == "" {
		t.Fatalf("want created task, got %+v", res)
	}
	if _, ok := repo.tasks[res.Task.ID]; !ok {
		t.Fatal("task not persisted")
	}
	if src, ok := sources.bySource[res.Task.ID]; !ok || src.Ref != "ENG-1" || src.Provider != domain.SourceProviderJira {
		t.Fatalf("source not linked: %+v", src)
	}
}

func TestCreateTaskFromSource_SecondStartReturnsExisting(t *testing.T) {
	uc, repo, _ := newCreateFromSourceUC()
	ctx := tenant.WithTenantID(context.Background(), "t1")

	first, _ := uc.Execute(ctx, sourceInput("ENG-1"))
	second, err := uc.Execute(ctx, sourceInput("ENG-1"))
	if err != nil {
		t.Fatalf("second Execute: %v", err)
	}
	if second.Created || second.Task.ID != first.Task.ID {
		t.Fatalf("want existing task %s, got %+v", first.Task.ID, second)
	}
	if len(repo.tasks) != 1 {
		t.Fatalf("want 1 task, got %d", len(repo.tasks))
	}
}

func TestCreateTaskFromSource_LostRaceReturnsWinnerAndRemovesOrphan(t *testing.T) {
	uc, repo, sources := newCreateFromSourceUC()
	ctx := tenant.WithTenantID(context.Background(), "t1")
	winner, _ := repo.Create(ctx, domain.Task{ID: "winner", TenantID: "t1", Title: "w"})
	sources.raceTaskID = winner.ID

	res, err := uc.Execute(ctx, sourceInput("ENG-2"))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Created || res.Task.ID != "winner" {
		t.Fatalf("want winner task, got %+v", res)
	}
	if len(repo.tasks) != 1 {
		t.Fatalf("orphan task not removed: %d tasks", len(repo.tasks))
	}
}

func TestCreateTaskFromSource_LinkFailureRemovesTask(t *testing.T) {
	uc, repo, sources := newCreateFromSourceUC()
	ctx := tenant.WithTenantID(context.Background(), "t1")
	sources.linkErr = errors.New("db down")

	if _, err := uc.Execute(ctx, sourceInput("ENG-3")); err == nil {
		t.Fatal("want error")
	}
	if len(repo.tasks) != 0 {
		t.Fatalf("task left behind after link failure: %d", len(repo.tasks))
	}
}

func TestCreateTaskFromSource_RejectsBadInput(t *testing.T) {
	uc, _, _ := newCreateFromSourceUC()
	ctx := tenant.WithTenantID(context.Background(), "t1")

	bad := sourceInput("ENG-4")
	bad.Provider = "bitbucket"
	if _, err := uc.Execute(ctx, bad); err == nil {
		t.Fatal("want error for unknown provider")
	}
	bad = sourceInput("  ")
	if _, err := uc.Execute(ctx, bad); err == nil {
		t.Fatal("want error for empty ref")
	}
}

func TestCreateTaskFromSource_SameKeyDifferentSites_AreDistinctTasks(t *testing.T) {
	uc, _, sources := newCreateFromSourceUC()
	ctx := tenant.WithTenantID(context.Background(), "t1")

	a := sourceInput("ENG-1")
	a.Site = " https://a.atlassian.net "
	b := sourceInput("ENG-1")
	b.Site = "https://b.atlassian.net"
	ra, err := uc.Execute(ctx, a)
	if err != nil || !ra.Created {
		t.Fatalf("site a: created=%v err=%v", ra.Created, err)
	}
	rb, err := uc.Execute(ctx, b)
	if err != nil || !rb.Created || rb.Task.ID == ra.Task.ID {
		t.Fatalf("site b must create its own task: %+v err=%v", rb, err)
	}
	if got := sources.bySource[ra.Task.ID].Site; got != "https://a.atlassian.net" {
		t.Errorf("stored site = %q, want trimmed", got)
	}
	again, err := uc.Execute(ctx, a)
	if err != nil || again.Created || again.Task.ID != ra.Task.ID {
		t.Errorf("repeat on site a must reuse: %+v err=%v", again, err)
	}
}

func TestCreateTaskFromSource_LegacySiteLessRow_IsReusedNotDuplicated(t *testing.T) {
	uc, _, sources := newCreateFromSourceUC()
	ctx := tenant.WithTenantID(context.Background(), "t1")

	legacy, err := uc.Execute(ctx, sourceInput("ENG-1"))
	if err != nil || !legacy.Created {
		t.Fatalf("legacy create: %v", err)
	}
	in := sourceInput("ENG-1")
	in.Site = "https://a.atlassian.net"
	res, err := uc.Execute(ctx, in)
	if err != nil || res.Created || res.Task.ID != legacy.Task.ID {
		t.Fatalf("site-carrying request must reuse legacy row: %+v err=%v", res, err)
	}
	if len(sources.bySource) != 1 {
		t.Errorf("sources = %d, want 1", len(sources.bySource))
	}
}
