package grpc

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeProjectExistence struct {
	mu      sync.Mutex
	exists  bool
	err     error
	asked   []string
	askedBy []string
}

func (f *fakeProjectExistence) Exists(_ context.Context, tenantID, projectID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = append(f.asked, projectID)
	f.askedBy = append(f.askedBy, tenantID)
	return f.exists, f.err
}

type orphanCounter struct {
	mu      sync.Mutex
	actions []string
}

func (o *orphanCounter) ObserveOrphanRequest(action string) {
	o.mu.Lock()
	o.actions = append(o.actions, action)
	o.mu.Unlock()
}

const (
	getRequestRPC    = "/orca.request.v1.RequestService/GetRequest"
	createRequestRPC = "/orca.request.v1.RequestService/CreateRequest"
	cancelRequestRPC = "/orca.request.v1.RequestService/CancelRequest"
)

func orphanInterceptor(t *testing.T, projects *fakeProjectExistence, obs *orphanCounter) func(role string, method string) (bool, error) {
	t.Helper()
	a := newAuthorizer(realPolicy(t), newAccessRoles(), &recordingAuditor{}).WithOrphanObserver(projects, obs)
	ic := RequestAccessInterceptor(a)
	return func(role, method string) (bool, error) {
		return run(ic, callerCtx(role, "user"), method, requestFor(t, method))
	}
}

func TestRequestAccess_CountsAnExistingRequestWhoseProjectIsGone(t *testing.T) {
	projects, obs := &fakeProjectExistence{exists: false}, &orphanCounter{}
	call := orphanInterceptor(t, projects, obs)

	// An admin skips the membership check, so only the existence probe can see the orphan.
	if called, err := call("admin", getRequestRPC); !called || err != nil {
		t.Fatalf("the metric must not change the outcome: called=%v err=%v", called, err)
	}
	if len(obs.actions) != 1 || obs.actions[0] != string(domain.GroupRead) {
		t.Fatalf("observed %v, want one read", obs.actions)
	}
	if projects.asked[0] != testProject || projects.askedBy[0] != testTenant {
		t.Fatalf("asked %v for tenant %v", projects.asked, projects.askedBy)
	}

	if _, err := call("admin", cancelRequestRPC); err != nil {
		t.Fatal(err)
	}
	if len(obs.actions) != 2 || obs.actions[1] == obs.actions[0] {
		t.Fatalf("each RPC group is its own series, got %v", obs.actions)
	}
}

func TestRequestAccess_OrphanMetricDoesNotChangeRefusals(t *testing.T) {
	projects, obs := &fakeProjectExistence{exists: false}, &orphanCounter{}
	call := orphanInterceptor(t, projects, obs)
	// A stranger is refused as before; the orphan is still counted because the Request was found.
	if called, err := call("stranger", getRequestRPC); called || status.Code(err) != codes.PermissionDenied {
		t.Fatalf("called=%v err=%v", called, err)
	}
	if len(obs.actions) != 1 {
		t.Fatalf("observed %v", obs.actions)
	}
}

func TestRequestAccess_NoOrphanWhenTheProjectExistsOrCannotBeChecked(t *testing.T) {
	cases := map[string]*fakeProjectExistence{
		"project exists":          {exists: true},
		"project-service failing": {exists: false, err: errors.New("project-service down")},
	}
	for name, projects := range cases {
		t.Run(name, func(t *testing.T) {
			obs := &orphanCounter{}
			call := orphanInterceptor(t, projects, obs)
			if called, err := call("admin", getRequestRPC); !called || err != nil {
				t.Fatalf("called=%v err=%v", called, err)
			}
			if len(obs.actions) != 0 {
				t.Fatalf("counted %v without having seen a missing project", obs.actions)
			}
		})
	}
}

func TestRequestAccess_OrphanProbeSkipsRPCsWithoutAnExistingRequest(t *testing.T) {
	projects, obs := &fakeProjectExistence{exists: false}, &orphanCounter{}
	call := orphanInterceptor(t, projects, obs)
	// CreateRequest names a project, not a Request: a project that does not exist yet is refused elsewhere, not an orphan.
	_, _ = call("owner", createRequestRPC)
	// A Request that does not exist (or belongs to another tenant) is not found before any probe.
	ic := RequestAccessInterceptor(newAuthorizer(realPolicy(t), newAccessRoles(), &recordingAuditor{}).WithOrphanObserver(projects, obs))
	_, err := run(ic, callerCtx("admin", "user"), getRequestRPC, requestForID(t, getRequestRPC, "99999999-9999-4999-8999-999999999999"))
	if status.Code(err) != codes.NotFound {
		t.Fatalf("err = %v", err)
	}
	if len(projects.asked) != 0 || len(obs.actions) != 0 {
		t.Fatalf("probed %v, observed %v", projects.asked, obs.actions)
	}
}

func TestRequestAccess_WithoutAnObserverNothingIsProbed(t *testing.T) {
	projects := &fakeProjectExistence{exists: false}
	a := newAuthorizer(realPolicy(t), newAccessRoles(), &recordingAuditor{})
	ic := RequestAccessInterceptor(a)
	if called, err := run(ic, callerCtx("admin", "user"), getRequestRPC, requestFor(t, getRequestRPC)); !called || err != nil {
		t.Fatalf("called=%v err=%v", called, err)
	}
	if len(projects.asked) != 0 {
		t.Fatal("an unconfigured observer must not probe")
	}
}
