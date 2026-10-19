package grpc

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const baseRefReq = "6f1d6a52-1c2e-4f0a-9a57-0d3c1f8a7b10"

// baseRefPorts is one in-memory Request behind every port SetRequestBaseRef needs.
type baseRefPorts struct {
	usecase.RequestRepository // only Get is used
	mu                        sync.Mutex
	req                       domain.Request
	lock                      domain.BaseRefLockReason
	check                     usecase.BaseRefCheck
}

func (p *baseRefPorts) InTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}
func (p *baseRefPorts) Get(context.Context, string) (domain.Request, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.req, nil
}
func (p *baseRefPorts) LockRequest(ctx context.Context, id string) (domain.Request, error) {
	return p.Get(ctx, id)
}
func (p *baseRefPorts) SetBaseRef(_ context.Context, _ string, ref string, _ int64) (domain.Request, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.req.BaseRef, p.req.Version = ref, p.req.Version+1
	return p.req, nil
}
func (p *baseRefPorts) Has(context.Context, string) (bool, domain.BaseRefLockReason, error) {
	return p.lock != domain.BaseRefUnlocked, p.lock, nil
}
func (p *baseRefPorts) HasMany(context.Context, []string) (map[string]domain.BaseRefLockReason, error) {
	return nil, errors.New("unused")
}
func (p *baseRefPorts) Check(context.Context, string, string) (usecase.BaseRefCheck, error) {
	if p.check.Status == "" {
		return usecase.BaseRefCheck{Status: usecase.BaseRefFound}, nil
	}
	return p.check, nil
}
func (p *baseRefPorts) InsertOutboxEvent(context.Context, domain.OutboxEvent) error { return nil }

func newBaseRefServer(t *testing.T, p *baseRefPorts) *Server {
	t.Helper()
	uc, err := usecase.NewSetRequestBaseRef(usecase.SetRequestBaseRefDeps{
		Requests: p, Locker: p, Writer: p, Presence: p, Checker: p, Tx: p, Outbox: p,
	})
	if err != nil {
		t.Fatal(err)
	}
	return NewServer(nil, nil).WithBaseRef(BaseRefUseCases{Set: uc})
}

func baseRefCtx() context.Context {
	return tenant.WithUserID(tenant.WithTenantID(context.Background(), "t1"), "user-1")
}

func newBaseRefPorts() *baseRefPorts {
	return &baseRefPorts{req: domain.Request{ID: baseRefReq, TenantID: "t1", ProjectID: "p1", Status: domain.RequestStatusNew, Version: 4}}
}

func errorInfo(t *testing.T, err error) *errdetails.ErrorInfo {
	t.Helper()
	for _, d := range status.Convert(err).Details() {
		if info, ok := d.(*errdetails.ErrorInfo); ok {
			return info
		}
	}
	t.Fatalf("no ErrorInfo detail on %v", err)
	return nil
}

func TestMapper_BaseRefFields(t *testing.T) {
	p := toProtoRequest(domain.Request{BaseRef: "release/1.4", BaseRefLocked: true, BaseRefLockReason: domain.BaseRefLockedTask})
	if p.GetBaseRef() != "release/1.4" || !p.GetBaseRefLocked() || p.GetBaseRefLockReason() != "task" {
		t.Fatalf("%+v", p)
	}
	if q := toProtoRequest(domain.Request{}); q.GetBaseRef() != "" || q.GetBaseRefLocked() || q.GetBaseRefLockReason() != "" {
		t.Fatalf("defaults: %+v", q)
	}
}

func TestSetRequestBaseRef_Unwired(t *testing.T) {
	_, err := NewServer(nil, nil).SetRequestBaseRef(baseRefCtx(), &requestv1.SetRequestBaseRefRequest{RequestId: baseRefReq})
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("%v", err)
	}
}

func TestSetRequestBaseRef_Success(t *testing.T) {
	p := newBaseRefPorts()
	resp, err := newBaseRefServer(t, p).SetRequestBaseRef(baseRefCtx(), &requestv1.SetRequestBaseRefRequest{RequestId: baseRefReq, BaseRef: "release/1.4", ExpectedVersion: 4})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetRequest().GetBaseRef() != "release/1.4" || resp.GetRequest().GetVersion() != 5 || resp.GetBaseRefUnverified() || resp.GetRequest().GetBaseRefLocked() {
		t.Fatalf("%+v", resp)
	}
}

func TestSetRequestBaseRef_UnreachableIsFlagged(t *testing.T) {
	p := newBaseRefPorts()
	p.check = usecase.BaseRefCheck{Status: usecase.BaseRefUnreachable}
	resp, err := newBaseRefServer(t, p).SetRequestBaseRef(baseRefCtx(), &requestv1.SetRequestBaseRefRequest{RequestId: baseRefReq, BaseRef: "release/1.4"})
	if err != nil || !resp.GetBaseRefUnverified() {
		t.Fatalf("%+v %v", resp, err)
	}
}

func TestSetRequestBaseRef_ErrorsKeepTheirGRPCKindAndDetails(t *testing.T) {
	t.Run("invalid is InvalidArgument", func(t *testing.T) {
		_, err := newBaseRefServer(t, newBaseRefPorts()).SetRequestBaseRef(baseRefCtx(), &requestv1.SetRequestBaseRefRequest{RequestId: baseRefReq, BaseRef: "-x"})
		if status.Code(err) != codes.InvalidArgument || !strings.HasPrefix(status.Convert(err).Message(), "REQUEST_BASE_REF_INVALID") {
			t.Fatalf("%v", err)
		}
	})
	t.Run("not found is NotFound with the suggestions", func(t *testing.T) {
		p := newBaseRefPorts()
		p.check = usecase.BaseRefCheck{Status: usecase.BaseRefNotFound, Suggestions: []string{"main", "release/1.3"}}
		_, err := newBaseRefServer(t, p).SetRequestBaseRef(baseRefCtx(), &requestv1.SetRequestBaseRefRequest{RequestId: baseRefReq, BaseRef: "release/9"})
		if status.Code(err) != codes.NotFound {
			t.Fatalf("%v", err)
		}
		info := errorInfo(t, err)
		if info.GetReason() != "REQUEST_BASE_REF_NOT_FOUND" || info.GetMetadata()["suggestions"] != "main,release/1.3" || info.GetMetadata()["ref"] != "release/9" {
			t.Fatalf("%+v", info)
		}
	})
	t.Run("locked is FailedPrecondition with the reason", func(t *testing.T) {
		p := newBaseRefPorts()
		p.lock = domain.BaseRefLockedProposal
		_, err := newBaseRefServer(t, p).SetRequestBaseRef(baseRefCtx(), &requestv1.SetRequestBaseRefRequest{RequestId: baseRefReq, BaseRef: "main"})
		if status.Code(err) != codes.FailedPrecondition {
			t.Fatalf("%v", err)
		}
		if info := errorInfo(t, err); info.GetReason() != "REQUEST_BASE_REF_LOCKED" || info.GetMetadata()["reason"] != "proposal" {
			t.Fatalf("%+v", info)
		}
	})
	t.Run("an agent session is refused", func(t *testing.T) {
		ctx := tenant.WithActorType(baseRefCtx(), tenant.ActorAgent)
		_, err := newBaseRefServer(t, newBaseRefPorts()).SetRequestBaseRef(ctx, &requestv1.SetRequestBaseRefRequest{RequestId: baseRefReq, BaseRef: "main"})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("%v", err)
		}
	})
	t.Run("no user is refused", func(t *testing.T) {
		ctx := tenant.WithTenantID(context.Background(), "t1")
		_, err := newBaseRefServer(t, newBaseRefPorts()).SetRequestBaseRef(ctx, &requestv1.SetRequestBaseRefRequest{RequestId: baseRefReq, BaseRef: "main"})
		if err == nil || status.Code(err) == codes.OK {
			t.Fatalf("%v", err)
		}
	})
}

func TestBaseRefStatus_PlainErrorsGetNoDetail(t *testing.T) {
	err := baseRefStatus(domain.ErrRequestBaseRefInvalid("must not be blank"))
	if status.Code(err) != codes.InvalidArgument || len(status.Convert(err).Details()) != 0 {
		t.Fatalf("%v", err)
	}
	if status.Code(baseRefStatus(errors.New("boom"))) != codes.Internal {
		t.Fatal("an unknown error stays Internal")
	}
}

// createPorts backs a real CreateRequest with one in-memory Request so the handler's wire mapping can be checked end to end.
type createPorts struct {
	usecase.RequestRepository
	usecase.RequestIdempotencyRepository
	stored  domain.Request
	checked string
	status  usecase.BaseRefStatus
}

func (c *createPorts) InTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }
func (c *createPorts) NextNumber(context.Context) (int64, error)                      { return 1, nil }
func (c *createPorts) Create(_ context.Context, r domain.Request) error               { c.stored = r; return nil }
func (c *createPorts) InsertOutboxEvent(context.Context, domain.OutboxEvent) error    { return nil }
func (c *createPorts) Execute(_ context.Context, in usecase.TransitionInput) (usecase.TransitionResult, error) {
	return usecase.TransitionResult{Request: c.stored}, nil
}
func (c *createPorts) Check(_ context.Context, _, ref string) (usecase.BaseRefCheck, error) {
	c.checked = ref
	return usecase.BaseRefCheck{Status: c.status}, nil
}

func TestCreateRequest_BaseRefTravelsThroughTheHandler(t *testing.T) {
	ports := &createPorts{status: usecase.BaseRefUnreachable}
	uc := usecase.NewCreateRequest(ports, ports, ports, ports, ports, nil).WithBaseRefChecker(ports)
	s := NewServer(nil, nil).WithIntake(IntakeUseCases{Create: uc})
	ctx := tenant.WithUserID(tenant.WithTenantID(context.Background(), "t1"), "user-1")

	resp, err := s.CreateRequest(ctx, &requestv1.CreateRequestRequest{ProjectId: baseRefReq, Title: "needs a branch", Source: &requestv1.RequestSource{Provider: "manual"}, BaseRef: " release/1.4 "})
	if err != nil {
		t.Fatal(err)
	}
	if ports.checked != "release/1.4" || ports.stored.BaseRef != "release/1.4" {
		t.Fatalf("checked %q, stored %q", ports.checked, ports.stored.BaseRef)
	}
	if resp.GetRequest().GetBaseRef() != "release/1.4" || !resp.GetBaseRefUnverified() || resp.GetRequest().GetBaseRefLocked() {
		t.Fatalf("%+v", resp)
	}

	_, err = s.CreateRequest(ctx, &requestv1.CreateRequestRequest{ProjectId: baseRefReq, Title: "bad", Source: &requestv1.RequestSource{Provider: "manual"}, BaseRef: "a..b"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("an invalid ref is InvalidArgument: %v", err)
	}
	ports.status = usecase.BaseRefNotFound
	_, err = s.CreateRequest(ctx, &requestv1.CreateRequestRequest{ProjectId: baseRefReq, Title: "gone", Source: &requestv1.RequestSource{Provider: "manual"}, BaseRef: "release/9"})
	if status.Code(err) != codes.NotFound || errorInfo(t, err).GetReason() != "REQUEST_BASE_REF_NOT_FOUND" {
		t.Fatalf("a missing ref is NotFound with its ErrorInfo: %v", err)
	}
}
