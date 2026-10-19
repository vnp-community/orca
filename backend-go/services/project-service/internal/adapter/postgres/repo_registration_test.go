//go:build integration

package postgres

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func keyedRepo(tenantID, projectID, devServerID, url string) domain.Repo {
	return domain.Repo{
		ID: uuid.NewString(), ProjectID: projectID, URL: url, DevServerID: devServerID,
		RegistrationKey: domain.RepoRegistrationKey(tenantID, devServerID, url),
	}
}

func projectInTenant(t *testing.T, repo *Repository, tenantID, name string) domain.Project {
	t.Helper()
	created, err := repo.Create(context.Background(), newTestProject(uuid.NewString(), tenantID, name))
	if err != nil {
		t.Fatalf("creating project: %v", err)
	}
	return created
}

func TestRepoRegistration_KeyedDuplicateIsRejectedAndNullKeysNeverConflict(t *testing.T) {
	pool := setupPool(t)
	projects, repos := New(pool), NewRepoRepository(pool)
	ctx := context.Background()
	tenantID, dev := uuid.NewString(), uuid.NewString()
	p1 := projectInTenant(t, projects, tenantID, "p1")
	p2 := projectInTenant(t, projects, tenantID, "p2")

	if _, err := repos.AddRepo(ctx, keyedRepo(tenantID, p1.ID, dev, "/srv/app")); err != nil {
		t.Fatalf("first registration: %v", err)
	}
	_, err := repos.AddRepo(ctx, keyedRepo(tenantID, p2.ID, dev, "/srv/app/"))
	if !errors.Is(err, domain.ErrRepoAlreadyRegistered) {
		t.Fatalf("same location in another project must be rejected, got %v", err)
	}

	// Legacy rows carry no key: any number of them may coexist.
	for i := 0; i < 3; i++ {
		legacy := domain.Repo{ID: uuid.NewString(), ProjectID: p2.ID, URL: "/srv/legacy", DevServerID: dev}
		if _, err := repos.AddRepo(ctx, legacy); err != nil {
			t.Fatalf("unkeyed row %d: %v", i, err)
		}
	}
}

func TestRepoRegistration_ManyConcurrentRegistrationsExactlyOneWins(t *testing.T) {
	pool := setupPool(t)
	projects, repos := New(pool), NewRepoRepository(pool)
	tenantID, dev := uuid.NewString(), uuid.NewString()
	const workers = 16
	ps := make([]domain.Project, workers)
	for i := range ps {
		ps[i] = projectInTenant(t, projects, tenantID, "racer")
	}

	var ok, conflict, other atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(p domain.Project) {
			defer wg.Done()
			<-start
			_, err := repos.AddRepo(context.Background(), keyedRepo(tenantID, p.ID, dev, "/srv/race"))
			switch {
			case err == nil:
				ok.Add(1)
			case errors.Is(err, domain.ErrRepoAlreadyRegistered):
				conflict.Add(1)
			default:
				other.Add(1)
				t.Errorf("unexpected error: %v", err)
			}
		}(ps[i])
	}
	close(start)
	wg.Wait()

	if ok.Load() != 1 || conflict.Load() != workers-1 || other.Load() != 0 {
		t.Fatalf("ok=%d conflict=%d other=%d, want 1/%d/0", ok.Load(), conflict.Load(), other.Load(), workers-1)
	}
	found, _, err := repos.FindRepoAtLocation(context.Background(), tenantID, dev, "/srv/race", "")
	if err != nil || found.RepoID == "" {
		t.Fatalf("the winner must be findable: %+v %v", found, err)
	}
}

func TestRepoRegistration_ConcurrentCreateWithOwnerLeavesNoLoserProject(t *testing.T) {
	pool := setupPool(t)
	projects := New(pool)
	tenantID, dev, user := uuid.NewString(), uuid.NewString(), uuid.NewString()
	const workers = 12

	var ok atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p := newTestProject(uuid.NewString(), tenantID, "concurrent")
			p.CreatedBy = user
			owner, _ := domain.NewProjectMember(p.ID, user, domain.ProjectRoleOwner)
			r := keyedRepo(tenantID, p.ID, dev, "/srv/atomic")
			<-start
			_, created, err := projects.CreateWithOwner(context.Background(), p, owner, &r)
			if err == nil && created != nil {
				ok.Add(1)
			} else if !errors.Is(err, domain.ErrRepoAlreadyRegistered) {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	close(start)
	wg.Wait()

	if ok.Load() != 1 {
		t.Fatalf("exactly one create may win, got %d", ok.Load())
	}
	var projectCount, repoCount, memberCount int
	_ = pool.QueryRow(context.Background(), `SELECT count(*) FROM project.projects WHERE tenant_id = $1`, tenantID).Scan(&projectCount)
	_ = pool.QueryRow(context.Background(), `SELECT count(*) FROM project.repos r JOIN project.projects p ON p.id = r.project_id WHERE p.tenant_id = $1`, tenantID).Scan(&repoCount)
	_ = pool.QueryRow(context.Background(), `SELECT count(*) FROM project.project_members m JOIN project.projects p ON p.id = m.project_id WHERE p.tenant_id = $1`, tenantID).Scan(&memberCount)
	if projectCount != 1 || repoCount != 1 || memberCount != 1 {
		t.Errorf("losers must roll back completely: projects=%d repos=%d members=%d", projectCount, repoCount, memberCount)
	}
}

func TestCreateWithOwner_WritesProjectOwnerAndRepo(t *testing.T) {
	pool := setupPool(t)
	projects, repos := New(pool), NewRepoRepository(pool)
	ctx := context.Background()
	tenantID, dev, user := uuid.NewString(), uuid.NewString(), uuid.NewString()

	p := newTestProject(uuid.NewString(), tenantID, "own")
	p.CreatedBy, p.DevServerID = user, dev
	owner, _ := domain.NewProjectMember(p.ID, user, domain.ProjectRoleOwner)
	r := keyedRepo(tenantID, p.ID, dev, "/srv/own")

	created, createdRepo, err := projects.CreateWithOwner(ctx, p, owner, &r)
	if err != nil {
		t.Fatal(err)
	}
	if createdRepo == nil || createdRepo.ProjectID != created.ID || createdRepo.DevServerID != dev || createdRepo.Position != 0 {
		t.Fatalf("repo = %+v", createdRepo)
	}
	m, err := projects.GetMembership(ctx, created.ID, user)
	if err != nil || m.Role != domain.ProjectRoleOwner {
		t.Fatalf("owner membership = %+v %v", m, err)
	}
	if list, _ := repos.ListRepos(ctx, created.ID); len(list) != 1 {
		t.Errorf("repos = %d", len(list))
	}

	// Without a repo it still creates project + owner.
	p2 := newTestProject(uuid.NewString(), tenantID, "bare")
	owner2, _ := domain.NewProjectMember(p2.ID, user, domain.ProjectRoleOwner)
	if _, rr, err := projects.CreateWithOwner(ctx, p2, owner2, nil); err != nil || rr != nil {
		t.Fatalf("bare create: repo=%v err=%v", rr, err)
	}
}

func TestFindRepoAtLocation_TenantHostAndSpellingIsolation(t *testing.T) {
	pool := setupPool(t)
	projects, repos := New(pool), NewRepoRepository(pool)
	ctx := context.Background()
	tenantA, tenantB, dev1, dev2 := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	pa := projectInTenant(t, projects, tenantA, "A project")
	pb := projectInTenant(t, projects, tenantB, "B project")

	legacy := domain.Repo{ID: uuid.NewString(), ProjectID: pa.ID, URL: "/srv/codex/", DevServerID: dev1} // unkeyed, odd spelling
	if _, err := repos.AddRepo(ctx, legacy); err != nil {
		t.Fatal(err)
	}
	if _, err := repos.AddRepo(ctx, keyedRepo(tenantB, pb.ID, dev1, "/srv/codex")); err != nil {
		t.Fatal(err)
	}

	got, found, err := repos.FindRepoAtLocation(ctx, tenantA, dev1, "/srv/codex", "")
	if err != nil || !found || got.RepoID != legacy.ID || got.ProjectName != "A project" || got.ProjectID != pa.ID || got.ProjectVisibility != domain.DefaultVisibility {
		t.Fatalf("legacy row with trailing slash must be found: %+v found=%v err=%v", got, found, err)
	}
	if _, found, _ := repos.FindRepoAtLocation(ctx, tenantA, dev2, "/srv/codex", ""); found {
		t.Error("another dev server must not match")
	}
	if _, found, _ := repos.FindRepoAtLocation(ctx, tenantA, dev1, "/srv/codex", legacy.ID); found {
		t.Error("the repo itself is excluded, and tenant B's repo must stay invisible to tenant A")
	}
	if _, found, _ := repos.FindRepoAtLocation(ctx, tenantA, "", "/srv/codex", ""); found {
		t.Error("a repo without a dev server is a different location")
	}
	// Uppercase dev server id still matches the stored lower-case uuid.
	if _, found, _ := repos.FindRepoAtLocation(ctx, tenantA, strings.ToUpper(dev1), "/srv/codex", ""); !found {
		t.Error("dev server id comparison must ignore case")
	}
}

func TestUpdateDevServerIDAndUpdate_RekeyAndConflict(t *testing.T) {
	pool := setupPool(t)
	projects, repos := New(pool), NewRepoRepository(pool)
	ctx := context.Background()
	tenantID, dev1, dev2 := uuid.NewString(), uuid.NewString(), uuid.NewString()
	p := projectInTenant(t, projects, tenantID, "p")

	a, err := repos.AddRepo(ctx, keyedRepo(tenantID, p.ID, dev1, "/srv/a"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repos.AddRepo(ctx, keyedRepo(tenantID, p.ID, dev2, "/srv/a")); err != nil {
		t.Fatal(err)
	}

	_, err = repos.UpdateDevServerID(ctx, a.ID, dev2, domain.RepoRegistrationKey(tenantID, dev2, "/srv/a"))
	if !errors.Is(err, domain.ErrRepoAlreadyRegistered) {
		t.Fatalf("moving onto an occupied location must conflict, got %v", err)
	}
	moved, err := repos.UpdateDevServerID(ctx, a.ID, dev1, a.RegistrationKey)
	if err != nil || moved.RegistrationKey != a.RegistrationKey {
		t.Fatalf("same-location write must pass: %+v %v", moved, err)
	}

	a.URL = "/srv/b"
	a.RegistrationKey = domain.RepoRegistrationKey(tenantID, dev1, "/srv/b")
	updated, err := repos.Update(ctx, a)
	if err != nil || updated.RegistrationKey != a.RegistrationKey || updated.URL != "/srv/b" {
		t.Fatalf("update with a new key: %+v %v", updated, err)
	}
}

func TestImportNested_Postgres_CreatesProjectsAndRejectsRegisteredPaths(t *testing.T) {
	pool := setupPool(t)
	repos, groups := NewRepoRepository(pool), NewProjectGroupRepository(pool)
	ctx := context.Background()
	tenantID, dev, user := uuid.NewString(), uuid.NewString(), uuid.NewString()

	gs, ps, err := groups.ImportNested(ctx, tenantID, user, dev, "", []domain.NestedRepoCandidate{
		{Path: "/home/dev/one", SuggestedName: "one"}, {Path: "/home/dev/two", SuggestedName: "two"},
	})
	if err != nil || len(gs) != 2 || len(ps) != 2 {
		t.Fatalf("import: groups=%d projects=%d err=%v", len(gs), len(ps), err)
	}
	if ps[0].TenantID != tenantID || ps[0].Name != "one" || ps[0].DevServerID != dev {
		t.Errorf("project = %+v", ps[0])
	}
	list, _ := repos.ListRepos(ctx, ps[0].ID)
	if len(list) != 1 || list[0].URL != "/home/dev/one" || list[0].RegistrationKey != domain.RepoRegistrationKey(tenantID, dev, "/home/dev/one") {
		t.Fatalf("repo of the imported project = %+v", list)
	}

	before := countProjects(t, pool, tenantID)
	_, _, err = groups.ImportNested(ctx, tenantID, user, dev, "", []domain.NestedRepoCandidate{
		{Path: "/home/dev/three", SuggestedName: "three"}, {Path: "/home/dev/two/", SuggestedName: "two again"},
	})
	if !errors.Is(err, domain.ErrRepoAlreadyRegistered) {
		t.Fatalf("a registered path must fail the whole import, got %v", err)
	}
	if after := countProjects(t, pool, tenantID); after != before {
		t.Errorf("failed import must roll back: projects %d -> %d", before, after)
	}
}

func countProjects(t *testing.T, pool *pgxpool.Pool, tenantID string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM project.projects WHERE tenant_id = $1`, tenantID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// Migration 0032 must not touch rows that already duplicate each other.
func TestMigration0032_LeavesExistingDuplicatesAloneAndIsReversible(t *testing.T) {
	dsn := testutil.StartPostgres(t, "project")
	migrationsPath, err := filepath.Abs("../../../migrations/postgres")
	if err != nil {
		t.Fatal(err)
	}
	migrate := func(args ...string) {
		t.Helper()
		var out []byte
		var runErr error
		for attempt := 0; attempt < 5; attempt++ {
			out, runErr = exec.Command("migrate", append([]string{"-path", migrationsPath, "-database", dsn}, args...)...).CombinedOutput()
			if runErr == nil || !strings.Contains(string(out), "the database system is starting up") {
				break
			}
			time.Sleep(time.Second)
		}
		if runErr != nil {
			t.Fatalf("migrate %v: %v\n%s", args, runErr, out)
		}
	}
	migrate("goto", "31")

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tenantID, dev := uuid.NewString(), uuid.NewString()
	p1, p2 := uuid.NewString(), uuid.NewString()
	for _, id := range []string{p1, p2} {
		if _, err := pool.Exec(ctx, `INSERT INTO project.projects (id, tenant_id, name, default_branch, visibility) VALUES ($1, $2, 'codex', 'main', 'private')`, id, tenantID); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO project.repos (id, project_id, url, display_name, dev_server_id) VALUES ($1, $2, '/Users/x/codex', 'codex', $3)`, uuid.NewString(), id, dev); err != nil {
			t.Fatal(err)
		}
	}

	migrate("up")
	var rows, keyed int
	_ = pool.QueryRow(ctx, `SELECT count(*), count(registration_key) FROM project.repos`).Scan(&rows, &keyed)
	if rows != 2 || keyed != 0 {
		t.Fatalf("after up: rows=%d keyed=%d, want 2 duplicates left untouched and unkeyed", rows, keyed)
	}
	repos := NewRepoRepository(pool)
	if _, err := repos.AddRepo(ctx, keyedRepo(tenantID, p1, dev, "/Users/x/other")); err != nil {
		t.Fatalf("new keyed registration after migration: %v", err)
	}

	migrate("down", "1")
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM project.repos`).Scan(&rows)
	if rows != 3 {
		t.Fatalf("down must keep every row, got %d", rows)
	}
	var hasColumn bool
	_ = pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema='project' AND table_name='repos' AND column_name='registration_key')`).Scan(&hasColumn)
	if hasColumn {
		t.Error("down must drop registration_key")
	}
	migrate("up")
}
