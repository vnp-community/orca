//go:build integration

package mysql

import (
	"context"
	"database/sql"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"

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
	db := setupDB(t)
	projects, repos := New(db), NewRepoRepository(db)
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

	for i := 0; i < 3; i++ {
		legacy := domain.Repo{ID: uuid.NewString(), ProjectID: p2.ID, URL: "/srv/legacy", DevServerID: dev}
		if _, err := repos.AddRepo(ctx, legacy); err != nil {
			t.Fatalf("unkeyed row %d: %v", i, err)
		}
	}
}

func TestRepoRegistration_ManyConcurrentRegistrationsExactlyOneWins(t *testing.T) {
	db := setupDB(t)
	projects, repos := New(db), NewRepoRepository(db)
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
}

func TestRepoRegistration_ConcurrentCreateWithOwnerLeavesNoLoserProject(t *testing.T) {
	db := setupDB(t)
	projects := New(db)
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
	_ = db.QueryRow(`SELECT count(*) FROM projects WHERE tenant_id = ?`, tenantID).Scan(&projectCount)
	_ = db.QueryRow(`SELECT count(*) FROM repos r JOIN projects p ON p.id = r.project_id WHERE p.tenant_id = ?`, tenantID).Scan(&repoCount)
	_ = db.QueryRow(`SELECT count(*) FROM project_members m JOIN projects p ON p.id = m.project_id WHERE p.tenant_id = ?`, tenantID).Scan(&memberCount)
	if projectCount != 1 || repoCount != 1 || memberCount != 1 {
		t.Errorf("losers must roll back completely: projects=%d repos=%d members=%d", projectCount, repoCount, memberCount)
	}
}

func TestCreateWithOwner_WritesProjectOwnerAndRepo(t *testing.T) {
	db := setupDB(t)
	projects, repos := New(db), NewRepoRepository(db)
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
	if createdRepo == nil || createdRepo.ProjectID != created.ID || createdRepo.DevServerID != dev || createdRepo.Position != 0 || createdRepo.RegistrationKey != r.RegistrationKey {
		t.Fatalf("repo = %+v", createdRepo)
	}
	m, err := projects.GetMembership(ctx, created.ID, user)
	if err != nil || m.Role != domain.ProjectRoleOwner {
		t.Fatalf("owner membership = %+v %v", m, err)
	}
	if list, _ := repos.ListRepos(ctx, created.ID); len(list) != 1 {
		t.Errorf("repos = %d", len(list))
	}

	p2 := newTestProject(uuid.NewString(), tenantID, "bare")
	owner2, _ := domain.NewProjectMember(p2.ID, user, domain.ProjectRoleOwner)
	if _, rr, err := projects.CreateWithOwner(ctx, p2, owner2, nil); err != nil || rr != nil {
		t.Fatalf("bare create: repo=%v err=%v", rr, err)
	}
}

func TestFindRepoAtLocation_TenantHostAndSpellingIsolation(t *testing.T) {
	db := setupDB(t)
	projects, repos := New(db), NewRepoRepository(db)
	ctx := context.Background()
	tenantA, tenantB, dev1, dev2 := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	pa := projectInTenant(t, projects, tenantA, "A project")
	pb := projectInTenant(t, projects, tenantB, "B project")

	legacy := domain.Repo{ID: uuid.NewString(), ProjectID: pa.ID, URL: "/srv/codex/", DevServerID: dev1}
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
		t.Error("the repo itself is excluded, and tenant B's repo must stay invisible to tenant A (MySQL has no RLS)")
	}
	if _, found, _ := repos.FindRepoAtLocation(ctx, tenantA, "", "/srv/codex", ""); found {
		t.Error("a repo without a dev server is a different location")
	}
	if _, found, _ := repos.FindRepoAtLocation(ctx, tenantA, strings.ToUpper(dev1), "/srv/codex", ""); !found {
		t.Error("dev server id comparison must ignore case")
	}
}

func TestUpdateDevServerIDAndUpdate_RekeyAndConflict(t *testing.T) {
	db := setupDB(t)
	projects, repos := New(db), NewRepoRepository(db)
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

func TestImportNested_MySQL_CreatesProjectsAndRejectsRegisteredPaths(t *testing.T) {
	db := setupDB(t)
	repos, groups := NewRepoRepository(db), NewProjectGroupRepository(db)
	ctx := context.Background()
	tenantID, dev, user := uuid.NewString(), uuid.NewString(), uuid.NewString()

	gs, ps, err := groups.ImportNested(ctx, tenantID, user, dev, "", []domain.NestedRepoCandidate{
		{Path: "/home/dev/one", SuggestedName: "one"}, {Path: "/home/dev/two", SuggestedName: "two"},
	})
	if err != nil || len(gs) != 2 || len(ps) != 2 {
		t.Fatalf("import: groups=%d projects=%d err=%v", len(gs), len(ps), err)
	}
	list, _ := repos.ListRepos(ctx, ps[0].ID)
	if len(list) != 1 || list[0].URL != "/home/dev/one" || list[0].RegistrationKey != domain.RepoRegistrationKey(tenantID, dev, "/home/dev/one") {
		t.Fatalf("repo of the imported project = %+v", list)
	}

	count := func() int {
		var n int
		if err := db.QueryRow(`SELECT count(*) FROM projects WHERE tenant_id = ?`, tenantID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	before := count()
	_, _, err = groups.ImportNested(ctx, tenantID, user, dev, "", []domain.NestedRepoCandidate{
		{Path: "/home/dev/three", SuggestedName: "three"}, {Path: "/home/dev/two/", SuggestedName: "two again"},
	})
	if !errors.Is(err, domain.ErrRepoAlreadyRegistered) {
		t.Fatalf("a registered path must fail the whole import, got %v", err)
	}
	if after := count(); after != before {
		t.Errorf("failed import must roll back: projects %d -> %d", before, after)
	}
}

// Migration 0032 must not touch rows that already duplicate each other.
func TestMigration0032_LeavesExistingDuplicatesAloneAndIsReversible(t *testing.T) {
	rawDSN := testutil.StartMySQL(t, "project")
	driverDSN := strings.TrimPrefix(rawDSN, "mysql://") + "?parseTime=true"
	migrationsPath, err := filepath.Abs("../../../migrations/mysql")
	if err != nil {
		t.Fatal(err)
	}
	migrate := func(args ...string) {
		t.Helper()
		out, err := exec.Command("migrate", append([]string{"-path", migrationsPath, "-database", rawDSN}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("migrate %v: %v\n%s", args, err, out)
		}
	}
	migrate("goto", "31")

	db, err := sql.Open("mysql", driverDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	tenantID, dev := uuid.NewString(), uuid.NewString()
	p1, p2 := uuid.NewString(), uuid.NewString()
	for _, id := range []string{p1, p2} {
		if _, err := db.ExecContext(ctx, `INSERT INTO projects (id, tenant_id, name, default_branch, visibility) VALUES (?, ?, 'codex', 'main', 'private')`, id, tenantID); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO repos (id, project_id, url, display_name, dev_server_id) VALUES (?, ?, '/Users/x/codex', 'codex', ?)`, uuid.NewString(), id, dev); err != nil {
			t.Fatal(err)
		}
	}

	migrate("up")
	var rows, keyed int
	_ = db.QueryRow(`SELECT count(*), count(registration_key) FROM repos`).Scan(&rows, &keyed)
	if rows != 2 || keyed != 0 {
		t.Fatalf("after up: rows=%d keyed=%d, want 2 duplicates left untouched and unkeyed", rows, keyed)
	}
	repos := NewRepoRepository(db)
	if _, err := repos.AddRepo(ctx, keyedRepo(tenantID, p1, dev, "/Users/x/other")); err != nil {
		t.Fatalf("new keyed registration after migration: %v", err)
	}

	migrate("down", "1")
	_ = db.QueryRow(`SELECT count(*) FROM repos`).Scan(&rows)
	if rows != 3 {
		t.Fatalf("down must keep every row, got %d", rows)
	}
	var cols int
	_ = db.QueryRow(`SELECT count(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'repos' AND column_name = 'registration_key'`).Scan(&cols)
	if cols != 0 {
		t.Error("down must drop registration_key")
	}
	migrate("up")
}
