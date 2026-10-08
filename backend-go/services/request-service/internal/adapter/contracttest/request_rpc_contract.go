package contracttest

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/grpcmw"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	requestgrpc "github.com/stablyai/orca-go/services/request-service/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type rpcKit struct {
	env    IntakeEnv
	client requestv1.RequestServiceClient
	ai     *scriptedClassifier
}

func startFullServer(t *testing.T, env IntakeEnv) *rpcKit {
	t.Helper()
	tr := realTransitioner(env)
	ai := &scriptedClassifier{out: proposalOf(domain.RequestTypeBug)}
	create := usecase.NewCreateRequest(env.Requests, env.Idempotency, env.Tx, env.Outbox, tr, &scriptedIssues{})
	propose := usecase.NewProposeRequestClassification(env.Requests, env.History, env.Processed, ai, tr, usecase.NoopApprovalRecorder{}, env.Runs, env.Tx, env.Outbox)
	runner := usecase.NewClassificationRunner(env.Requests, env.Runs, propose, env.Tx, "rpc", time.Minute)
	t.Cleanup(runner.Close)
	canceller, guard := usecase.NoopApprovalCanceller{}, usecase.NoopExecutionGuard{}
	children := usecase.NewIntakeChildCreator(create)
	srv := requestgrpc.NewServer(usecase.NewGetRequest(env.Requests), usecase.NewListRequests(env.Requests)).
		WithIntake(requestgrpc.IntakeUseCases{Create: create, Lookup: usecase.NewLookupRequestBySource(env.Requests, env.Idempotency)}).
		WithClassification(requestgrpc.ClassificationUseCases{
			Runner:  runner,
			Confirm: usecase.NewConfirmRequestType(env.Requests, env.History, tr, usecase.NoopApprovalRecorder{}, env.Tx, env.Outbox),
			Change:  usecase.NewChangeRequestType(env.Requests, env.History, tr, canceller, guard, env.Tx, env.Outbox),
			History: usecase.NewListRequestTypeHistory(env.Requests, env.History),
		}).
		WithLifecycle(requestgrpc.LifecycleUseCases{
			Flow:       usecase.NewGetRequestFlow(),
			Return:     usecase.NewReturnRequestToBacklog(env.Requests, tr, env.Returns, canceller, guard, env.Tx, env.Outbox),
			Reopen:     usecase.NewReopenRequest(env.Requests, tr, env.Returns, usecase.NewClassificationAttemptsReset(env.Requests), env.Tx),
			Cancel:     usecase.NewCancelRequest(env.Requests, tr, env.Returns, canceller, guard, env.Tx),
			SpawnChild: usecase.NewSpawnChildRequest(env.Requests, env.Links, children, env.Tx),
			Links:      usecase.NewListRequestLinks(env.Requests, env.Links),
		})
	lis := bufconn.Listen(1 << 20)
	g := grpc.NewServer(grpc.ChainUnaryInterceptor(grpcmw.TenantExtractionInterceptor()))
	requestv1.RegisterRequestServiceServer(g, srv)
	go func() { _ = g.Serve(lis) }()
	t.Cleanup(g.Stop)
	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return &rpcKit{env: env, client: requestv1.NewRequestServiceClient(conn), ai: ai}
}

func rpcAs(tenantID, userID string) context.Context {
	md := metadata.Pairs(grpcmw.MetadataTenantID, tenantID)
	if userID != "" {
		md.Append(grpcmw.MetadataUserID, userID)
	}
	return metadata.NewOutgoingContext(context.Background(), md)
}

func wantStatus(t *testing.T, err error, code codes.Code, contains string) {
	t.Helper()
	if status.Code(err) != code || (contains != "" && !contains2(status.Convert(err).Message(), contains)) {
		t.Fatalf("want %v containing %q, got %v", code, contains, err)
	}
}

func contains2(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// RunRequestRPCContract drives the intake, classification and lifecycle RPCs over gRPC against
// the dialect's real repositories and the real state machine.
func RunRequestRPCContract(t *testing.T, newEnv func(t *testing.T) IntakeEnv) {
	k := startFullServer(t, newEnv(t))
	scenarios := []struct {
		name string
		fn   func(t *testing.T, k *rpcKit)
	}{
		{"CreateRequest_Manual", rpcCreateManual},
		{"CreateRequest_IdempotentByClientRequestID", rpcCreateIdempotent},
		{"CreateRequest_MissingProjectAndUser", rpcCreateRejections},
		{"CreateRequest_IgnoresTypeHint", rpcCreateIgnoresTypeHint},
		{"ListRequests_FilterBySource", rpcListBySource},
		{"LookupRequestBySource", rpcLookup},
		{"ClassifyRequest_RunIDAndResult", rpcClassify},
		{"ClassifyRequest_NotClassifiable", rpcClassifyRejected},
		{"ConfirmChange_RoundTripAndHistory", rpcConfirmChangeHistory},
		{"ConfirmChange_ActorFromMetadata", rpcActorFromMetadata},
		{"GetRequestFlow", rpcFlow},
		{"ReturnReopenCancel", rpcReturnReopenCancel},
		{"SpawnChildAndLinks", rpcSpawnAndLinks},
		{"SpawnChildConcurrent12SameKey", rpcSpawnConcurrent},
	}
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) { sc.fn(t, k) })
	}
}

func (k *rpcKit) create(t *testing.T, ctx context.Context, title, clientID string) *requestv1.CreateRequestResponse {
	t.Helper()
	resp, err := k.client.CreateRequest(ctx, &requestv1.CreateRequestRequest{
		ProjectId: uuid.NewString(), Title: title, Source: &requestv1.RequestSource{Provider: "manual"}, ClientRequestId: clientID})
	if err != nil {
		t.Fatalf("CreateRequest: %v", err)
	}
	return resp
}

func rpcCreateManual(t *testing.T, k *rpcKit) {
	ctx := rpcAs(newTenant(), uuid.NewString())
	resp := k.create(t, ctx, "from rpc", "")
	r := resp.GetRequest()
	if !resp.GetCreated() || r.GetStatus() != "classifying" || r.GetNumber() != 1 || r.GetTitle() != "from rpc" || r.GetSourceProvider() != "manual" {
		t.Fatalf("%+v", resp)
	}
}

func rpcCreateIdempotent(t *testing.T, k *rpcKit) {
	ctx := rpcAs(newTenant(), uuid.NewString())
	a := k.create(t, ctx, "x", "client-7")
	req := &requestv1.CreateRequestRequest{ProjectId: a.GetRequest().GetProjectId(), Title: "x", Source: &requestv1.RequestSource{Provider: "manual"}, ClientRequestId: "client-7"}
	b, err := k.client.CreateRequest(ctx, req)
	if err != nil || b.GetCreated() || b.GetRequest().GetId() != a.GetRequest().GetId() {
		t.Fatalf("%+v %v", b, err)
	}
}

func rpcCreateRejections(t *testing.T, k *rpcKit) {
	tenantID := newTenant()
	_, err := k.client.CreateRequest(rpcAs(tenantID, uuid.NewString()), &requestv1.CreateRequestRequest{Title: "t", Source: &requestv1.RequestSource{Provider: "manual"}})
	wantStatus(t, err, codes.InvalidArgument, "REQUEST_PROJECT_REQUIRED")
	_, err = k.client.CreateRequest(rpcAs(tenantID, ""), &requestv1.CreateRequestRequest{ProjectId: uuid.NewString(), Title: "t", Source: &requestv1.RequestSource{Provider: "manual"}})
	wantStatus(t, err, codes.InvalidArgument, "REQUEST_REPORTER_REQUIRED")
}

func rpcCreateIgnoresTypeHint(t *testing.T, k *rpcKit) {
	ctx := rpcAs(newTenant(), uuid.NewString())
	resp, err := k.client.CreateRequest(ctx, &requestv1.CreateRequestRequest{ProjectId: uuid.NewString(), Title: "t", Source: &requestv1.RequestSource{Provider: "manual"},
		Hints: &requestv1.SourceHints{IssueType: "Bug", TypeHint: "hotfix", Labels: []string{"a"}}})
	if err != nil {
		t.Fatal(err)
	}
	h := resp.GetRequest().GetSourceHints()
	if h.GetIssueType() != "Bug" || h.GetTypeHint() != "" || len(h.GetLabels()) != 1 {
		t.Fatalf("%+v", h)
	}
}

func rpcListBySource(t *testing.T, k *rpcKit) {
	ctx := rpcAs(newTenant(), uuid.NewString())
	_, err := k.client.CreateRequest(ctx, &requestv1.CreateRequestRequest{ProjectId: uuid.NewString(), Title: "from gh", Source: &requestv1.RequestSource{Provider: "github", Ref: "Acme/Repo#12"}})
	if err != nil {
		t.Fatal(err)
	}
	// Lower-case spelling still finds the request: the filter is normalised like the stored value.
	got, err := k.client.ListRequests(ctx, &requestv1.ListRequestsRequest{SourceProvider: "github", SourceRef: "acme/repo#12"})
	if err != nil || len(got.GetRequests()) != 1 || got.GetRequests()[0].GetSourceRef() != "acme/repo#12" {
		t.Fatalf("%+v %v", got, err)
	}
	none, _ := k.client.ListRequests(ctx, &requestv1.ListRequestsRequest{SourceProvider: "github", SourceRef: "acme/repo#13"})
	if len(none.GetRequests()) != 0 {
		t.Fatal("wrong match")
	}
}

func rpcLookup(t *testing.T, k *rpcKit) {
	ctx := rpcAs(newTenant(), uuid.NewString())
	created, _ := k.client.CreateRequest(ctx, &requestv1.CreateRequestRequest{ProjectId: uuid.NewString(), Title: "t", Source: &requestv1.RequestSource{Provider: "jira", Ref: "eng-4", Site: "https://a.atlassian.net"}})
	got, err := k.client.LookupRequestBySource(ctx, &requestv1.LookupRequestBySourceRequest{Provider: "jira", Site: "https://A.atlassian.net/", Ref: "ENG-4"})
	if err != nil || !got.GetFound() || got.GetRequestId() != created.GetRequest().GetId() {
		t.Fatalf("%+v %v", got, err)
	}
	miss, err := k.client.LookupRequestBySource(ctx, &requestv1.LookupRequestBySourceRequest{Provider: "jira", Ref: "ENG-5"})
	if err != nil || miss.GetFound() {
		t.Fatalf("%+v %v", miss, err)
	}
}

func rpcClassify(t *testing.T, k *rpcKit) {
	ctx := rpcAs(newTenant(), uuid.NewString())
	r := k.create(t, ctx, "needs type", "").GetRequest()
	resp, err := k.client.ClassifyRequest(ctx, &requestv1.ClassifyRequestRequest{RequestId: r.GetId()})
	if err != nil || resp.GetRunId() == "" {
		t.Fatalf("%+v %v", resp, err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		got, err := k.client.GetRequest(ctx, &requestv1.GetRequestRequest{Id: r.GetId()})
		if err != nil {
			t.Fatal(err)
		}
		if got.GetRequest().GetStatus() == "awaiting_type_confirmation" {
			if got.GetRequest().GetType() != "bug" || got.GetRequest().GetClassificationAttempts() < 1 {
				t.Fatalf("%+v", got.GetRequest())
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("never reached awaiting_type_confirmation: %+v", got.GetRequest())
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func rpcClassifyRejected(t *testing.T, k *rpcKit) {
	tenantID := newTenant()
	ctx := rpcAs(tenantID, uuid.NewString())
	r := createRequest(t, k.env.Env, CtxForTenant(tenantID), func(r *domain.Request) { r.Status = domain.RequestStatusExecuting })
	_, err := k.client.ClassifyRequest(ctx, &requestv1.ClassifyRequestRequest{RequestId: r.ID})
	wantStatus(t, err, codes.FailedPrecondition, "REQUEST_NOT_CLASSIFIABLE")
}

func rpcConfirmChangeHistory(t *testing.T, k *rpcKit) {
	tenantID := newTenant()
	user := uuid.NewString()
	ctx := rpcAs(tenantID, user)
	r := createRequest(t, k.env.Env, CtxForTenant(tenantID), func(r *domain.Request) {
		c := 0.8
		r.Status, r.Type, r.TypeSource, r.Confidence, r.Size = domain.RequestStatusAwaitingTypeConfirmation, domain.RequestTypeBug, domain.TypeSourceAI, &c, domain.RequestSizeM
	})
	// An AI proposal row to order against the human rows below.
	if err := k.env.History.Append(CtxForTenant(tenantID), domain.RequestTypeChange{RequestID: r.ID, ToType: domain.RequestTypeBug, ActorID: uuid.Nil.String(), ActorKind: domain.ActorKindAgent, At: time.Now().UTC().Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	conf, err := k.client.ConfirmRequestType(ctx, &requestv1.ConfirmRequestTypeRequest{RequestId: r.ID, Type: "task", Reason: "small", ExpectedVersion: r.Version})
	if err != nil || conf.GetRequest().GetStatus() != "planning" || conf.GetRequest().GetTypeSource() != "human" {
		t.Fatalf("%+v %v", conf, err)
	}
	chg, err := k.client.ChangeRequestType(ctx, &requestv1.ChangeRequestTypeRequest{RequestId: r.ID, NewType: "change_request", Reason: "grew", ExpectedVersion: conf.GetRequest().GetVersion()})
	if err != nil || chg.GetRequest().GetStatus() != "awaiting_type_confirmation" || chg.GetRequest().GetType() != "change_request" {
		t.Fatalf("%+v %v", chg, err)
	}
	hist, err := k.client.ListRequestTypeHistory(ctx, &requestv1.ListRequestTypeHistoryRequest{RequestId: r.ID})
	if err != nil || len(hist.GetChanges()) != 3 {
		t.Fatalf("%+v %v", hist, err)
	}
	c := hist.GetChanges()
	if c[0].GetActorKind() != "ai" || c[1].GetToType() != "task" || c[1].GetActorId() != user || c[2].GetToType() != "change_request" || c[2].GetFromType() != "task" {
		t.Fatalf("%+v", c)
	}
	_, err = k.client.ConfirmRequestType(ctx, &requestv1.ConfirmRequestTypeRequest{RequestId: r.ID, Type: "bug", ExpectedVersion: 1})
	wantStatus(t, err, codes.FailedPrecondition, "REQUEST_VERSION_CONFLICT")
}

func rpcActorFromMetadata(t *testing.T, k *rpcKit) {
	tenantID := newTenant()
	r := createRequest(t, k.env.Env, CtxForTenant(tenantID), func(r *domain.Request) { r.Status = domain.RequestStatusAwaitingTypeConfirmation })
	noUser := rpcAs(tenantID, "")
	_, err := k.client.ConfirmRequestType(noUser, &requestv1.ConfirmRequestTypeRequest{RequestId: r.ID, Type: "task"})
	wantStatus(t, err, codes.InvalidArgument, "REQUEST_REPORTER_REQUIRED")
	_, err = k.client.ChangeRequestType(noUser, &requestv1.ChangeRequestTypeRequest{RequestId: r.ID, NewType: "task", Reason: "x"})
	wantStatus(t, err, codes.InvalidArgument, "REQUEST_REPORTER_REQUIRED")
	_, err = k.client.ConfirmRequestType(rpcAs(newTenant(), uuid.NewString()), &requestv1.ConfirmRequestTypeRequest{RequestId: r.ID, Type: "task"})
	wantStatus(t, err, codes.NotFound, "REQUEST_NOT_FOUND")
}

func rpcFlow(t *testing.T, k *rpcKit) {
	ctx := rpcAs(newTenant(), uuid.NewString())
	l, err := k.client.GetRequestFlow(ctx, &requestv1.GetRequestFlowRequest{Type: "bug", Size: "L"})
	if err != nil || !l.GetHasPhases() || l.GetType() != "bug" || len(l.GetStatusPath()) == 0 || l.GetStatusPath()[0] != "new" || l.GetStatusPath()[len(l.GetStatusPath())-1] != "completed" {
		t.Fatalf("%+v %v", l, err)
	}
	m, _ := k.client.GetRequestFlow(ctx, &requestv1.GetRequestFlowRequest{Type: "bug", Size: "M"})
	if m.GetHasPhases() {
		t.Fatal("bug with size M has no phases")
	}
	h, _ := k.client.GetRequestFlow(ctx, &requestv1.GetRequestFlowRequest{Type: "hotfix"})
	if !h.GetHumanConfirmRequired() {
		t.Fatal("hotfix needs a human confirmation")
	}
	for _, typ := range domain.AllRequestTypes() {
		f, err := k.client.GetRequestFlow(ctx, &requestv1.GetRequestFlowRequest{Type: string(typ)})
		path := f.GetStatusPath()
		if err != nil || len(path) < 4 || path[0] != "new" || path[len(path)-1] != "completed" || f.GetType() != string(typ) {
			t.Fatalf("%s: %+v %v", typ, f, err)
		}
	}
	_, err = k.client.GetRequestFlow(ctx, &requestv1.GetRequestFlowRequest{Type: "nope"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("unknown type: %v", err)
	}
}

func rpcReturnReopenCancel(t *testing.T, k *rpcKit) {
	tenantID := newTenant()
	ctx := rpcAs(tenantID, uuid.NewString())
	r := k.create(t, ctx, "to return", "").GetRequest()
	ret, err := k.client.ReturnToBacklog(ctx, &requestv1.ReturnToBacklogRequest{RequestId: r.GetId(), Stage: "classification", Category: "missing_info", Reason: "need logs", ExpectedVersion: r.GetVersion()})
	if err != nil || ret.GetRequest().GetStatus() != "request_backlog" || ret.GetRequest().GetReturnedCategory() != "missing_info" || ret.GetRequest().GetReturnedFromStage() != "classification" {
		t.Fatalf("%+v %v", ret, err)
	}
	// Burn attempts, then reopen: the counter restarts.
	rr := ret.GetRequest()
	stored, _ := k.env.Requests.Get(CtxForTenant(tenantID), rr.GetId())
	stored.ClassificationAttempts = 4
	if _, err := k.env.Requests.Update(CtxForTenant(tenantID), stored, stored.Version); err != nil {
		t.Fatal(err)
	}
	re, err := k.client.ReopenRequest(ctx, &requestv1.ReopenRequestRequest{RequestId: r.GetId(), Note: "logs attached"})
	if err != nil || re.GetRequest().GetStatus() != "classifying" || re.GetRequest().GetClassificationAttempts() != 0 || re.GetRequest().GetReturnedCategory() != "" {
		t.Fatalf("%+v %v", re, err)
	}
	can, err := k.client.CancelRequest(ctx, &requestv1.CancelRequestRequest{RequestId: r.GetId(), Reason: "not needed"})
	if err != nil || can.GetRequest().GetStatus() != "cancelled" {
		t.Fatalf("%+v %v", can, err)
	}
	_, err = k.client.CancelRequest(rpcAs(tenantID, ""), &requestv1.CancelRequestRequest{RequestId: r.GetId(), Reason: "x"})
	wantStatus(t, err, codes.InvalidArgument, "REQUEST_REPORTER_REQUIRED")
}

func rpcSpawnAndLinks(t *testing.T, k *rpcKit) {
	tenantID := newTenant()
	ctx := rpcAs(tenantID, uuid.NewString())
	parent := createRequest(t, k.env.Env, CtxForTenant(tenantID), func(r *domain.Request) {
		r.Status, r.Type = domain.RequestStatusCompleted, domain.RequestTypeSpike
	})
	in := &requestv1.SpawnChildRequestRequest{ParentRequestId: parent.ID, LinkReason: "spawned_by_spike", Title: "implement it", TypeHint: "change_request", ClientRequestId: "c-1"}
	a, err := k.client.SpawnChildRequest(ctx, in)
	if err != nil || !a.GetCreated() || a.GetChild().GetStatus() != "classifying" || a.GetChild().GetSourceHints().GetTypeHint() != "change_request" {
		t.Fatalf("%+v %v", a, err)
	}
	b, err := k.client.SpawnChildRequest(ctx, in)
	if err != nil || b.GetCreated() || b.GetChild().GetId() != a.GetChild().GetId() {
		t.Fatalf("replay: %+v %v", b, err)
	}
	links, err := k.client.ListRequestLinks(ctx, &requestv1.ListRequestLinksRequest{RequestId: parent.ID})
	if err != nil || len(links.GetChildren()) != 1 || links.GetChildren()[0].GetChildRequestId() != a.GetChild().GetId() || links.GetChildren()[0].GetReason() != "spawned_by_spike" {
		t.Fatalf("%+v %v", links, err)
	}
	up, _ := k.client.ListRequestLinks(ctx, &requestv1.ListRequestLinksRequest{RequestId: a.GetChild().GetId()})
	if len(up.GetParents()) != 1 {
		t.Fatalf("%+v", up)
	}
	bad := &requestv1.SpawnChildRequestRequest{ParentRequestId: parent.ID, LinkReason: "followup_hotfix", Title: "t", ClientRequestId: "c-2"}
	if _, err := k.client.SpawnChildRequest(ctx, bad); status.Code(err) != codes.FailedPrecondition && status.Code(err) != codes.InvalidArgument {
		t.Fatalf("a spike parent cannot spawn a hotfix follow-up: %v", err)
	}
}

func rpcSpawnConcurrent(t *testing.T, k *rpcKit) {
	tenantID := newTenant()
	ctx := rpcAs(tenantID, uuid.NewString())
	parent := createRequest(t, k.env.Env, CtxForTenant(tenantID), func(r *domain.Request) {
		r.Status, r.Type = domain.RequestStatusCompleted, domain.RequestTypeQuestion
	})
	in := &requestv1.SpawnChildRequestRequest{ParentRequestId: parent.ID, LinkReason: "spawned_by_question", Title: "do it", TypeHint: "task", ClientRequestId: "same"}
	const n = 12
	resp := make([]*requestv1.SpawnChildRequestResponse, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = retryDeadlock(func() error {
				var err error
				resp[i], err = k.client.SpawnChildRequest(ctx, in)
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
		if resp[i].GetCreated() {
			created++
		}
		if resp[i].GetChild().GetId() != resp[0].GetChild().GetId() {
			t.Fatal("callers saw different children")
		}
	}
	links, _ := k.client.ListRequestLinks(ctx, &requestv1.ListRequestLinksRequest{RequestId: parent.ID})
	if created != 1 || len(links.GetChildren()) != 1 {
		t.Fatalf("created=%d links=%d", created, len(links.GetChildren()))
	}
}
