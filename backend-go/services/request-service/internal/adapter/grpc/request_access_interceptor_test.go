package grpc

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/adapter/opaclient"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
)

type matrixFile struct {
	Roles         []string            `json:"roles"`
	PersonAllowed map[string][]string `json:"person_allowed"`
}

func loadMatrix(t *testing.T) matrixFile {
	t.Helper()
	b, err := os.ReadFile("../opaclient/testdata/access_matrix.json")
	if err != nil {
		t.Fatal(err)
	}
	var m matrixFile
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func realPolicy(t *testing.T) usecase.AccessPolicy {
	t.Helper()
	p := opaclient.NewRequestPolicy("../../../../../policy/orca-authz")
	if err := p.Warm(context.Background()); err != nil {
		t.Fatal(err)
	}
	return p
}

// callerCtx builds the context the chain hands to the access interceptor for a role.
func callerCtx(role, actor string) context.Context {
	ctx := tenant.WithTenantID(context.Background(), testTenant)
	user := map[string]string{"admin": userAdmin, "owner": userOwner, "member": userMember, "reporter": userReporter, "stranger": userStranger}[role]
	ctx = tenant.WithUserID(ctx, user)
	if role == "admin" {
		ctx = tenant.WithRole(ctx, "admin")
	}
	return tenant.WithActorType(ctx, actor)
}

// requestFor builds an empty request message of the RPC with the catalog field set to the right id.
func requestFor(t *testing.T, method string) proto.Message {
	t.Helper()
	parts := strings.Split(strings.TrimPrefix(method, "/"), "/")
	d, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(parts[0]))
	if err != nil {
		t.Fatal(err)
	}
	md := d.(protoreflect.ServiceDescriptor).Methods().ByName(protoreflect.Name(parts[1]))
	msg := dynamicpb.NewMessage(md.Input())
	e := domain.Catalog[method]
	if e.Field != "" {
		id := map[domain.Locator]string{domain.LocRequestID: testRequest, domain.LocApprovalID: testApproval, domain.LocProjectID: testProject, domain.LocEntityID: testEntity}[e.Locator]
		msg.Set(md.Input().Fields().ByName(protoreflect.Name(e.Field)), protoreflect.ValueOfString(id))
	}
	return msg
}

func run(ic grpc.UnaryServerInterceptor, ctx context.Context, method string, req any) (called bool, err error) {
	_, err = ic(ctx, req, &grpc.UnaryServerInfo{FullMethod: method}, func(context.Context, any) (any, error) {
		called = true
		return nil, nil
	})
	return called, err
}

// Every RPC of the catalog, as every caller role and as an agent, through the real Rego bundle.
func TestRequestAccess_EveryCatalogRPCByRole(t *testing.T) {
	m := loadMatrix(t)
	aud := &recordingAuditor{}
	ic := RequestAccessInterceptor(newAuthorizer(realPolicy(t), newAccessRoles(), aud))

	for method, e := range domain.Catalog {
		if e.Group == domain.GroupInternal {
			continue
		}
		person := map[string]bool{}
		for _, r := range m.PersonAllowed[string(e.Group)] {
			person[r] = true
		}
		for _, role := range m.Roles {
			// "reporter" is a property of one Request; an RPC addressed to a project has no Request to be reporter of.
			if role == "reporter" && e.Locator == domain.LocProjectID {
				person[role] = false
			}
			called, err := run(ic, callerCtx(role, "user"), method, requestFor(t, method))
			if person[role] && (err != nil || !called) {
				t.Errorf("%s as %s should pass, got called=%v err=%v", method, role, called, err)
			}
			if !person[role] && (called || status.Code(err) != codes.PermissionDenied) {
				t.Errorf("%s as %s should be PermissionDenied, got called=%v err=%v", method, role, called, err)
			}
		}
		// The user behind the agent is an owner; an admin behind an agent gets no more than an owner would.
		for _, behind := range []string{"owner", "admin"} {
			called, err := run(ic, callerCtx(behind, "agent"), method, requestFor(t, method))
			want := e.AgentAllowed
			if called != want || (!want && status.Code(err) != codes.PermissionDenied) || (want && err != nil) {
				t.Errorf("%s as agent(%s): called=%v err=%v want allowed=%v", method, behind, called, err, want)
			}
		}
	}
	if len(aud.denied) == 0 {
		t.Error("refusals must be audited")
	}
	for _, d := range aud.denied {
		if d.RPC == "" || d.ActorType == "" {
			t.Errorf("denied audit lacks rpc or actor type: %+v", d)
		}
	}
}

func TestRequestAccess_NotFoundIsTheSameForMissingAndOtherTenant(t *testing.T) {
	ic := RequestAccessInterceptor(newAuthorizer(realPolicy(t), newAccessRoles(), &recordingAuditor{}))
	method := "/orca.request.v1.RequestService/GetRequest"
	missing := requestFor(t, method)
	missing.ProtoReflect().Set(missing.ProtoReflect().Descriptor().Fields().ByName("id"), protoreflect.ValueOfString("77777777-7777-4777-8777-777777777777"))

	_, errMissing := run(ic, callerCtx("owner", "user"), method, missing)
	otherCtx := tenant.WithTenantID(callerCtx("owner", "user"), otherTenant)
	_, errOther := run(ic, otherCtx, method, requestFor(t, method))

	if status.Code(errMissing) != codes.NotFound || status.Code(errOther) != codes.NotFound {
		t.Fatalf("want NotFound for both, got %v and %v", errMissing, errOther)
	}
	if !strings.HasPrefix(status.Convert(errMissing).Message(), "REQUEST_NOT_FOUND") || !strings.HasPrefix(status.Convert(errOther).Message(), "REQUEST_NOT_FOUND") {
		t.Fatalf("both must carry REQUEST_NOT_FOUND: %q %q", status.Convert(errMissing).Message(), status.Convert(errOther).Message())
	}
}

func TestRequestAccess_PolicyErrorIsInternalAndHandlerNotCalled(t *testing.T) {
	ic := RequestAccessInterceptor(newAuthorizer(erroringPolicy{}, newAccessRoles(), &recordingAuditor{}))
	called, err := run(ic, callerCtx("admin", "user"), "/orca.request.v1.RequestService/GetRequest", requestFor(t, "/orca.request.v1.RequestService/GetRequest"))
	if called || status.Code(err) != codes.Internal {
		t.Fatalf("want Internal and no handler call, got called=%v err=%v", called, err)
	}
}

func TestRequestAccess_ProjectServiceDownIsUnavailableNotAllow(t *testing.T) {
	roles := newAccessRoles()
	roles.err = context.DeadlineExceeded
	ic := RequestAccessInterceptor(newAuthorizer(realPolicy(t), roles, &recordingAuditor{}))
	called, err := run(ic, callerCtx("owner", "user"), "/orca.request.v1.RequestService/GetRequest", requestFor(t, "/orca.request.v1.RequestService/GetRequest"))
	if called || status.Code(err) != codes.Unavailable {
		t.Fatalf("want Unavailable, got called=%v err=%v", called, err)
	}
}

func TestRequestAccess_EmptyGlobalRoleIsNotAdmin(t *testing.T) {
	ic := RequestAccessInterceptor(newAuthorizer(realPolicy(t), newAccessRoles(), &recordingAuditor{}))
	ctx := tenant.WithRole(callerCtx("stranger", "user"), "")
	called, err := run(ic, ctx, "/orca.request.v1.RequestService/SetRequestFlowSettings", requestFor(t, "/orca.request.v1.RequestService/SetRequestFlowSettings"))
	if called || status.Code(err) != codes.PermissionDenied {
		t.Fatalf("got called=%v err=%v", called, err)
	}
}

func TestRequestAccess_NoUserIsUnauthenticated(t *testing.T) {
	ic := RequestAccessInterceptor(newAuthorizer(realPolicy(t), newAccessRoles(), &recordingAuditor{}))
	ctx := tenant.WithTenantID(context.Background(), testTenant)
	_, err := run(ic, ctx, "/orca.request.v1.RequestService/GetRequest", requestFor(t, "/orca.request.v1.RequestService/GetRequest"))
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("got %v", err)
	}
}

func TestRequestAccess_HealthAndReflectionAreNotGoverned(t *testing.T) {
	ic := RequestAccessInterceptor(newAuthorizer(realPolicy(t), newAccessRoles(), &recordingAuditor{}))
	for _, m := range []string{"/grpc.health.v1.Health/Check", "/grpc.reflection.v1.ServerReflection/ServerReflectionInfo"} {
		if called, err := run(ic, context.Background(), m, nil); !called || err != nil {
			t.Errorf("%s: called=%v err=%v", m, called, err)
		}
	}
}

func TestRequestAccess_UnknownMethodDenied(t *testing.T) {
	ic := RequestAccessInterceptor(newAuthorizer(realPolicy(t), newAccessRoles(), &recordingAuditor{}))
	called, err := run(ic, callerCtx("admin", "user"), "/orca.request.v1.RequestService/NotInCatalog", nil)
	if called || status.Code(err) != codes.PermissionDenied {
		t.Fatalf("got called=%v err=%v", called, err)
	}
}

func TestRequestAccess_ListScopesToMemberProjects(t *testing.T) {
	const method = "/orca.request.v1.RequestService/ListRequests"
	empty := dynamicpb.NewMessage(requestFor(t, method).ProtoReflect().Descriptor())
	var seen []string
	var scoped bool
	ic := RequestAccessInterceptor(newAuthorizer(realPolicy(t), newAccessRoles(), &recordingAuditor{}))
	handler := func(ctx context.Context, _ any) (any, error) {
		seen, scoped = usecase.ProjectScope(ctx)
		return nil, nil
	}
	call := func(role string) error {
		seen, scoped = nil, false
		_, err := ic(callerCtx(role, "user"), empty, &grpc.UnaryServerInfo{FullMethod: method}, handler)
		return err
	}
	if err := call("owner"); err != nil || !scoped || len(seen) != 1 || seen[0] != testProject {
		t.Fatalf("member lists see only their projects: scoped=%v %v err=%v", scoped, seen, err)
	}
	if err := call("admin"); err != nil || scoped {
		t.Fatalf("admin sees all: scoped=%v err=%v", scoped, err)
	}
	// A user without any project still passes, with an empty scope that yields no rows.
	if err := call("stranger"); err != nil || !scoped || len(seen) != 0 {
		t.Fatalf("stranger gets an empty scope: scoped=%v %v err=%v", scoped, seen, err)
	}
}

func TestRequestAccess_UnregisteredEntityFailsClosedForNonAdmin(t *testing.T) {
	a := usecase.NewAuthorizeRequestAction(accessRequests{}, accessApprovals{}, newAccessRoles(), realPolicy(t), &recordingAuditor{})
	ic := RequestAccessInterceptor(a)
	method := "/orca.request.v1.RequestService/GetClarification"
	if called, err := run(ic, callerCtx("owner", "user"), method, requestFor(t, method)); called || status.Code(err) != codes.NotFound {
		t.Fatalf("owner: got called=%v err=%v", called, err)
	}
	if called, err := run(ic, callerCtx("admin", "user"), method, requestFor(t, method)); !called || err != nil {
		t.Fatalf("admin passes through to the handler: called=%v err=%v", called, err)
	}
}

func TestActorTypeInterceptor_NormalizesMetadata(t *testing.T) {
	for md, want := range map[string]string{"agent": "agent", "system": "system", "": "user", "root": "user", "USER": "user"} {
		ctx := incomingWithActor(md)
		var got string
		_, _ = ActorTypeInterceptor()(ctx, nil, &grpc.UnaryServerInfo{}, func(c context.Context, _ any) (any, error) {
			got = tenant.ActorType(c)
			return nil, nil
		})
		if got != want {
			t.Errorf("metadata %q -> %q, want %q", md, got, want)
		}
	}
}
