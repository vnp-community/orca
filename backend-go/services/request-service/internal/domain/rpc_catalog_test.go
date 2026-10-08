package domain

import (
	"sort"
	"strings"
	"testing"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

func serviceDescs() []*grpc.ServiceDesc {
	return []*grpc.ServiceDesc{
		&requestv1.RequestService_ServiceDesc,
		&requestv1.ApprovalService_ServiceDesc,
		&requestv1.ApprovalPolicyAdminService_ServiceDesc,
		&requestv1.AiBudgetAdminService_ServiceDesc,
	}
}

// A new RPC in any request.v1 service must be classified before it can ship.
func TestValidateCatalog_AllMethodsClassified(t *testing.T) {
	if err := ValidateCatalog(serviceDescs()...); err != nil {
		t.Fatal(err)
	}
}

// Every service registered under the proto package is in serviceDescs, so a new service cannot hide.
func TestServiceDescsCoverEveryRequestProtoService(t *testing.T) {
	known := map[string]bool{}
	for _, d := range serviceDescs() {
		known[d.ServiceName] = true
	}
	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		if fd.Package() != "orca.request.v1" {
			return true
		}
		for i := 0; i < fd.Services().Len(); i++ {
			if name := string(fd.Services().Get(i).FullName()); !known[name] {
				t.Errorf("service %s is not covered by the RPC catalog", name)
			}
		}
		return true
	})
}

func TestValidateCatalog_DetectsMissing(t *testing.T) {
	extra := requestv1.RequestService_ServiceDesc
	extra.Methods = append(append([]grpc.MethodDesc{}, extra.Methods...), grpc.MethodDesc{MethodName: "BrandNewRpc"})
	err := ValidateCatalog(&extra, &requestv1.ApprovalService_ServiceDesc, &requestv1.ApprovalPolicyAdminService_ServiceDesc, &requestv1.AiBudgetAdminService_ServiceDesc)
	if err == nil || !strings.Contains(err.Error(), "BrandNewRpc") {
		t.Fatalf("want unclassified BrandNewRpc, got %v", err)
	}
}

func TestValidateCatalog_DetectsStaleEntry(t *testing.T) {
	err := ValidateCatalog(&requestv1.ApprovalService_ServiceDesc)
	if err == nil || !strings.Contains(err.Error(), "unknown=") {
		t.Fatalf("catalog entries of services not passed must be reported, got %v", err)
	}
}

// Locator fields must exist in the request message and be strings, or the interceptor would never find the Request.
func TestCatalogFieldsExistInRequestMessages(t *testing.T) {
	for _, d := range serviceDescs() {
		sd, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(d.ServiceName))
		if err != nil {
			t.Fatal(err)
		}
		svc := sd.(protoreflect.ServiceDescriptor)
		for i := 0; i < svc.Methods().Len(); i++ {
			m := svc.Methods().Get(i)
			full := "/" + d.ServiceName + "/" + string(m.Name())
			e := Catalog[full]
			if e.Locator == LocNone {
				if e.Field != "" {
					t.Errorf("%s: Field set without a locator", full)
				}
				continue
			}
			f := m.Input().Fields().ByName(protoreflect.Name(e.Field))
			if f == nil || f.Kind() != protoreflect.StringKind || f.IsList() {
				t.Errorf("%s: field %q is not a string field of %s", full, e.Field, m.Input().FullName())
			}
			if e.Locator == LocEntityID && e.Entity == "" {
				t.Errorf("%s: entity locator without Entity kind", full)
			}
		}
	}
}

// What an MCP agent may call. Anything else must stay false: a person controls approvals, planning and execution.
func TestCatalogAgentMatrix(t *testing.T) {
	allowed := []string{
		"GetRequest", "ListRequests", "ListBacklog", "ListRequestTypeHistory", "ListRequestLinks", "ListSolutions",
		"GetApproval", "ListApprovals", "ListPendingForUser", "GetRequestFlow", "GetRequestFlowSettings",
		"CreateRequest", "SpawnChildRequest", "ClassifyRequest", "ChangeRequestType", "GenerateSolution",
		"ReturnToBacklog", "ReopenRequest",
	}
	denied := []string{
		"Approve", "Reject", "Cancel", "ExtendApproval", "StartPhase", "GeneratePlan", "CommitPlan", "CancelRequest",
		"ConfirmRequestType", "ChooseSolutionOption", "SetRequestFlowSettings", "EraseRequest", "ExportRequest",
		"ExportTenantRequests", "AcceptRisk", "OverrideRiskGate", "WaiveReadiness", "UpsertApprovalPolicy",
		"UpsertAiBudget", "SetAiTenantSettings", "RequestApproval", "ReportTaskOutcome", "UpsertContextSource",
	}
	byName := map[string]Entry{}
	for full, e := range Catalog {
		byName[RPCName(full)] = e
	}
	want := map[string]bool{}
	for _, n := range allowed {
		want[n] = true
		if e, ok := byName[n]; !ok || !e.AgentAllowed {
			t.Errorf("%s must be agent-allowed", n)
		}
	}
	for _, n := range denied {
		if e, ok := byName[n]; !ok || e.AgentAllowed {
			t.Errorf("%s must not be agent-allowed", n)
		}
	}
	var extra []string
	for n, e := range byName {
		if e.AgentAllowed && !want[n] {
			extra = append(extra, n)
		}
	}
	sort.Strings(extra)
	if len(extra) > 0 {
		t.Errorf("agent-allowed RPCs missing from the reviewed list: %v", extra)
	}
}

func TestCatalogInternalNeverAgentAndAdminHasNoLocator(t *testing.T) {
	for full, e := range Catalog {
		if e.Group == GroupInternal && e.AgentAllowed {
			t.Errorf("%s: internal RPC cannot be agent-allowed", full)
		}
		if (e.Group == GroupAdmin || e.Group == GroupInternal) && e.Locator != LocNone {
			t.Errorf("%s: %s group does not locate a Request", full, e.Group)
		}
		switch e.RateClass {
		case RateRead, RateWrite, RateAI, RateWebhook, RateNone:
		default:
			t.Errorf("%s: unknown rate class %q", full, e.RateClass)
		}
		if e.Group == GroupInternal && e.RateClass != RateNone {
			t.Errorf("%s: internal RPCs are not rate limited", full)
		}
	}
}

func TestCatalogRateClassesForAIRPCs(t *testing.T) {
	for _, n := range []string{"GenerateSolution", "GeneratePlan", "ClassifyRequest", "StartPhase"} {
		found := false
		for full, e := range Catalog {
			if RPCName(full) == n {
				found = true
				if e.RateClass != RateAI {
					t.Errorf("%s must be class ai, got %s", n, e.RateClass)
				}
			}
		}
		if !found {
			t.Errorf("%s missing", n)
		}
	}
}

func TestPublicAndInternalMethodsPartitionCatalog(t *testing.T) {
	pub, in := PublicMethods(), InternalMethods()
	if len(pub)+len(in) != len(Catalog) {
		t.Fatalf("public %d + internal %d != catalog %d", len(pub), len(in), len(Catalog))
	}
	for _, m := range in {
		if Catalog[m].Group != GroupInternal {
			t.Errorf("%s listed internal", m)
		}
	}
	if !sort.StringsAreSorted(pub) || !sort.StringsAreSorted(in) {
		t.Error("lists must be sorted for stable wiring")
	}
	if RPCName("/orca.request.v1.RequestService/GetRequest") != "GetRequest" || RPCName("Plain") != "Plain" {
		t.Error("RPCName")
	}
}
