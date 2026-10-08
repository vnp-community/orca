package grpc

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func requestDescs() []*grpc.ServiceDesc {
	return []*grpc.ServiceDesc{
		&requestv1.RequestService_ServiceDesc,
		&requestv1.ApprovalService_ServiceDesc,
		&requestv1.ApprovalPolicyAdminService_ServiceDesc,
		&requestv1.AiBudgetAdminService_ServiceDesc,
	}
}

func allProtoMethods() []string {
	var out []string
	for _, d := range requestDescs() {
		for _, m := range d.Methods {
			out = append(out, "/"+d.ServiceName+"/"+m.MethodName)
		}
		for _, s := range d.Streams {
			out = append(out, "/"+d.ServiceName+"/"+s.StreamName)
		}
	}
	sort.Strings(out)
	return out
}

// A new RPC without a row would be silently treated as advancing; this test makes the omission loud.
func TestFlowMethodClass_CoversEveryProtoRPC(t *testing.T) {
	proto := map[string]bool{}
	var missing []string
	for _, m := range allProtoMethods() {
		proto[m] = true
		if _, ok := flowMethodClass[m]; !ok {
			missing = append(missing, m)
		}
	}
	if len(missing) > 0 {
		t.Errorf("RPCs without a flow class (add them to flowMethodClass):\n%s", strings.Join(missing, "\n"))
	}
	var stale []string
	for m := range flowMethodClass {
		if !proto[m] {
			stale = append(stale, m)
		}
	}
	if len(stale) > 0 {
		sort.Strings(stale)
		t.Errorf("flowMethodClass rows for RPCs that are not in the proto:\n%s", strings.Join(stale, "\n"))
	}
}

func TestFlowMethodClass_CriticalRowsMatchCR025(t *testing.T) {
	want := map[string]flowClass{
		requestSvc + "GetRequest": classRead, requestSvc + "ListRequests": classRead, requestSvc + "ListSolutions": classRead,
		requestSvc + "ListBacklog": classRead, requestSvc + "GetRequestFlowSettings": classRead,
		approvalSvc + "ListApprovals": classRead, approvalSvc + "ListPendingForUser": classRead,
		requestSvc + "CancelRequest": classSafeExit, requestSvc + "ReturnToBacklog": classSafeExit,
		approvalSvc + "Reject": classSafeExit, approvalSvc + "Cancel": classSafeExit,
		requestSvc + "CreateRequest": classAdvance, requestSvc + "ClassifyRequest": classAdvance, requestSvc + "ConfirmRequestType": classAdvance,
		requestSvc + "ChangeRequestType": classAdvance, requestSvc + "GenerateSolution": classAdvance, requestSvc + "ChooseSolutionOption": classAdvance,
		requestSvc + "GeneratePlan": classAdvance, requestSvc + "StartPhase": classAdvance, approvalSvc + "Approve": classAdvance,
		requestSvc + "SpawnChildRequest": classAdvance, requestSvc + "ReopenRequest": classAdvance,
		requestSvc + "ReportTaskOutcome": classInternal, requestSvc + "LookupRequestBySource": classInternal,
		requestSvc + "SetRequestFlowSettings": classAdmin,
	}
	for m, c := range want {
		if got := flowMethodClass[m]; got != c {
			t.Errorf("%s: class %d, want %d", m, got, c)
		}
	}
}

func ctxFor(tenantID string) context.Context {
	return tenant.WithTenantID(context.Background(), tenantID)
}

func callGate(t *testing.T, ctx context.Context, method string, effective func(context.Context) (bool, error)) (handled bool, err error) {
	t.Helper()
	_, err = FlowGate(effective)(ctx, nil, &grpc.UnaryServerInfo{FullMethod: method}, func(context.Context, any) (any, error) {
		handled = true
		return nil, nil
	})
	return handled, err
}

func requireFlowDisabled(t *testing.T, err error) {
	t.Helper()
	if status.Code(err) != codes.FailedPrecondition || !strings.Contains(err.Error(), "REQUEST_FLOW_DISABLED") {
		t.Fatalf("want FailedPrecondition REQUEST_FLOW_DISABLED, got %v", err)
	}
}

func TestFlowGate_DisabledBlocksOnlyAdvancingRPCs(t *testing.T) {
	off := func(context.Context) (bool, error) { return false, nil }
	for _, m := range allProtoMethods() {
		handled, err := callGate(t, ctxFor("t1"), m, off)
		if flowMethodClass[m] == classAdvance {
			requireFlowDisabled(t, err)
			if handled {
				t.Errorf("%s: handler ran although the flow is off", m)
			}
			continue
		}
		if err != nil || !handled {
			t.Errorf("%s (class %d): want pass-through while off, got handled=%v err=%v", m, flowMethodClass[m], handled, err)
		}
	}
}

func TestFlowGate_EnabledPassesEveryClass(t *testing.T) {
	on := func(context.Context) (bool, error) { return true, nil }
	for _, m := range allProtoMethods() {
		if handled, err := callGate(t, ctxFor("t1"), m, on); err != nil || !handled {
			t.Errorf("%s: want pass-through while on, got handled=%v err=%v", m, handled, err)
		}
	}
}

func TestFlowGate_ReadErrorFailsClosedForAdvancingOnly(t *testing.T) {
	broken := func(context.Context) (bool, error) { return false, errors.New("db down") }
	_, err := callGate(t, ctxFor("t1"), requestSvc+"CreateRequest", broken)
	requireFlowDisabled(t, err)
	for _, m := range []string{requestSvc + "GetRequest", requestSvc + "CancelRequest", requestSvc + "ReportTaskOutcome", approvalSvc + "Reject"} {
		if handled, err := callGate(t, ctxFor("t1"), m, broken); err != nil || !handled {
			t.Errorf("%s must not depend on the flag: handled=%v err=%v", m, handled, err)
		}
	}
}

func TestFlowGate_UnknownMethodIsAdvancing(t *testing.T) {
	off := func(context.Context) (bool, error) { return false, nil }
	handled, err := callGate(t, ctxFor("t1"), requestSvc+"BrandNewRPC", off)
	requireFlowDisabled(t, err)
	if handled {
		t.Fatal("unclassified RPC must not run while off")
	}
}

func TestFlowGate_FlagIsPerTenant(t *testing.T) {
	perTenant := func(ctx context.Context) (bool, error) {
		id, _ := tenant.TenantID(ctx)
		return id == "tenant-a", nil
	}
	if handled, err := callGate(t, ctxFor("tenant-a"), requestSvc+"CreateRequest", perTenant); err != nil || !handled {
		t.Fatalf("tenant A (on): handled=%v err=%v", handled, err)
	}
	_, err := callGate(t, ctxFor("tenant-b"), requestSvc+"CreateRequest", perTenant)
	requireFlowDisabled(t, err)
}

func TestFlowGate_AdvancingCallWithoutTenantIsRejected(t *testing.T) {
	on := func(context.Context) (bool, error) { return true, nil }
	handled, err := callGate(t, context.Background(), requestSvc+"CreateRequest", on)
	if err == nil || handled {
		t.Fatalf("want rejection without a tenant, got handled=%v err=%v", handled, err)
	}
	if strings.Contains(err.Error(), "REQUEST_FLOW_DISABLED") {
		t.Fatalf("a missing tenant is not a disabled flow: %v", err)
	}
}

type fakeServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (f fakeServerStream) Context() context.Context { return f.ctx }

func TestStreamFlowGate(t *testing.T) {
	off := func(context.Context) (bool, error) { return false, nil }
	run := func(method string) (bool, error) {
		handled := false
		err := StreamFlowGate(off)(nil, fakeServerStream{ctx: ctxFor("t1")}, &grpc.StreamServerInfo{FullMethod: method}, func(any, grpc.ServerStream) error {
			handled = true
			return nil
		})
		return handled, err
	}
	if handled, err := run(requestSvc + "ExportTenantRequests"); err != nil || !handled {
		t.Fatalf("admin stream must pass while off: handled=%v err=%v", handled, err)
	}
	_, err := run(requestSvc + "SomeNewStream")
	requireFlowDisabled(t, err)
}

// Health checks and reflection carry no tenant and are not part of the flow: the gate must not touch them.
func TestFlowGate_InfrastructureServicesPassThrough(t *testing.T) {
	off := func(context.Context) (bool, error) { return false, nil }
	for _, m := range []string{"/grpc.health.v1.Health/Check", "/grpc.health.v1.Health/Watch", "/grpc.reflection.v1.ServerReflection/ServerReflectionInfo"} {
		handled, err := callGate(t, context.Background(), m, off)
		if err != nil || !handled {
			t.Errorf("%s: handled=%v err=%v, want pass-through without a tenant", m, handled, err)
		}
	}
}
