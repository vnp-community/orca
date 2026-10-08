package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type fakeFinder struct {
	gotProvider, gotSite, gotRef string
	id                           string
	ok                           bool
	err                          error
	calls                        int
}

func (f *fakeFinder) FindActiveBySource(_ context.Context, provider, site, ref string) (string, bool, error) {
	f.calls++
	f.gotProvider, f.gotSite, f.gotRef = provider, site, ref
	return f.id, f.ok, f.err
}

func lookupWith(f *fakeFinder, flag func(context.Context) (bool, error)) *LookupRequestBySource {
	opts := []LookupOption{WithActiveSourceFinder(f)}
	if flag != nil {
		opts = append(opts, WithLookupFlagGate(flag))
	}
	return NewLookupRequestBySource(nil, nil, opts...)
}

func TestLookupActive_FoundReturnsOnlyTheID(t *testing.T) {
	f := &fakeFinder{id: "req-9", ok: true}
	r, found, err := lookupWith(f, func(context.Context) (bool, error) { return true, nil }).
		Execute(flowCtx("t1", ""), domain.SourceRef{Provider: domain.SourceProviderJira, Ref: "eng-4", Site: "https://A.atlassian.net/"})
	if err != nil || !found || r.ID != "req-9" {
		t.Fatalf("got (%+v, %v, %v)", r, found, err)
	}
	if r.Title != "" || r.Body != "" {
		t.Fatalf("lookup leaked content: %+v", r)
	}
	if f.gotProvider != "jira" || f.gotRef != "ENG-4" || f.gotSite != "https://a.atlassian.net" {
		t.Fatalf("the lookup key must be normalised like CreateRequest does: %q %q %q", f.gotProvider, f.gotSite, f.gotRef)
	}
}

func TestLookupActive_EmptySiteIsPassedThroughToMatchAnySite(t *testing.T) {
	f := &fakeFinder{ok: false}
	if _, found, err := lookupWith(f, nil).Execute(flowCtx("t1", ""), domain.SourceRef{Provider: domain.SourceProviderGithub, Ref: "Acme/Repo#7"}); err != nil || found {
		t.Fatalf("got (%v, %v)", found, err)
	}
	if f.gotSite != "" || f.gotRef != "acme/repo#7" {
		t.Fatalf("site %q ref %q", f.gotSite, f.gotRef)
	}
}

func TestLookupActive_FlagOffAnswersNotFoundWithoutQuerying(t *testing.T) {
	f := &fakeFinder{id: "req-9", ok: true}
	for name, flag := range map[string]func(context.Context) (bool, error){
		"off":        func(context.Context) (bool, error) { return false, nil },
		"read error": func(context.Context) (bool, error) { return false, errors.New("db down") },
	} {
		_, found, err := lookupWith(f, flag).Execute(flowCtx("t1", ""), domain.SourceRef{Provider: domain.SourceProviderJira, Ref: "ENG-4"})
		if err != nil || found {
			t.Fatalf("%s: got (%v, %v), want found=false and no error", name, found, err)
		}
	}
	if f.calls != 0 {
		t.Fatalf("finder queried %d times while the flow is off", f.calls)
	}
}

func TestLookupActive_Rejections(t *testing.T) {
	f := &fakeFinder{id: "x", ok: true}
	u := lookupWith(f, nil)
	if _, _, err := u.Execute(context.Background(), domain.SourceRef{Provider: domain.SourceProviderJira, Ref: "ENG-4"}); err == nil {
		t.Fatal("a call without a tenant must fail")
	}
	if _, _, err := u.Execute(tenant.WithTenantID(context.Background(), "t1"), domain.SourceRef{Provider: domain.SourceProviderManual}); err == nil {
		t.Fatal("manual has no external issue to look up")
	}
	if _, _, err := u.Execute(flowCtx("t1", ""), domain.SourceRef{Provider: "nope", Ref: "x"}); err == nil {
		t.Fatal("unknown provider must fail")
	}
	if f.calls != 0 {
		t.Fatal("invalid input reached the finder")
	}
}

func TestLookupActive_FinderErrorIsReturned(t *testing.T) {
	f := &fakeFinder{err: errors.New("db down")}
	if _, _, err := lookupWith(f, nil).Execute(flowCtx("t1", ""), domain.SourceRef{Provider: domain.SourceProviderJira, Ref: "ENG-4"}); err == nil {
		t.Fatal("a failing lookup must surface so issue-status-sync can retry")
	}
}
