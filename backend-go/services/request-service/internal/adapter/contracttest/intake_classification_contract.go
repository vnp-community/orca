package contracttest

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// IntakeEnv adds what the intake and classification scenarios need beyond the repository Env.
// The hooks reach the database below the adapters (admin connection) so assertions do not
// trust the code under test.
type IntakeEnv struct {
	Env
	Outbox    usecase.OutboxWriter
	Processed usecase.ProcessedEventRepository
	Returns   usecase.ReturnHistoryRepository
	Runs      usecase.ClassificationRunRepository
	// OutboxSubjects lists the tenant's outbox subjects in insertion (seq) order.
	OutboxSubjects func(t *testing.T, tenantID string) []string
	// CountRows counts a table's rows for the tenant (idempotency, runs, ...).
	CountRows func(t *testing.T, table, tenantID string) int
	// NextNumberPeek reads request_counters.next_number for the tenant (0 when no row).
	NextNumberPeek func(t *testing.T, tenantID string) int64
	// SetRunLease forces a run's lease expiry.
	SetRunLease func(t *testing.T, runID string, expires time.Time)
}

// realTransitioner is the production state machine over the dialect's repositories.
func realTransitioner(env IntakeEnv) usecase.RequestTransitioner {
	return usecase.NewTransitionRequest(env.Requests, env.Tx.(usecase.TxScope), env.Outbox)
}

// failingTransitioner makes the last step of a multi-step write fail, to prove nothing stays behind.
type failingTransitioner struct{ err error }

func (f failingTransitioner) Execute(context.Context, usecase.TransitionInput) (usecase.TransitionResult, error) {
	return usecase.TransitionResult{}, f.err
}

type scriptedIssues struct {
	mu    sync.Mutex
	calls int
	snap  usecase.IssueSnapshot
	err   error
}

func (s *scriptedIssues) GetIssue(context.Context, domain.SourceProvider, string, string) (usecase.IssueSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	return s.snap, s.err
}

func userCtx(tenantID string) context.Context {
	return tenant.WithUserID(CtxForTenant(tenantID), uuid.NewString())
}

// RunIntakeContract runs the CR-REQ-004 scenarios on the dialect's real database.
func RunIntakeContract(t *testing.T, newEnv func(t *testing.T) IntakeEnv) {
	env := newEnv(t)
	scenarios := []struct {
		name string
		fn   func(t *testing.T, env IntakeEnv)
	}{
		{"Concurrent12SameKey", intakeConcurrent12SameKey},
		{"JiraKeyCaseInsensitive", intakeJiraKeyCaseInsensitive},
		{"TwoJiraSitesTwoRequests", intakeTwoJiraSites},
		{"EnrichFromIssue", intakeEnrichFromIssue},
		{"GitHubMissingTitleNoExternalCall", intakeGitHubMissingTitle},
		{"StatusClassifyingAndTwoEventsInOrder", intakeStatusAndEventOrder},
		{"FailureMidwayLeavesNothing", intakeFailureMidway},
		{"CancelledRequestReturnedUnchanged", intakeCancelledReturnedUnchanged},
		{"TenantIsolationOfKeys", intakeTenantIsolationOfKeys},
		{"SourceHintsAndSourceFilter", intakeSourceHintsAndFilter},
		{"LookupBySource", intakeLookupBySource},
	}
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) { sc.fn(t, env) })
	}
}

func newCreate(env IntakeEnv, issues usecase.IssueFetcher, tr usecase.RequestTransitioner) *usecase.CreateRequest {
	if tr == nil {
		tr = realTransitioner(env)
	}
	return usecase.NewCreateRequest(env.Requests, env.Idempotency, env.Tx, env.Outbox, tr, issues)
}

func jiraIn(project, ref, site, title string) usecase.CreateRequestInput {
	return usecase.CreateRequestInput{ProjectID: project, Title: title, Source: domain.SourceRef{Provider: domain.SourceProviderJira, Ref: ref, Site: site}}
}

func intakeConcurrent12SameKey(t *testing.T, env IntakeEnv) {
	tenantID := newTenant()
	ctx := userCtx(tenantID)
	uc := newCreate(env, &scriptedIssues{}, nil)
	in := jiraIn(uuid.NewString(), "eng-1", "https://acme.atlassian.net", "same issue")
	const n = 12
	results := make([]usecase.CreateRequestResult, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = retryDeadlock(func() error {
				var err error
				results[i], err = uc.Execute(ctx, in)
				return err
			})
		}(i)
	}
	wg.Wait()
	created := 0
	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Fatalf("call %d: %v", i, errs[i])
		}
		if results[i].Created {
			created++
		}
		if results[i].Request.ID != results[0].Request.ID {
			t.Fatalf("call %d saw another request", i)
		}
	}
	if created != 1 {
		t.Fatalf("%d callers created, want exactly 1", created)
	}
	subjects := env.OutboxSubjects(t, tenantID)
	createdEvents := 0
	for _, s := range subjects {
		if s == domain.SubjectRequestCreated {
			createdEvents++
		}
	}
	if createdEvents != 1 || len(subjects) != 2 {
		t.Fatalf("outbox = %v", subjects)
	}
	if got := env.CountRows(t, "requests", tenantID); got != 1 {
		t.Fatalf("%d requests", got)
	}
	if peek := env.NextNumberPeek(t, tenantID); peek != 1 || results[0].Request.Number != 1 {
		t.Fatalf("number burnt by losers: counter=%d number=%d", peek, results[0].Request.Number)
	}
}

func intakeJiraKeyCaseInsensitive(t *testing.T, env IntakeEnv) {
	ctx := userCtx(newTenant())
	uc := newCreate(env, &scriptedIssues{}, nil)
	project := uuid.NewString()
	a, err := uc.Execute(ctx, jiraIn(project, "eng-1", "https://Acme.atlassian.net/", "t"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := uc.Execute(ctx, jiraIn(project, "ENG-1", "https://acme.atlassian.net", "t"))
	if err != nil || b.Created || b.Request.ID != a.Request.ID {
		t.Fatalf("%+v %v", b, err)
	}
}

func intakeTwoJiraSites(t *testing.T, env IntakeEnv) {
	ctx := userCtx(newTenant())
	uc := newCreate(env, &scriptedIssues{}, nil)
	project := uuid.NewString()
	a, _ := uc.Execute(ctx, jiraIn(project, "ENG-1", "https://a.atlassian.net", "t"))
	b, err := uc.Execute(ctx, jiraIn(project, "ENG-1", "https://b.atlassian.net", "t"))
	if err != nil || !b.Created || a.Request.ID == b.Request.ID {
		t.Fatalf("different sites must not share a request: %+v %v", b, err)
	}
}

func intakeEnrichFromIssue(t *testing.T, env IntakeEnv) {
	ctx := userCtx(newTenant())
	issues := &scriptedIssues{snap: usecase.IssueSnapshot{Title: "From Jira", Body: "desc", URL: "https://j/1", Hints: domain.SourceHints{IssueType: "Bug", Labels: []string{"x"}}}}
	uc := newCreate(env, issues, nil)
	res, err := uc.Execute(ctx, jiraIn(uuid.NewString(), "ENG-2", "", ""))
	if err != nil || res.Request.Title != "From Jira" {
		t.Fatalf("%+v %v", res, err)
	}
	got, _ := env.Requests.Get(ctx, res.Request.ID)
	if got.Body != "desc" || got.SourceURL != "https://j/1" || got.SourceHints.IssueType != "Bug" {
		t.Fatalf("enrichment not persisted: %+v", got)
	}
	issues.err = errors.New("jira down")
	res, err = uc.Execute(ctx, jiraIn(uuid.NewString(), "ENG-3", "", "typed title"))
	if err != nil || !res.Created {
		t.Fatalf("enrichment failure with a title must still create: %+v %v", res, err)
	}
}

func intakeGitHubMissingTitle(t *testing.T, env IntakeEnv) {
	issues := &scriptedIssues{}
	uc := newCreate(env, issues, nil)
	_, err := uc.Execute(userCtx(newTenant()), usecase.CreateRequestInput{ProjectID: uuid.NewString(), Source: domain.SourceRef{Provider: domain.SourceProviderGithub, Ref: "o/r#1"}})
	requireCode(t, err, "REQUEST_TITLE_REQUIRED")
	if issues.calls != 0 {
		t.Fatal("github intake must not call the issue tracker")
	}
}

func intakeStatusAndEventOrder(t *testing.T, env IntakeEnv) {
	tenantID := newTenant()
	res, err := newCreate(env, &scriptedIssues{}, nil).Execute(userCtx(tenantID), usecase.CreateRequestInput{
		ProjectID: uuid.NewString(), Title: "t", Source: domain.SourceRef{Provider: domain.SourceProviderManual}})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := env.Requests.Get(CtxForTenant(tenantID), res.Request.ID)
	if got.Status != domain.RequestStatusClassifying {
		t.Fatalf("status = %s", got.Status)
	}
	subjects := env.OutboxSubjects(t, tenantID)
	if len(subjects) != 2 || subjects[0] != domain.SubjectRequestCreated || subjects[1] != domain.SubjectRequestStatusChanged {
		t.Fatalf("outbox order = %v", subjects)
	}
}

func intakeFailureMidway(t *testing.T, env IntakeEnv) {
	for name, tr := range map[string]usecase.RequestTransitioner{"transition fails": failingTransitioner{errors.New("boom")}} {
		t.Run(name, func(t *testing.T) {
			tenantID := newTenant()
			ctx := userCtx(tenantID)
			uc := newCreate(env, &scriptedIssues{}, tr)
			if _, err := uc.Execute(ctx, jiraIn(uuid.NewString(), "ENG-9", "", "t")); err == nil {
				t.Fatal("want error")
			}
			for _, table := range []string{"requests", "request_idempotency"} {
				if n := env.CountRows(t, table, tenantID); n != 0 {
					t.Fatalf("%s kept %d rows", table, n)
				}
			}
			if len(env.OutboxSubjects(t, tenantID)) != 0 {
				t.Fatal("outbox kept events of a rolled-back creation")
			}
			if peek := env.NextNumberPeek(t, tenantID); peek != 0 {
				t.Fatalf("number counter advanced to %d", peek)
			}
			// The same key works afterwards: no orphaned claim blocks it.
			ok, err := newCreate(env, &scriptedIssues{}, nil).Execute(ctx, jiraIn(uuid.NewString(), "ENG-9", "", "t"))
			if err != nil || !ok.Created || ok.Request.Number != 1 {
				t.Fatalf("retry after failure: %+v %v", ok, err)
			}
		})
	}
}

func intakeCancelledReturnedUnchanged(t *testing.T, env IntakeEnv) {
	tenantID := newTenant()
	ctx := userCtx(tenantID)
	uc := newCreate(env, &scriptedIssues{}, nil)
	in := jiraIn(uuid.NewString(), "ENG-5", "", "t")
	first, _ := uc.Execute(ctx, in)
	r, _ := env.Requests.Get(ctx, first.Request.ID)
	r.Status = domain.RequestStatusCancelled
	if _, err := env.Requests.Update(ctx, r, r.Version); err != nil {
		t.Fatal(err)
	}
	again, err := uc.Execute(ctx, in)
	if err != nil || again.Created || again.Request.Status != domain.RequestStatusCancelled {
		t.Fatalf("%+v %v", again, err)
	}
}

func intakeTenantIsolationOfKeys(t *testing.T, env IntakeEnv) {
	uc := newCreate(env, &scriptedIssues{}, nil)
	project := uuid.NewString()
	a, _ := uc.Execute(userCtx(newTenant()), jiraIn(project, "ENG-1", "", "t"))
	b, err := uc.Execute(userCtx(newTenant()), jiraIn(project, "ENG-1", "", "t"))
	if err != nil || !b.Created || a.Request.ID == b.Request.ID {
		t.Fatalf("tenants must not share keys: %+v %v", b, err)
	}
}

func intakeSourceHintsAndFilter(t *testing.T, env IntakeEnv) {
	ctx := userCtx(newTenant())
	uc := newCreate(env, &scriptedIssues{}, nil)
	in := jiraIn(uuid.NewString(), "ENG-1", "https://acme.atlassian.net", "t")
	in.Hints = domain.SourceHints{IssueType: "Story", Labels: []string{"ệ", "api"}, Priority: "High", TypeHint: "hotfix"}
	res, err := uc.Execute(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := env.Requests.Get(ctx, res.Request.ID)
	if got.SourceHints.IssueType != "Story" || len(got.SourceHints.Labels) != 2 || got.SourceHints.Labels[0] != "ệ" || got.SourceHints.TypeHint != "" {
		t.Fatalf("hints = %+v", got.SourceHints)
	}
	// "Does this issue already have a request?": filter values are the normalised ones.
	ref, _ := domain.NormalizeSourceRef(domain.SourceRef{Provider: domain.SourceProviderJira, Ref: "eng-1", Site: "HTTPS://acme.atlassian.net/"})
	list, err := env.Requests.List(ctx, usecase.ListFilter{SourceProvider: "jira", SourceSite: ref.Site, SourceRef: ref.Ref})
	if err != nil || len(list.Requests) != 1 || list.Requests[0].ID != res.Request.ID {
		t.Fatalf("%+v %v", list, err)
	}
	none, _ := env.Requests.List(ctx, usecase.ListFilter{SourceProvider: "jira", SourceRef: "ENG-404"})
	if len(none.Requests) != 0 {
		t.Fatal("filter matched the wrong request")
	}
}

func intakeLookupBySource(t *testing.T, env IntakeEnv) {
	ctx := userCtx(newTenant())
	uc := newCreate(env, &scriptedIssues{}, nil)
	res, _ := uc.Execute(ctx, jiraIn(uuid.NewString(), "ENG-1", "", "t"))
	lookup := usecase.NewLookupRequestBySource(env.Requests, env.Idempotency)
	r, found, err := lookup.Execute(ctx, domain.SourceRef{Provider: domain.SourceProviderJira, Ref: "eng-1"})
	if err != nil || !found || r.ID != res.Request.ID {
		t.Fatalf("%+v %v %v", r, found, err)
	}
	if _, found, _ := lookup.Execute(userCtx(newTenant()), domain.SourceRef{Provider: domain.SourceProviderJira, Ref: "eng-1"}); found {
		t.Fatal("lookup crossed tenants")
	}
}
