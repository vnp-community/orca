package usecase

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type createFixture struct {
	s      *memStore
	issues *fakeIssues
	uc     *CreateRequest
	tr     *memTransitioner
}

func newCreateFixture() *createFixture {
	s := newMemStore()
	f := &createFixture{s: s, issues: &fakeIssues{}, tr: &memTransitioner{s: s}}
	f.uc = NewCreateRequest(s, s, s, s, f.tr, f.issues)
	return f
}

func manualInput(title string) CreateRequestInput {
	return CreateRequestInput{ProjectID: testProject, Title: title, Source: domain.SourceRef{Provider: domain.SourceProviderManual}}
}

func TestCreate_Manual_NoClientID_AlwaysNew(t *testing.T) {
	f := newCreateFixture()
	a, err := f.uc.Execute(tctx(), manualInput("one"))
	if err != nil || !a.Created {
		t.Fatalf("first: %+v %v", a, err)
	}
	b, err := f.uc.Execute(tctx(), manualInput("one"))
	if err != nil || !b.Created || a.Request.ID == b.Request.ID || b.Request.Number != 2 {
		t.Fatalf("second must be a new request: %+v %v", b, err)
	}
}

func TestCreate_Manual_WithClientID_Idempotent(t *testing.T) {
	f := newCreateFixture()
	in := manualInput("one")
	in.ClientRequestID = "client-1"
	a, _ := f.uc.Execute(tctx(), in)
	b, err := f.uc.Execute(tctx(), in)
	if err != nil || b.Created || b.Request.ID != a.Request.ID {
		t.Fatalf("retry must return the first request: %+v %v", b, err)
	}
	if len(f.s.requests) != 1 {
		t.Fatalf("%d requests stored", len(f.s.requests))
	}
}

func jiraInput(ref string) CreateRequestInput {
	return CreateRequestInput{ProjectID: testProject, Source: domain.SourceRef{Provider: domain.SourceProviderJira, Ref: ref, Site: "https://acme.atlassian.net"}}
}

func TestCreate_Jira_EnrichesTitle(t *testing.T) {
	f := newCreateFixture()
	f.issues.snap = IssueSnapshot{Title: "Login broken", Body: "steps", URL: "https://acme/ENG-1", Hints: domain.SourceHints{IssueType: "Bug", Labels: []string{"auth"}, TypeHint: "hotfix"}}
	res, err := f.uc.Execute(tctx(), jiraInput("eng-1"))
	if err != nil {
		t.Fatal(err)
	}
	r := res.Request
	if r.Title != "Login broken" || r.Body != "steps" || r.SourceURL != "https://acme/ENG-1" || r.SourceRef != "ENG-1" || r.SourceHints.IssueType != "Bug" {
		t.Fatalf("not enriched: %+v", r)
	}
	if r.SourceHints.TypeHint != "" {
		t.Fatal("type_hint must be dropped for public calls")
	}
}

func TestCreate_Jira_EnrichFailsButTitlePresent_StillCreates(t *testing.T) {
	f := newCreateFixture()
	f.issues.err = errors.New("jira down")
	in := jiraInput("ENG-2")
	in.Title = "typed by caller"
	res, err := f.uc.Execute(tctx(), in)
	if err != nil || !res.Created || res.Request.Title != "typed by caller" {
		t.Fatalf("%+v %v", res, err)
	}
	if f.issues.calls != 1 {
		t.Fatalf("enrichment should have been attempted once, got %d", f.issues.calls)
	}
}

func TestCreate_Jira_NotFound(t *testing.T) {
	f := newCreateFixture()
	f.issues.err = ErrIssueNotFound
	_, err := f.uc.Execute(tctx(), jiraInput("ENG-404"))
	mustCode(t, err, "REQUEST_SOURCE_NOT_FOUND")
	f.issues.err = errors.New("boom")
	_, err = f.uc.Execute(tctx(), jiraInput("ENG-500"))
	mustCode(t, err, "REQUEST_SOURCE_FETCH_FAILED")
	if len(f.s.requests) != 0 || f.s.counter != 0 {
		t.Fatal("failed intake must leave nothing behind")
	}
}

func TestCreate_GitHub_MissingTitle_NoExternalCall(t *testing.T) {
	f := newCreateFixture()
	_, err := f.uc.Execute(tctx(), CreateRequestInput{ProjectID: testProject, Source: domain.SourceRef{Provider: domain.SourceProviderGithub, Ref: "o/r#1"}})
	mustCode(t, err, "REQUEST_TITLE_REQUIRED")
	if f.issues.calls != 0 {
		t.Fatal("github intake must not call the issue tracker")
	}
}

func TestCreate_ClaimLoser_ReturnsWinner(t *testing.T) {
	f := newCreateFixture()
	in := jiraInput("ENG-7")
	in.Title = "t"
	var wg sync.WaitGroup
	results := make([]CreateRequestResult, 12)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			res, err := f.uc.Execute(tctx(), in)
			if err != nil {
				t.Error(err)
			}
			results[i] = res
		}(i)
	}
	wg.Wait()
	created := 0
	for _, r := range results {
		if r.Created {
			created++
		}
		if r.Request.ID != results[0].Request.ID {
			t.Fatal("all callers must see the same request")
		}
	}
	if created != 1 || len(f.s.requests) != 1 || f.s.counter != 1 {
		t.Fatalf("created=%d requests=%d counter=%d", created, len(f.s.requests), f.s.counter)
	}
	n := 0
	for _, e := range f.s.events {
		if e.Subject == domain.SubjectRequestCreated {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("%d created events", n)
	}
}

func TestCreate_ClosedRequestReturnedAsIs(t *testing.T) {
	f := newCreateFixture()
	in := jiraInput("ENG-8")
	in.Title = "t"
	first, _ := f.uc.Execute(tctx(), in)
	r := f.s.requests[first.Request.ID]
	r.Status = domain.RequestStatusCancelled
	f.s.requests[r.ID] = r
	again, err := f.uc.Execute(tctx(), in)
	if err != nil || again.Created || again.Request.Status != domain.RequestStatusCancelled {
		t.Fatalf("%+v %v", again, err)
	}
}

func TestCreate_TypeHintIgnoredForPublicCall(t *testing.T) {
	f := newCreateFixture()
	in := manualInput("t")
	in.Hints = domain.SourceHints{TypeHint: "hotfix", IssueType: "Bug"}
	res, _ := f.uc.Execute(tctx(), in)
	if res.Request.SourceHints.TypeHint != "" || res.Request.SourceHints.IssueType != "Bug" {
		t.Fatalf("%+v", res.Request.SourceHints)
	}
	in.AllowTypeHint = true
	res, _ = f.uc.Execute(tctx(), in)
	if res.Request.SourceHints.TypeHint != "hotfix" {
		t.Fatal("trusted callers keep type_hint")
	}
}

func TestCreate_EmitsCreatedThenStatusChanged(t *testing.T) {
	f := newCreateFixture()
	res, err := f.uc.Execute(tctx(), manualInput("t"))
	if err != nil {
		t.Fatal(err)
	}
	got := f.s.subjects()
	if len(got) != 2 || got[0] != domain.SubjectRequestCreated || got[1] != domain.SubjectRequestStatusChanged {
		t.Fatalf("events = %v", got)
	}
	if res.Request.Status != domain.RequestStatusClassifying || f.s.requests[res.Request.ID].Status != domain.RequestStatusClassifying {
		t.Fatalf("status = %s", res.Request.Status)
	}
}

func TestCreate_FailureMidwayLeavesNothing(t *testing.T) {
	for name, arrange := range map[string]func(f *createFixture){
		"transition fails": func(f *createFixture) { f.tr.fail = errors.New("boom") },
		"create fails":     func(f *createFixture) { f.s.failCreate = errors.New("boom") },
		"outbox fails":     func(f *createFixture) { f.s.failOutbox = func(string) error { return errors.New("boom") } },
	} {
		t.Run(name, func(t *testing.T) {
			f := newCreateFixture()
			arrange(f)
			in := jiraInput("ENG-9")
			in.Title = "t"
			if _, err := f.uc.Execute(tctx(), in); err == nil {
				t.Fatal("want error")
			}
			if len(f.s.idem) != 0 || f.s.counter != 0 || len(f.s.requests) != 0 || len(f.s.events) != 0 {
				t.Fatalf("leftovers: idem=%d counter=%d requests=%d events=%d", len(f.s.idem), f.s.counter, len(f.s.requests), len(f.s.events))
			}
		})
	}
}

func TestCreate_RejectsMissingIdentity(t *testing.T) {
	f := newCreateFixture()
	_, err := f.uc.Execute(tenant.WithTenantID(context.Background(), testTenant), manualInput("t"))
	mustCode(t, err, "REQUEST_REPORTER_REQUIRED")
	in := manualInput("t")
	in.ProjectID = ""
	_, err = f.uc.Execute(tctx(), in)
	mustCode(t, err, "REQUEST_PROJECT_REQUIRED")
	in.ProjectID = "nope"
	_, err = f.uc.Execute(tctx(), in)
	mustCode(t, err, "REQUEST_PROJECT_INVALID")
	_, err = f.uc.Execute(context.Background(), manualInput("t"))
	mustCode(t, err, "REQUEST_TENANT_REQUIRED")
}

type countingRecorder struct{ n int }

func (c *countingRecorder) RecordCreated(context.Context, domain.Request) error { c.n++; return nil }

func TestCreate_CreationRecorderRunsInTx(t *testing.T) {
	f := newCreateFixture()
	rec := &countingRecorder{}
	f.uc.WithCreationRecorder(rec)
	if _, err := f.uc.Execute(tctx(), manualInput("t")); err != nil || rec.n != 1 {
		t.Fatalf("recorder calls = %d err=%v", rec.n, err)
	}
}

func TestLookupRequestBySource(t *testing.T) {
	f := newCreateFixture()
	in := jiraInput("eng-5")
	in.Title = "t"
	created, _ := f.uc.Execute(tctx(), in)
	lookup := NewLookupRequestBySource(f.s, f.s)
	r, found, err := lookup.Execute(tctx(), domain.SourceRef{Provider: domain.SourceProviderJira, Ref: "ENG-5", Site: "HTTPS://acme.atlassian.net/"})
	if err != nil || !found || r.ID != created.Request.ID {
		t.Fatalf("%+v %v %v", r, found, err)
	}
	_, found, err = lookup.Execute(tctx(), domain.SourceRef{Provider: domain.SourceProviderJira, Ref: "ENG-6", Site: "https://acme.atlassian.net"})
	if err != nil || found {
		t.Fatalf("unknown ref: found=%v err=%v", found, err)
	}
	_, _, err = lookup.Execute(tctx(), domain.SourceRef{Provider: domain.SourceProviderManual})
	mustCode(t, err, "REQUEST_SOURCE_REF_REQUIRED")
}

func TestListRequests_NormalisesSourceFilter(t *testing.T) {
	f := newCreateFixture()
	var seen ListFilter
	uc := NewListRequests(filterSpy{RequestRepository: f.s, seen: &seen})
	if _, err := uc.Execute(tctx(), ListFilter{SourceProvider: "jira", SourceSite: "HTTPS://Acme.atlassian.net/", SourceRef: "eng-1"}); err != nil {
		t.Fatal(err)
	}
	if seen.SourceRef != "ENG-1" || seen.SourceSite != "https://acme.atlassian.net" {
		t.Fatalf("filter not normalised: %+v", seen)
	}
	_, err := uc.Execute(tctx(), ListFilter{SourceProvider: "github", SourceRef: "no-number"})
	mustCode(t, err, "REQUEST_SOURCE_REF_INVALID")
}

type filterSpy struct {
	RequestRepository
	seen *ListFilter
}

func (f filterSpy) List(_ context.Context, lf ListFilter) (ListResult, error) {
	*f.seen = lf
	return ListResult{}, nil
}

type recordingFlags struct{ marked []string }

func (r *recordingFlags) MarkSecretSuspected(_ context.Context, id string) error {
	r.marked = append(r.marked, id)
	return nil
}
func (r *recordingFlags) Get(context.Context, string) (SecurityFlags, error) {
	return SecurityFlags{}, nil
}

// Manual, webhook and MCP all enter through Execute, so one guard covers them; each source is exercised.
func TestCreate_MasksHighConfidenceSecretsAndFlagsTheRequest(t *testing.T) {
	pem := "-----BEGIN RSA PRIVATE KEY-----\nMIIBOgIBAAJBAKj34GkxFhD90vcNLYLInFEX6Ppy1tPf9Cnzj4p4WGeKLs1Pt8Qu\n-----END RSA PRIVATE KEY-----"
	for _, provider := range []domain.SourceProvider{domain.SourceProviderManual, domain.SourceProviderWebhook, domain.SourceProviderMCP} {
		f := newCreateFixture()
		flags := &recordingFlags{}
		f.uc.WithSecurityFlags(flags)
		in := manualInput("login broken")
		in.Source = domain.SourceRef{Provider: provider}
		if provider == domain.SourceProviderWebhook {
			in.Source = domain.SourceRef{Provider: provider, Site: "jira-hook", Ref: "ref-1"}
		}
		in.ClientRequestID = "client-1"
		in.Body = "my key is\n" + pem
		res, err := f.uc.Execute(tctx(), in)
		if err != nil {
			t.Fatalf("%s: %v", provider, err)
		}
		if strings.Contains(res.Request.Body, "BEGIN RSA PRIVATE KEY") || !strings.Contains(res.Request.Body, "[REDACTED:private_key]") {
			t.Errorf("%s: stored body not masked: %q", provider, res.Request.Body)
		}
		if len(flags.marked) != 1 || flags.marked[0] != res.Request.ID {
			t.Errorf("%s: contains_secret_suspected not recorded: %v", provider, flags.marked)
		}
		// A retry returns the same Request: the idempotency key does not depend on the masked body.
		again, err := f.uc.Execute(tctx(), in)
		if err != nil || again.Request.ID != res.Request.ID || again.Created {
			t.Errorf("%s: retry %v %v", provider, again.Request.ID, err)
		}
	}
}

func TestCreate_CleanTextIsNotFlagged(t *testing.T) {
	f := newCreateFixture()
	flags := &recordingFlags{}
	f.uc.WithSecurityFlags(flags)
	in := manualInput("Lỗi đăng nhập")
	in.Body = "người dùng không đặt lại được mật khẩu"
	res, err := f.uc.Execute(tctx(), in)
	if err != nil || res.Request.Body != in.Body || len(flags.marked) != 0 {
		t.Fatalf("%v %q %v", err, res.Request.Body, flags.marked)
	}
}
