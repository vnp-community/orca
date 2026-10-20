package wscompat

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

// SetRequestBaseRef echoes the ref into the Request, as request-service does after a successful write.
func (f *fakeRequestClient) SetRequestBaseRef(ctx context.Context, in *requestv1.SetRequestBaseRefRequest, _ ...grpc.CallOption) (*requestv1.SetRequestBaseRefResponse, error) {
	f.rec(ctx, "SetRequestBaseRef", in)
	if f.err != nil {
		return nil, f.err
	}
	rq := fakeRequestMsg()
	rq.BaseRef, rq.Version = in.GetBaseRef(), in.GetExpectedVersion()+1
	return &requestv1.SetRequestBaseRefResponse{Request: rq}, nil
}

// baseRefClient lets a test choose the Request the create, get and list RPCs return.
type baseRefClient struct {
	*fakeRequestClient
	request    *requestv1.Request
	unverified bool
}

func (c *baseRefClient) CreateRequest(ctx context.Context, in *requestv1.CreateRequestRequest, _ ...grpc.CallOption) (*requestv1.CreateRequestResponse, error) {
	c.rec(ctx, "CreateRequest", in)
	return &requestv1.CreateRequestResponse{Request: c.request, Created: true, BaseRefUnverified: c.unverified}, nil
}

func (c *baseRefClient) GetRequest(ctx context.Context, in *requestv1.GetRequestRequest, _ ...grpc.CallOption) (*requestv1.GetRequestResponse, error) {
	c.rec(ctx, "GetRequest", in)
	return &requestv1.GetRequestResponse{Request: c.request}, nil
}

func (c *baseRefClient) ListRequests(ctx context.Context, in *requestv1.ListRequestsRequest, _ ...grpc.CallOption) (*requestv1.ListRequestsResponse, error) {
	c.rec(ctx, "ListRequests", in)
	return &requestv1.ListRequestsResponse{Requests: []*requestv1.Request{c.request}}, nil
}

func newBaseRefRegistry(c requestv1.RequestServiceClient) *Registry {
	r := NewRegistry()
	registerRequestChannels(r, c, nil, nil, false)
	return r
}

func lockedRequest() *requestv1.Request {
	rq := fakeRequestMsg()
	rq.BaseRef, rq.BaseRefLocked, rq.BaseRefLockReason = "release/2.1", true, "proposal"
	return rq
}

func TestRequestCreate_BaseRefIsPassedAndReturned(t *testing.T) {
	rq := fakeRequestMsg()
	rq.BaseRef = "release/2.1"
	c := &baseRefClient{fakeRequestClient: &fakeRequestClient{}, request: rq, unverified: true}
	res, err := callRequestChannel(t, newBaseRefRegistry(c), testRequestIdentity, context.Background(), "request.create",
		`{"projectId":"p1","title":"T","baseRef":"release/2.1"}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := c.last().in.(*requestv1.CreateRequestRequest).GetBaseRef(); got != "release/2.1" {
		t.Errorf("baseRef sent to the service = %q", got)
	}
	v := res["request"].(map[string]any)
	mustEqual(t, v["baseRef"], "release/2.1")
	mustEqual(t, v["baseRefLocked"], false)
	mustEqual(t, v["baseRefLockReason"], "")
	mustEqual(t, v["baseRefUnverified"], true)
}

func TestRequestCreate_WithoutBaseRefKeepsOldClientsWorking(t *testing.T) {
	c := &baseRefClient{fakeRequestClient: &fakeRequestClient{}, request: fakeRequestMsg()}
	res, err := callRequestChannel(t, newBaseRefRegistry(c), testRequestIdentity, context.Background(), "request.create",
		`{"projectId":"p1","title":"T","body":"B"}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := c.last().in.(*requestv1.CreateRequestRequest).GetBaseRef(); got != "" {
		t.Errorf("an old client's create must send no baseRef, got %q", got)
	}
	v := res["request"].(map[string]any)
	if val, has := v["baseRef"]; !has || val != "" {
		t.Errorf("baseRef = %v (present %v), want the key with an empty value", val, has)
	}
	mustEqual(t, v["baseRefUnverified"], false)
}

func TestRequestGetAndList_AlwaysCarryBaseRefFields(t *testing.T) {
	for _, tc := range []struct {
		name string
		rq   *requestv1.Request
		ref  string
		lock bool
		why  string
	}{
		{"unset", fakeRequestMsg(), "", false, ""},
		{"locked", lockedRequest(), "release/2.1", true, "proposal"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newBaseRefRegistry(&baseRefClient{fakeRequestClient: &fakeRequestClient{}, request: tc.rq})
			get, err := callRequestChannel(t, r, testRequestIdentity, context.Background(), "request.get", `{"id":"r1"}`)
			if err != nil {
				t.Fatal(err)
			}
			list, err := callRequestChannel(t, r, testRequestIdentity, context.Background(), "request.list", `{}`)
			if err != nil {
				t.Fatal(err)
			}
			for name, v := range map[string]map[string]any{
				"request.get":  get["request"].(map[string]any),
				"request.list": list["requests"].([]any)[0].(map[string]any),
			} {
				for key, want := range map[string]any{"baseRef": tc.ref, "baseRefLocked": tc.lock, "baseRefLockReason": tc.why} {
					if got, has := v[key]; !has || got != want {
						t.Errorf("%s: %s = %v (present %v), want %v", name, key, got, has, want)
					}
				}
			}
		})
	}
}

// Responses that do not look up worktrees must not claim "unlocked": the keys are absent, baseRef still shows.
func TestRequestViews_OtherChannelsOmitLockState(t *testing.T) {
	rq := fakeRequestMsg()
	rq.BaseRef = "main"
	r := newRequestTestRegistry(&fakeRequestClient{})
	res, err := callRequestChannel(t, r, testRequestIdentity, context.Background(), "request.cancel", `{"id":"r1","reason":"dup"}`)
	if err != nil {
		t.Fatal(err)
	}
	v := res["request"].(map[string]any)
	if _, has := v["baseRef"]; !has {
		t.Error("baseRef must be on every RequestView")
	}
	for _, key := range []string{"baseRefLocked", "baseRefLockReason", "baseRefUnverified"} {
		if _, has := v[key]; has {
			t.Errorf("%s must be omitted where request-service did not work it out", key)
		}
	}
}

func TestRequestSetBaseRef_Success(t *testing.T) {
	fake := &fakeRequestClient{}
	res, err := callRequestChannel(t, newRequestTestRegistry(fake), testRequestIdentity, context.Background(), "request.setBaseRef",
		`{"id":"r1","baseRef":"release/2.1","expectedVersion":3,"tenantId":"EVIL"}`)
	if err != nil {
		t.Fatal(err)
	}
	in := fake.last().in.(*requestv1.SetRequestBaseRefRequest)
	mustEqual(t, in.GetRequestId(), "r1")
	mustEqual(t, in.GetBaseRef(), "release/2.1")
	mustEqual(t, in.GetExpectedVersion(), int64(3))
	v := res["request"].(map[string]any)
	mustEqual(t, v["baseRef"], "release/2.1")
	mustEqual(t, v["version"], float64(4))
	mustEqual(t, v["baseRefLocked"], false)
	mustEqual(t, v["baseRefUnverified"], false)
	if got := fake.last().md.Get("x-orca-tenant-id"); len(got) == 0 || got[0] != "t1" {
		t.Errorf("tenant metadata = %v, want the session tenant", got)
	}
}

func TestRequestSetBaseRef_EmptyRefResetsToDefault(t *testing.T) {
	fake := &fakeRequestClient{}
	res, err := callRequestChannel(t, newRequestTestRegistry(fake), testRequestIdentity, context.Background(), "request.setBaseRef", `{"id":"r1","baseRef":""}`)
	if err != nil {
		t.Fatal(err)
	}
	mustEqual(t, fake.count(), 1)
	mustEqual(t, res["request"].(map[string]any)["baseRef"], "")
}

func TestRequestSetBaseRef_BadParameters(t *testing.T) {
	for name, args := range map[string]string{
		"missing baseRef (a misspelt key must not reset the ref)": `{"id":"r1","base_ref":"main"}`,
		"null baseRef":      `{"id":"r1","baseRef":null}`,
		"not an object":     `"main"`,
		"baseRef not text":  `{"id":"r1","baseRef":7}`,
		"version not whole": `{"id":"r1","baseRef":"main","expectedVersion":"x"}`,
	} {
		t.Run(name, func(t *testing.T) {
			fake := &fakeRequestClient{}
			_, err := callRequestChannel(t, newRequestTestRegistry(fake), testRequestIdentity, context.Background(), "request.setBaseRef", args)
			if err == nil || !strings.HasPrefix(err.Error(), "INVALID_ARGUMENT:") {
				t.Errorf("err = %v, want INVALID_ARGUMENT", err)
			}
			mustEqual(t, fake.count(), 0)
		})
	}
}

func baseRefStatusErr(code codes.Code, errCode, msg string, meta map[string]string) error {
	st, err := status.New(code, errCode+": "+msg).WithDetails(&errdetails.ErrorInfo{Reason: errCode, Domain: "orca.request.v1", Metadata: meta})
	if err != nil {
		panic(err)
	}
	return st.Err()
}

// mismatchedInfoErr: an ErrorInfo whose reason is not the code in the message must not add a trailer.
func mismatchedInfoErr() error {
	st, err := status.New(codes.FailedPrecondition, "REQUEST_BASE_REF_LOCKED: x").WithDetails(
		&errdetails.ErrorInfo{Reason: "REQUEST_OTHER", Domain: "orca.request.v1", Metadata: map[string]string{"reason": "task"}})
	if err != nil {
		panic(err)
	}
	return st.Err()
}

func TestRequestSetBaseRef_ServiceErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"locked carries the reason", baseRefStatusErr(codes.FailedPrecondition, "REQUEST_BASE_REF_LOCKED", "base ref can no longer change (reason: proposal)", map[string]string{"reason": "proposal"}),
			`REQUEST_BASE_REF_LOCKED: base ref can no longer change (reason: proposal) {"reason":"proposal"}`},
		{"locked by execution", baseRefStatusErr(codes.FailedPrecondition, "REQUEST_BASE_REF_LOCKED", "locked", map[string]string{"reason": "execution"}),
			`REQUEST_BASE_REF_LOCKED: locked {"reason":"execution"}`},
		{"not found carries suggestions, not the ref", baseRefStatusErr(codes.NotFound, "REQUEST_BASE_REF_NOT_FOUND", `base ref "mian" does not exist in the repository`, map[string]string{"ref": "mian", "suggestions": "main,maint"}),
			`REQUEST_BASE_REF_NOT_FOUND: base ref "mian" does not exist in the repository {"suggestions":["main","maint"]}`},
		{"not found without suggestions", baseRefStatusErr(codes.NotFound, "REQUEST_BASE_REF_NOT_FOUND", "gone", map[string]string{"ref": "x"}),
			`REQUEST_BASE_REF_NOT_FOUND: gone`},
		{"invalid", baseRefStatusErr(codes.InvalidArgument, "REQUEST_BASE_REF_INVALID", "base ref must not contain spaces", nil),
			`REQUEST_BASE_REF_INVALID: base ref must not contain spaces`},
		{"version conflict", status.Error(codes.FailedPrecondition, "REQUEST_VERSION_CONFLICT: reload and retry"), `REQUEST_VERSION_CONFLICT: reload and retry`},
		{"not the owner", status.Error(codes.PermissionDenied, "denied"), `REQUEST_FORBIDDEN: not permitted`},
		{"agent actor", status.Error(codes.PermissionDenied, "REQUEST_ACTOR_NOT_ALLOWED: only a person"), `REQUEST_ACTOR_NOT_ALLOWED: only a person`},
		{"request missing", status.Error(codes.NotFound, "REQUEST_NOT_FOUND: no such request"), `REQUEST_NOT_FOUND: no such request`},
		{"details of another code are not trailed", mismatchedInfoErr(), `REQUEST_BASE_REF_LOCKED: x`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := callRequestChannel(t, newRequestTestRegistry(&fakeRequestClient{err: tc.err}), testRequestIdentity, context.Background(), "request.setBaseRef",
				`{"id":"r1","baseRef":"main","expectedVersion":3}`)
			if err == nil || err.Error() != tc.want {
				t.Errorf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestRequestCreate_BaseRefErrorsKeepTheirCodes(t *testing.T) {
	err := baseRefStatusErr(codes.NotFound, "REQUEST_BASE_REF_NOT_FOUND", `base ref "v9" does not exist in the repository`, map[string]string{"ref": "v9", "suggestions": "v1,v2,v3,v4,v5,v6,v7"})
	_, got := callRequestChannel(t, newRequestTestRegistry(&fakeRequestClient{err: err}), testRequestIdentity, context.Background(), "request.create",
		`{"projectId":"p1","title":"T","baseRef":"v9"}`)
	want := `REQUEST_BASE_REF_NOT_FOUND: base ref "v9" does not exist in the repository {"suggestions":["v1","v2","v3","v4","v5"]}`
	if got == nil || got.Error() != want {
		t.Errorf("err = %v, want %q", got, want)
	}
}

func TestRequestFlowStatus_PublishesFeatures(t *testing.T) {
	res, err := callRequestChannel(t, newRequestTestRegistry(&fakeRequestClient{}), testRequestIdentity, context.Background(), "request.flowStatus", `{}`)
	if err != nil {
		t.Fatal(err)
	}
	mustEqual(t, res["enabled"], true)
	feats, ok := res["features"].([]any)
	if !ok || len(feats) != 4 || feats[0] != "request.baseRef" || feats[1] != "approval.runAll" || feats[2] != "request.planAuto" || feats[3] != "request.planRevisions" {
		t.Errorf("features = %v, want [request.baseRef approval.runAll request.planAuto request.planRevisions]", res["features"])
	}
}
