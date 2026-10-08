package contracttest

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/tenant"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	requestgrpc "github.com/stablyai/orca-go/services/request-service/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// SolutionEnv wires one dialect for the CR-REQ-007/008 scenarios, with hooks only SQL can provide.
type SolutionEnv struct {
	IntakeEnv
	SolutionStore usecase.SolutionStore
	AnalysisRuns  usecase.AnalysisRunStore
	// The approval engine's stores: the flows run against the real OpenApproval / DecideApproval.
	Approvals usecase.ApprovalRepository
	Approvers usecase.ApprovalApproverRepository
	Policies  usecase.ApprovalPolicyRepository
	Locker    usecase.RequestLocker
	TxScope   usecase.TxScope
	// RawInsertRun inserts an analysis_runs row bypassing the repository, to probe the unique indexes directly.
	RawInsertRun func(t *testing.T, tenantID, requestID, id, kind, status string, idemKey *string) error
	// ExpireRun moves a running run's lease into the past.
	ExpireRun func(t *testing.T, runID string)
	// TamperOptions rewrites a solution's options behind the application's back.
	TamperOptions func(t *testing.T, solutionID, optionsJSON string)
	// Artifacts carries the CR-REQ-027/028 stores; when Index is set the artifact scenarios run too.
	Artifacts ArtifactEnv
}

func RunSolutionContract(t *testing.T, newEnv func(t *testing.T) SolutionEnv) {
	env := newEnv(t)
	scenarios := []struct {
		name string
		fn   func(t *testing.T, env SolutionEnv)
	}{
		{"RunUniqueIndexes", solRunUniqueIndexes},
		{"StartRunSemantics", solStartRunSemantics},
		{"ConcurrentStartsShareOneRun", solConcurrentStarts},
		{"AgentGateAllowsExactlyTheMaximum", solAgentGate},
		{"LeaseRenewFinishAndClaim", solLease},
		{"SolutionStoreCASSupersedeDeleteDraft", solStoreOperations},
		{"UnicodeAndDigestSurviveTheDatabase", solUnicodeDigest},
		{"TenantIsolation", solTenantIsolation},
		{"SolutionFlowGenerateChooseApprove", solFlowApprove},
		{"RegenerateWithFeedback", solFlowRegenerate},
		{"InvalidOutputLeavesNoDraft", solFlowInvalid},
		{"AgentQuestionCompletesWithoutPlan", solFlowQuestion},
		{"HotfixAutoApproves", solFlowHotfix},
		{"RecoveryFailsInterruptedRun", solRecovery},
		{"PersistRollsBackEverything", solPersistRollback},
		{"RPCFlow", solRPCFlow},
		{"RPCReject", solRPCReject},
		{"ApproverCanChoose", solApproverCanChoose},
		{"SubjectHandlerContract", solSubjectHandlerContract},
	}
	if env.Artifacts.Index != nil {
		scenarios = append(scenarios, struct {
			name string
			fn   func(t *testing.T, env SolutionEnv)
		}{"SolutionArtifactsDecisionsAndGates", solArtifactsDecisionsAndGates})
	}
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) { sc.fn(t, env) })
	}
}

// --- kit ---

type solKit struct {
	env       SolutionEnv
	ctx       context.Context
	tenantID  string
	userID    string
	completer *solCompleter
	agent     *solAgent
	probe     *solProbe
	runner    *usecase.AnalysisRunner
	generate  *usecase.GenerateSolution
	list      *usecase.ListSolutions
	choose    *usecase.ChooseSolutionOption
	handlers  map[domain.SubjectType]usecase.SubjectHandler
	opened    *solOpener
	conn      usecase.AnalysisConnection
	registry  *usecase.SubjectHandlerRegistry
	decide    *usecase.DecideApproval
	client    requestv1.RequestServiceClient
	approvals requestv1.ApprovalServiceClient
}

type solCompleter struct {
	mu      sync.Mutex
	replies []string
	prompts []string
}

func (c *solCompleter) Complete(_ context.Context, _ string, prompt string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := len(c.prompts)
	c.prompts = append(c.prompts, prompt)
	if len(c.replies) == 0 {
		return "", errors.New("no scripted reply")
	}
	return c.replies[min(n, len(c.replies)-1)], nil
}

type solAgent struct {
	mu     sync.Mutex
	reply  string
	inputs []usecase.AgentPromptInput
}

func (a *solAgent) ExecPrompt(_ context.Context, _ usecase.AnalysisConnection, in usecase.AgentPromptInput) (usecase.AgentPromptResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.inputs = append(a.inputs, in)
	return usecase.AgentPromptResult{Stdout: a.reply, AppliedAccessMode: "readonly"}, nil
}

type solProbe struct{}

func (solProbe) Snapshot(context.Context, string) (usecase.RepoSnapshot, error) {
	return usecase.RepoSnapshot{Branch: "main"}, nil
}

// solOpener delegates to the real OpenApproval and records successful opens; fail forces an error before opening.
type solOpener struct {
	mu    sync.Mutex
	inner usecase.SolutionApprovalOpener
	list  []usecase.OpenSolutionApprovalInput
	fail  error
}

func (o *solOpener) Open(ctx context.Context, in usecase.OpenSolutionApprovalInput) error {
	o.mu.Lock()
	fail := o.fail
	o.mu.Unlock()
	if fail != nil {
		return fail
	}
	if err := o.inner.Open(ctx, in); err != nil {
		return err
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.list = append(o.list, in)
	return nil
}

func (o *solOpener) count() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return len(o.list)
}

type solConns struct{ conn usecase.AnalysisConnection }

func (c solConns) ResolveForProject(context.Context, string) (usecase.AnalysisConnection, error) {
	return c.conn, nil
}

func validSolutionDoc(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("../../domain/testdata/solution_options/valid_two_options.json")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	return string(b)
}

func analysisDoc(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("../../domain/testdata/analysis_documents/" + name)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	return string(b)
}

func newSolKit(t *testing.T, env SolutionEnv) *solKit { return newSolKitWith(t, env, false) }

// newSolKitWith with artifacts=true also wires provenance, coverage checks, the Decision record and the approval gates.
func newSolKitWith(t *testing.T, env SolutionEnv, artifacts bool) *solKit {
	t.Helper()
	tenantID := newTenant()
	userID := uuid.NewString()
	k := &solKit{
		env: env, tenantID: tenantID, userID: userID, ctx: tenant.WithUserID(CtxForTenant(tenantID), userID),
		completer: &solCompleter{}, agent: &solAgent{}, probe: &solProbe{},
		conn: usecase.AnalysisConnection{ConnectionID: "c1", RepoPath: "/srv/repo", WorktreeID: "wt-1"},
	}
	tr := realTransitioner(env.IntakeEnv)
	settings := usecase.DefaultAnalysisSettings()
	settings.Heartbeat = time.Hour

	// The real approval engine, with only this feature's handlers registered.
	k.registry = usecase.NewSubjectHandlerRegistry()
	canceler := &usecase.CancelPendingApprovalsForRequest{Repo: env.Approvals, Tx: env.Tx, Locker: env.Locker, Registry: k.registry, Outbox: env.Outbox}
	canceller := &usecase.PendingApprovalCanceller{Inner: canceler}
	ret := usecase.NewReturnRequestToBacklog(env.Requests, tr, env.Returns, canceller, usecase.NoActiveExecutionGuard{}, env.Tx, env.Outbox)
	deps := usecase.AnalysisApprovalHandlerDeps{Requests: env.Requests, Solutions: env.SolutionStore, Returner: ret, Transition: tr, Outbox: env.Outbox}
	var recorder *usecase.SolutionArtifactRecorder
	var recordDecision *usecase.RecordDecision
	if artifacts {
		recorder = usecase.NewSolutionArtifactRecorder(usecase.NewMintArtifactIDs(env.Artifacts.Index), env.Artifacts.Relations).WithDecisions(env.Artifacts.Decisions)
		recordDecision = usecase.NewRecordDecision(env.Artifacts.Decisions, env.Outbox).WithHighRiskServices(3)
		deps.Gates, deps.Coverage, deps.Decisions = usecase.NewApprovalGates(env.Artifacts.Decisions, env.Artifacts.Clarifications), recorder, recorder
	}
	k.handlers = map[domain.SubjectType]usecase.SubjectHandler{
		domain.SubjectSolution: usecase.NewSolutionApprovalHandler(deps),
		domain.SubjectFindings: usecase.NewFindingsApprovalHandler(deps),
		domain.SubjectAnswer:   usecase.NewAnswerApprovalHandler(deps),
	}
	for st, h := range k.handlers {
		k.registry.Register(st, h)
	}
	teams, admins := &rigTeams{teamsOf: map[string][]string{}, members: map[string][]string{}}, &rigAdmins{}
	open := &usecase.OpenApproval{Repo: env.Approvals, Tx: env.TxScope, Requests: env.Requests, Locker: env.Locker, Registry: k.registry,
		Resolver: &usecase.ResolveApproverPolicy{Repo: env.Policies}, ApproverRepo: env.Approvers, Outbox: env.Outbox, Teams: teams, Admins: admins}
	k.opened = &solOpener{inner: usecase.OpenApprovalOpener{Approvals: open}}
	expire := &usecase.ExpireApprovals{Repo: env.Approvals, Tx: env.Tx, Locker: env.Locker, Registry: k.registry, Returner: ret, Outbox: env.Outbox}
	k.decide = &usecase.DecideApproval{Repo: env.Approvals, Tx: env.Tx, Locker: env.Locker, Registry: k.registry, Outbox: env.Outbox, Expirer: expire,
		Authorizer: &usecase.AuthorizeApprovalDecision{ApproverRepo: env.Approvers, Teams: teams}}

	writer := usecase.NewAnalysisResultWriter(env.Requests, env.SolutionStore, env.AnalysisRuns, k.opened, tr, env.Tx, env.Outbox, "worker-1")
	gen := usecase.NewRunSolutionGeneration(env.Requests, env.SolutionStore, k.completer, nil, writer, settings)
	if artifacts {
		writer.WithArtifacts(recorder)
		gen.WithCoverage(recorder)
	}
	readonly := usecase.NewRunAgentReadonlyAnalysis(usecase.AgentReadonlyDeps{
		Requests: env.Requests, Solutions: env.SolutionStore, Conns: solConns{k.conn}, Agent: k.agent, Probe: k.probe, Writer: writer, Settings: settings,
	})
	k.runner = usecase.NewAnalysisRunner(env.AnalysisRuns, env.SolutionStore, env.Tx, gen, readonly, settings, "worker-1")
	t.Cleanup(k.runner.Close)
	auth := usecase.ApproverAwareAuthorizer{Approvals: env.Approvals, Decider: &usecase.AuthorizeApprovalDecision{ApproverRepo: env.Approvers, Teams: teams}}
	k.generate = usecase.NewGenerateSolution(usecase.GenerateSolutionDeps{
		Requests: env.Requests, Solutions: env.SolutionStore, Runs: env.AnalysisRuns, Conns: solConns{k.conn}, Auth: auth,
		Approvals: canceller, Transition: tr, Tx: env.Tx, Spawner: k.runner, Settings: settings, LeaseOwner: "worker-1",
	})
	k.list = usecase.NewListSolutions(env.Requests, env.SolutionStore, env.AnalysisRuns)
	k.choose = usecase.NewChooseSolutionOption(env.Requests, env.SolutionStore, env.Approvals, auth, env.Tx)
	if artifacts {
		k.choose.WithDecisions(recordDecision, nil)
		k.generate.WithDecisions(recorder)
	}
	k.startServer(t)
	return k
}

// startServer serves RequestService (solution RPCs) and ApprovalService over bufconn, as the gateway would call them.
func (k *solKit) startServer(t *testing.T) {
	t.Helper()
	env := k.env
	srv := requestgrpc.NewServer(usecase.NewGetRequest(env.Requests), usecase.NewListRequests(env.Requests)).
		WithSolution(requestgrpc.SolutionUseCases{Generate: k.generate, List: k.list, Choose: k.choose})
	approvalSrv := requestgrpc.NewApprovalServer(requestgrpc.ApprovalUseCases{
		Decide: k.decide, Get: &usecase.GetApproval{Repo: env.Approvals}, List: &usecase.ListApprovals{Repo: env.Approvals},
	})
	lis := bufconn.Listen(1 << 20)
	g := grpc.NewServer(grpc.ChainUnaryInterceptor(grpcmw.TenantExtractionInterceptor()))
	requestv1.RegisterRequestServiceServer(g, srv)
	requestv1.RegisterApprovalServiceServer(g, approvalSrv)
	go func() { _ = g.Serve(lis) }()
	t.Cleanup(g.Stop)
	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	k.client, k.approvals = requestv1.NewRequestServiceClient(conn), requestv1.NewApprovalServiceClient(conn)
}

// pendingApproval returns the single pending approval of a subject, as the engine stored it.
func (k *solKit) pendingApproval(t *testing.T, st domain.SubjectType, solutionID string) domain.Approval {
	t.Helper()
	a, err := k.env.Approvals.FindPendingBySubject(k.ctx, k.tenantID, st, solutionID)
	if err != nil || a == nil {
		t.Fatalf("no pending %s approval for %s: %v", st, solutionID, err)
	}
	return *a
}

func (k *solKit) seed(t *testing.T, typ domain.RequestType, status domain.RequestStatus) domain.Request {
	t.Helper()
	return seedRequest(t, k.env.IntakeEnv, k.ctx, status, func(r *domain.Request) {
		r.Type, r.TypeSource, r.Size, r.ReporterID = typ, domain.TypeSourceHuman, domain.RequestSizeM, k.userID
	})
}

func (k *solKit) waitRun(t *testing.T, runID string) domain.AnalysisRun {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		run, err := k.env.AnalysisRuns.Get(k.ctx, runID)
		if err != nil {
			t.Fatal(err)
		}
		if run.Status != domain.RunStatusRunning {
			return run
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("run %s still running after 15s", runID)
	return domain.AnalysisRun{}
}

func (k *solKit) requestStatus(t *testing.T, id string) domain.RequestStatus {
	t.Helper()
	r, err := k.env.Requests.Get(k.ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	return r.Status
}

func (k *solKit) rpc() context.Context { return rpcAs(k.tenantID, k.userID) }

// approveRPC approves through ApprovalService, the way the UI does, sending the digest the UI last saw.
func (k *solKit) approveRPC(a domain.Approval, digest string) (*requestv1.ApproveResponse, error) {
	return k.approvals.Approve(k.rpc(), &requestv1.ApproveRequest{Id: a.ID, Comment: "ok", ExpectedDigest: digest})
}

func solutionsOf(t *testing.T, k *solKit, requestID string) []domain.Solution {
	t.Helper()
	list, err := k.env.SolutionStore.ListByRequest(k.ctx, usecase.SolutionListFilter{RequestID: requestID})
	if err != nil {
		t.Fatal(err)
	}
	return list
}

// --- repository scenarios ---

func solRunUniqueIndexes(t *testing.T, env SolutionEnv) {
	k := newSolKit(t, env)
	r := k.seed(t, domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)
	insert := func(kind, status string, key *string) error {
		return env.RawInsertRun(t, k.tenantID, r.ID, uuid.NewString(), kind, status, key)
	}
	if err := insert("solution", "running", nil); err != nil {
		t.Fatalf("first running run: %v", err)
	}
	if err := insert("solution", "running", nil); err == nil {
		t.Fatal("a second running run for the same (request, kind) must be rejected")
	}
	if err := insert("diagnosis", "running", nil); err != nil {
		t.Fatalf("a running run of another kind must be accepted: %v", err)
	}
	// Finished runs do not count, however many there are.
	for i := 0; i < 3; i++ {
		if err := insert("solution", "failed", nil); err != nil {
			t.Fatalf("failed run %d: %v", i, err)
		}
	}
	// NULL idempotency keys never collide; a repeated real key does.
	if err := insert("findings", "failed", nil); err != nil {
		t.Fatal(err)
	}
	key := "k-" + uuid.NewString()
	if err := insert("answer", "failed", &key); err != nil {
		t.Fatal(err)
	}
	if err := insert("findings", "failed", &key); err == nil {
		t.Fatal("the same idempotency key twice for one request must be rejected")
	}
	other := k.seed(t, domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)
	if err := env.RawInsertRun(t, k.tenantID, other.ID, uuid.NewString(), "answer", "failed", &key); err != nil {
		t.Fatalf("the same key on another request is fine: %v", err)
	}
}

func startInput(t *testing.T, k *solKit, r domain.Request, mode domain.AnalysisMode, kind domain.RunKind, key string) (domain.AnalysisRun, domain.Solution) {
	t.Helper()
	owner := "w-test"
	run := domain.AnalysisRun{ID: uuid.NewString(), TenantID: k.tenantID, RequestID: r.ID, Kind: kind, Mode: mode, Status: domain.RunStatusRunning, Attempt: 1,
		LeaseOwner: &owner, SolutionID: uuid.NewString(), ProjectID: r.ProjectID, ActorID: k.userID}
	if key != "" {
		run.IdempotencyKey = &key
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	draft := domain.Solution{ID: run.SolutionID, TenantID: k.tenantID, RequestID: r.ID, Kind: domain.SolutionKind(kind), Status: domain.SolutionStatusDraft,
		OptionsJSON: []byte(`{}`), GenerationRunID: run.ID, Version: 1, CreatedAt: now, UpdatedAt: now}
	return run, draft
}

func startIn(t *testing.T, k *solKit, run domain.AnalysisRun, draft domain.Solution, max int) (usecase.StartRunResult, error) {
	t.Helper()
	var res usecase.StartRunResult
	err := k.env.Tx.InTx(k.ctx, func(ctx context.Context) error {
		var err error
		res, err = k.env.AnalysisRuns.StartRun(ctx, run, draft, usecase.StartRunOptions{LeaseTTL: time.Minute, MaxAgentRuns: max})
		return err
	})
	return res, err
}

func solStartRunSemantics(t *testing.T, env SolutionEnv) {
	k := newSolKit(t, env)
	r := k.seed(t, domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)
	run, draft := startInput(t, k, r, domain.AnalysisModeComplete, domain.RunKindSolution, "key-1")
	res, err := startIn(t, k, run, draft, 0)
	if err != nil || !res.Created || res.Run.ID != run.ID || res.Run.LeaseExpiresAt == nil || !res.Run.LeaseExpiresAt.After(time.Now().Add(30*time.Second)) {
		t.Fatalf("first start: %+v %v", res, err)
	}
	if res.Run.SolutionID != draft.ID || res.Run.ProjectID != r.ProjectID || res.Run.ActorID != k.userID {
		t.Fatalf("run columns not stored: %+v", res.Run)
	}
	stored, err := env.SolutionStore.Get(k.ctx, draft.ID)
	if err != nil || stored.Status != domain.SolutionStatusDraft || stored.GenerationRunID != run.ID || stored.Kind != domain.SolutionKindSolution {
		t.Fatalf("draft = %+v %v", stored, err)
	}

	// A second start for the same request and kind returns the running run and writes nothing.
	run2, draft2 := startInput(t, k, r, domain.AnalysisModeComplete, domain.RunKindSolution, "")
	res2, err := startIn(t, k, run2, draft2, 0)
	if err != nil || res2.Created || res2.Run.ID != run.ID {
		t.Fatalf("second start: %+v %v", res2, err)
	}
	if _, err := env.SolutionStore.Get(k.ctx, draft2.ID); err == nil {
		t.Fatal("the losing start left a draft behind")
	}

	// Finish it; the same idempotency key still answers with the original run, a fresh key starts a new one.
	failed := res.Run
	failed.Fail("X", "x", time.Now())
	if ok, err := env.AnalysisRuns.FinishOwned(k.ctx, failed, "w-test"); err != nil || !ok {
		t.Fatalf("finish: %v %v", ok, err)
	}
	run3, draft3 := startInput(t, k, r, domain.AnalysisModeComplete, domain.RunKindSolution, "key-1")
	res3, err := startIn(t, k, run3, draft3, 0)
	if err != nil || res3.Created || res3.Run.ID != run.ID {
		t.Fatalf("same key after finish: %+v %v", res3, err)
	}
	run4, draft4 := startInput(t, k, r, domain.AnalysisModeComplete, domain.RunKindSolution, "key-2")
	res4, err := startIn(t, k, run4, draft4, 0)
	if err != nil || !res4.Created {
		t.Fatalf("new key after finish: %+v %v", res4, err)
	}
}

func solConcurrentStarts(t *testing.T, env SolutionEnv) {
	k := newSolKit(t, env)
	r := k.seed(t, domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)
	const n = 12
	var wg sync.WaitGroup
	ids := make([]string, n)
	created := make([]bool, n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			run, draft := startInput(t, k, r, domain.AnalysisModeComplete, domain.RunKindSolution, "")
			errs[i] = retryDeadlock(func() error {
				res, err := startIn(t, k, run, draft, 0)
				ids[i], created[i] = res.Run.ID, res.Created
				return err
			})
		}()
	}
	wg.Wait()
	winners := 0
	for i := range ids {
		if errs[i] != nil {
			t.Fatalf("start %d: %v", i, errs[i])
		}
		if created[i] {
			winners++
		}
		if ids[i] != ids[0] {
			t.Fatalf("run ids differ: %v", ids)
		}
	}
	if winners != 1 {
		t.Fatalf("%d starts created a run, want exactly 1", winners)
	}
	drafts := 0
	for _, s := range solutionsOf(t, k, r.ID) {
		if s.Status == domain.SolutionStatusDraft {
			drafts++
		}
	}
	if drafts != 1 {
		t.Fatalf("%d drafts, want 1", drafts)
	}
}

func solAgentGate(t *testing.T, env SolutionEnv) {
	k := newSolKit(t, env)
	project := uuid.NewString()
	const n, max = 8, 2
	reqs := make([]domain.Request, n)
	for i := range reqs {
		reqs[i] = seedRequest(t, env.IntakeEnv, k.ctx, domain.RequestStatusAnalyzing, func(r *domain.Request) {
			r.Type, r.TypeSource, r.Size, r.ProjectID, r.ReporterID = domain.RequestTypeQuestion, domain.TypeSourceHuman, domain.RequestSizeM, project, k.userID
		})
	}
	if err := env.AnalysisRuns.EnsureProjectGate(k.ctx, project); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	created, busy := 0, 0
	for i := range reqs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			run, draft := startInput(t, k, reqs[i], domain.AnalysisModeAgentReadonly, domain.RunKindAnswer, "")
			res, err := startIn(t, k, run, draft, max)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil && res.Created:
				created++
			case err != nil && errCode(err) == "REQUEST_ANALYSIS_BUSY":
				busy++
			default:
				t.Errorf("unexpected outcome: %+v %v", res, err)
			}
		}()
	}
	wg.Wait()
	if created != max || busy != n-max {
		t.Fatalf("created=%d busy=%d, want %d and %d", created, busy, max, n-max)
	}
	running, err := env.AnalysisRuns.CountRunning(k.ctx, project, domain.AnalysisModeAgentReadonly)
	if err != nil || running != max {
		t.Fatalf("CountRunning = %d %v", running, err)
	}
	// A refused start leaves no run and no draft: only the 2 admitted requests have anything.
	rows := 0
	for _, r := range reqs {
		rows += len(solutionsOf(t, k, r.ID))
	}
	if rows != max {
		t.Fatalf("%d drafts exist, want %d", rows, max)
	}
	// Another project is unaffected by this one's cap.
	other := seedRequest(t, env.IntakeEnv, k.ctx, domain.RequestStatusAnalyzing, func(r *domain.Request) {
		r.Type, r.TypeSource, r.Size, r.ReporterID = domain.RequestTypeQuestion, domain.TypeSourceHuman, domain.RequestSizeM, k.userID
	})
	if err := env.AnalysisRuns.EnsureProjectGate(k.ctx, other.ProjectID); err != nil {
		t.Fatal(err)
	}
	run, draft := startInput(t, k, other, domain.AnalysisModeAgentReadonly, domain.RunKindAnswer, "")
	if res, err := startIn(t, k, run, draft, max); err != nil || !res.Created {
		t.Fatalf("other project: %+v %v", res, err)
	}
}

func solLease(t *testing.T, env SolutionEnv) {
	k := newSolKit(t, env)
	var runs []domain.AnalysisRun
	for i := 0; i < 6; i++ {
		r := k.seed(t, domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)
		run, draft := startInput(t, k, r, domain.AnalysisModeComplete, domain.RunKindSolution, "")
		if _, err := startIn(t, k, run, draft, 0); err != nil {
			t.Fatal(err)
		}
		runs = append(runs, run)
	}
	a := runs[0]
	if ok, err := env.AnalysisRuns.RenewLease(k.ctx, a.ID, "someone-else", time.Minute); err != nil || ok {
		t.Fatalf("renew by a stranger = %v %v", ok, err)
	}
	if ok, err := env.AnalysisRuns.RenewLease(k.ctx, a.ID, "w-test", time.Minute); err != nil || !ok {
		t.Fatalf("renew by the owner = %v %v", ok, err)
	}
	done := a
	done.Succeed(time.Now())
	if ok, err := env.AnalysisRuns.FinishOwned(k.ctx, done, "someone-else"); err != nil || ok {
		t.Fatalf("finish by a stranger = %v %v", ok, err)
	}
	if ok, err := env.AnalysisRuns.FinishOwned(k.ctx, done, "w-test"); err != nil || !ok {
		t.Fatalf("finish by the owner = %v %v", ok, err)
	}
	got, _ := env.AnalysisRuns.Get(k.ctx, a.ID)
	if got.Status != domain.RunStatusSucceeded || got.LeaseOwner != nil || got.FinishedAt == nil {
		t.Fatalf("finished run = %+v", got)
	}
	if ok, _ := env.AnalysisRuns.RenewLease(k.ctx, a.ID, "w-test", time.Minute); ok {
		t.Fatal("a finished run cannot be renewed")
	}

	// Five runs expire; two sweepers racing must never claim the same one, and a live run is never claimed.
	for _, r := range runs[1:5] {
		env.ExpireRun(t, r.ID)
	}
	env.ExpireRun(t, runs[5].ID)
	var wg sync.WaitGroup
	claims := make([][]domain.AnalysisRun, 2)
	errs := make([]error, 2)
	for i, owner := range []string{"sweeper-a", "sweeper-b"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = retryDeadlock(func() error {
				var err error
				claims[i], err = env.AnalysisRuns.ClaimExpired(context.Background(), owner, time.Minute, 3)
				return err
			})
		}()
	}
	wg.Wait()
	seen := map[string]string{}
	for i, list := range claims {
		if errs[i] != nil {
			t.Fatalf("claim %d: %v", i, errs[i])
		}
		for _, run := range list {
			if prev, dup := seen[run.ID]; dup {
				t.Fatalf("run %s claimed by %s and again by sweeper %d", run.ID, prev, i)
			}
			seen[run.ID] = fmt.Sprint(i)
			if run.TenantID != k.tenantID || run.SolutionID == "" || run.ActorID == "" {
				t.Fatalf("claimed run lacks what recovery needs: %+v", run)
			}
		}
	}
	if len(seen) != 5 {
		t.Fatalf("claimed %d of 5 expired runs (batch 3 x 2 sweepers)", len(seen))
	}
	after, _ := env.AnalysisRuns.ClaimExpired(context.Background(), "sweeper-c", time.Minute, 10)
	if len(after) != 0 {
		t.Fatalf("claimed runs must have a fresh lease, got %d more", len(after))
	}
}

func solStoreOperations(t *testing.T, env SolutionEnv) {
	k := newSolKit(t, env)
	r := k.seed(t, domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)
	now := time.Now().UTC().Truncate(time.Microsecond)
	mk := func(kind domain.SolutionKind, status domain.SolutionStatus, at time.Time) domain.Solution {
		s := domain.Solution{ID: uuid.NewString(), TenantID: k.tenantID, RequestID: r.ID, Kind: kind, Status: status, OptionsJSON: []byte(`{"a":1}`), Version: 1, CreatedAt: at, UpdatedAt: at}
		if err := env.SolutionStore.Insert(k.ctx, s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	proposed := mk(domain.SolutionKindSolution, domain.SolutionStatusProposed, now)
	rejected := mk(domain.SolutionKindSolution, domain.SolutionStatusRejected, now.Add(time.Second))
	approved := mk(domain.SolutionKindSolution, domain.SolutionStatusApproved, now.Add(2*time.Second))
	diag := mk(domain.SolutionKindDiagnosis, domain.SolutionStatusProposed, now.Add(3*time.Second))
	draft := mk(domain.SolutionKindSolution, domain.SolutionStatusDraft, now.Add(4*time.Second))

	if ok, err := env.SolutionStore.Choose(k.ctx, proposed.ID, 1, 99); err != nil || ok {
		t.Fatalf("choose with a stale version = %v %v", ok, err)
	}
	if ok, err := env.SolutionStore.Choose(k.ctx, approved.ID, 1, 1); err != nil || ok {
		t.Fatalf("choose on a non-proposed solution = %v %v", ok, err)
	}
	if ok, err := env.SolutionStore.Choose(k.ctx, proposed.ID, 1, 1); err != nil || !ok {
		t.Fatalf("choose = %v %v", ok, err)
	}
	got, _ := env.SolutionStore.Get(k.ctx, proposed.ID)
	if got.ChosenOption == nil || *got.ChosenOption != 1 || got.Version != 2 {
		t.Fatalf("after choose: %+v", got)
	}
	if ok, _ := env.SolutionStore.Choose(k.ctx, proposed.ID, 0, 1); ok {
		t.Fatal("the version moved on; the old one must lose")
	}

	n, err := env.SolutionStore.SupersedeOpen(k.ctx, r.ID, domain.SolutionKindSolution, draft.ID)
	if err != nil || n != 2 {
		t.Fatalf("supersede open = %d %v (proposed + rejected)", n, err)
	}
	for id, want := range map[string]domain.SolutionStatus{
		proposed.ID: domain.SolutionStatusSuperseded, rejected.ID: domain.SolutionStatusSuperseded, approved.ID: domain.SolutionStatusApproved,
		diag.ID: domain.SolutionStatusProposed, draft.ID: domain.SolutionStatusDraft,
	} {
		if s, _ := env.SolutionStore.Get(k.ctx, id); s.Status != want {
			t.Fatalf("solution %s = %s, want %s", id, s.Status, want)
		}
	}

	if err := env.SolutionStore.DeleteDraft(k.ctx, approved.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := env.SolutionStore.Get(k.ctx, approved.ID); err != nil {
		t.Fatal("DeleteDraft must not delete a non-draft")
	}
	if err := env.SolutionStore.DeleteDraft(k.ctx, draft.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := env.SolutionStore.Get(k.ctx, draft.ID); err == nil {
		t.Fatal("draft survived DeleteDraft")
	}

	byKind, _ := env.SolutionStore.ListByRequest(k.ctx, usecase.SolutionListFilter{RequestID: r.ID, Kind: domain.SolutionKindDiagnosis})
	byStatus, _ := env.SolutionStore.ListByRequest(k.ctx, usecase.SolutionListFilter{RequestID: r.ID, Status: domain.SolutionStatusSuperseded})
	limited, _ := env.SolutionStore.ListByRequest(k.ctx, usecase.SolutionListFilter{RequestID: r.ID, Limit: 2})
	if len(byKind) != 1 || len(byStatus) != 2 || len(limited) != 2 || limited[0].ID != proposed.ID {
		t.Fatalf("filters: kind=%d status=%d limit=%d", len(byKind), len(byStatus), len(limited))
	}
}

func solUnicodeDigest(t *testing.T, env SolutionEnv) {
	k := newSolKit(t, env)
	r := k.seed(t, domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)
	doc := validSolutionDoc(t)
	before, err := domain.DigestOptions([]byte(doc), nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	s := domain.Solution{ID: uuid.NewString(), TenantID: k.tenantID, RequestID: r.ID, Kind: domain.SolutionKindSolution, Status: domain.SolutionStatusProposed,
		OptionsJSON: []byte(doc), Version: 1, CreatedAt: now, UpdatedAt: now}
	if err := env.SolutionStore.Insert(k.ctx, s); err != nil {
		t.Fatal(err)
	}
	got, err := env.SolutionStore.Get(k.ctx, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got.OptionsJSON), "Thêm lớp cache ở tầng dịch vụ") {
		t.Fatalf("Vietnamese text did not survive: %s", got.OptionsJSON)
	}
	after, err := domain.DigestOptions(got.OptionsJSON, nil)
	if err != nil || after != before {
		t.Fatalf("digest changed through the database: %s -> %s (%v)", before, after, err)
	}
	opts, err := domain.ParseSolutionOptions(got.OptionsJSON)
	if err != nil || opts.Validate(2) != nil {
		t.Fatalf("stored document no longer valid: %v", err)
	}
}

func solTenantIsolation(t *testing.T, env SolutionEnv) {
	k := newSolKit(t, env)
	r := k.seed(t, domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)
	run, draft := startInput(t, k, r, domain.AnalysisModeComplete, domain.RunKindSolution, "")
	if _, err := startIn(t, k, run, draft, 0); err != nil {
		t.Fatal(err)
	}
	foreign := CtxForTenant(newTenant())
	if _, err := env.SolutionStore.Get(foreign, draft.ID); errCode(err) != "SOLUTION_NOT_FOUND" {
		t.Fatalf("foreign Get = %v", err)
	}
	if list, _ := env.SolutionStore.ListByRequest(foreign, usecase.SolutionListFilter{RequestID: r.ID}); len(list) != 0 {
		t.Fatal("foreign list sees solutions")
	}
	if _, err := env.AnalysisRuns.Get(foreign, run.ID); err == nil {
		t.Fatal("foreign Get of a run succeeded")
	}
	if runs, _ := env.AnalysisRuns.ListRecent(foreign, r.ID, 5); len(runs) != 0 {
		t.Fatal("foreign list sees runs")
	}
	if ok, _ := env.AnalysisRuns.RenewLease(foreign, run.ID, "w-test", time.Minute); ok {
		t.Fatal("foreign renew succeeded")
	}
	if ok, _ := env.SolutionStore.Choose(foreign, draft.ID, 0, 1); ok {
		t.Fatal("foreign choose succeeded")
	}
	if err := env.SolutionStore.DeleteDraft(foreign, draft.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := env.SolutionStore.Get(k.ctx, draft.ID); err != nil {
		t.Fatal("a foreign DeleteDraft removed my draft")
	}
	if n, _ := env.AnalysisRuns.CountRunning(foreign, r.ProjectID, domain.AnalysisModeComplete); n != 0 {
		t.Fatal("foreign count sees my run")
	}
	// Use cases refuse another tenant's request outright.
	_, err := k.generate.Execute(tenant.WithUserID(foreign, k.userID), usecase.GenerateSolutionInput{RequestID: r.ID})
	requireCode(t, err, "REQUEST_SOLUTION_REQUEST_NOT_FOUND")
	_, err = k.list.Execute(foreign, usecase.ListSolutionsInput{RequestID: r.ID})
	requireCode(t, err, "REQUEST_SOLUTION_REQUEST_NOT_FOUND")
}

// --- flows ---

func solFlowApprove(t *testing.T, env SolutionEnv) {
	k := newSolKit(t, env)
	k.completer.replies = []string{"Đây là kết quả:\n```json\n" + validSolutionDoc(t) + "\n```"}
	r := k.seed(t, domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)

	start := time.Now()
	gen, err := k.generate.Execute(k.ctx, usecase.GenerateSolutionInput{RequestID: r.ID})
	if err != nil || gen.RunID == "" || time.Since(start) > 2*time.Second {
		t.Fatalf("generate: %+v %v (%v)", gen, err, time.Since(start))
	}
	run := k.waitRun(t, gen.RunID)
	if run.Status != domain.RunStatusSucceeded {
		t.Fatalf("run = %+v", run)
	}
	if got := k.requestStatus(t, r.ID); got != domain.RequestStatusAwaitingAnalysisApproval {
		t.Fatalf("request = %s", got)
	}
	sol, _ := env.SolutionStore.Get(k.ctx, gen.SolutionID)
	if sol.Status != domain.SolutionStatusProposed || k.opened.count() != 1 {
		t.Fatalf("solution=%s approvals=%d", sol.Status, k.opened.count())
	}
	subjects := env.OutboxSubjects(t, k.tenantID)
	if countOf(subjects, domain.SubjectSolutionProposed) != 1 {
		t.Fatalf("outbox = %v", subjects)
	}

	// The engine opened a real pending approval bound to the no-choice digest.
	pending := k.pendingApproval(t, domain.SubjectSolution, sol.ID)
	openedDigest := k.opened.list[0].Digest
	if pending.SubjectDigest != openedDigest || pending.Stage != string(domain.RequestStatusAwaitingAnalysisApproval) || pending.RequestID != r.ID {
		t.Fatalf("approval = %+v, opened digest %s", pending, openedDigest)
	}
	// Approving before a choice is refused by the handler even with the digest the approval was opened with.
	_, err = k.approveRPC(pending, openedDigest)
	wantStatus(t, err, codes.FailedPrecondition, "REQUEST_SOLUTION_OPTION_NOT_CHOSEN")

	chosen, err := k.choose.Execute(k.ctx, usecase.ChooseSolutionOptionInput{RequestID: r.ID, SolutionID: sol.ID, OptionID: "opt-2"})
	if err != nil || chosen.ApprovalDigest == openedDigest {
		t.Fatalf("choose: %+v %v", chosen, err)
	}
	if got := k.pendingApproval(t, domain.SubjectSolution, sol.ID); got.SubjectDigest != chosen.ApprovalDigest {
		t.Fatalf("the pending approval kept digest %s, want %s", got.SubjectDigest, chosen.ApprovalDigest)
	}
	// A digest the UI saw before the choice is rejected; so is any content change behind the engine's back.
	_, err = k.approveRPC(pending, openedDigest)
	wantStatus(t, err, codes.FailedPrecondition, "REQUEST_APPROVAL_DIGEST_MISMATCH")
	env.TamperOptions(t, sol.ID, `{"schema_version":1,"tampered":true}`)
	_, err = k.approveRPC(pending, chosen.ApprovalDigest)
	wantStatus(t, err, codes.FailedPrecondition, "REQUEST_APPROVAL_DIGEST_MISMATCH")
	if got := k.requestStatus(t, r.ID); got != domain.RequestStatusAwaitingAnalysisApproval {
		t.Fatalf("a refused approval moved the request to %s", got)
	}
	env.TamperOptions(t, sol.ID, validSolutionDoc(t))
	res, err := k.approveRPC(pending, chosen.ApprovalDigest)
	if err != nil || res.GetRequestStatus() != string(domain.RequestStatusPlanning) || res.GetApproval().GetId() != pending.ID {
		t.Fatalf("approve: %+v %v", res, err)
	}
	if got := k.requestStatus(t, r.ID); got != domain.RequestStatusPlanning {
		t.Fatalf("request = %s, want planning", got)
	}
	final, _ := env.SolutionStore.Get(k.ctx, sol.ID)
	if final.Status != domain.SolutionStatusApproved || final.ChosenOption == nil || *final.ChosenOption != 1 {
		t.Fatalf("final = %+v", final)
	}
	if countOf(env.OutboxSubjects(t, k.tenantID), domain.SubjectSolutionApproved) != 1 {
		t.Fatal("solution.approved missing")
	}
	// Approving twice is the same decision, not a second transition.
	if _, err := k.approveRPC(pending, chosen.ApprovalDigest); err != nil {
		t.Fatalf("repeat approve: %v", err)
	}
	if countOf(env.OutboxSubjects(t, k.tenantID), domain.SubjectSolutionApproved) != 1 {
		t.Fatal("a repeated approve emitted solution.approved again")
	}
}

func countOf(list []string, s string) int {
	n := 0
	for _, x := range list {
		if x == s {
			n++
		}
	}
	return n
}

func solFlowRegenerate(t *testing.T, env SolutionEnv) {
	k := newSolKit(t, env)
	k.completer.replies = []string{validSolutionDoc(t)}
	r := k.seed(t, domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)
	first, err := k.generate.Execute(k.ctx, usecase.GenerateSolutionInput{RequestID: r.ID})
	if err != nil {
		t.Fatal(err)
	}
	k.waitRun(t, first.RunID)

	second, err := k.generate.Execute(k.ctx, usecase.GenerateSolutionInput{RequestID: r.ID, Feedback: "thử phương án không đổi schema"})
	if err != nil || second.RunID == first.RunID {
		t.Fatalf("regenerate: %+v %v", second, err)
	}
	run := k.waitRun(t, second.RunID)
	if run.Status != domain.RunStatusSucceeded || run.Feedback != "thử phương án không đổi schema" {
		t.Fatalf("run = %+v", run)
	}
	old, _ := env.SolutionStore.Get(k.ctx, first.SolutionID)
	cur, _ := env.SolutionStore.Get(k.ctx, second.SolutionID)
	if got, err := env.Approvals.FindPendingBySubject(k.ctx, k.tenantID, domain.SubjectSolution, first.SolutionID); err != nil || got != nil {
		t.Fatalf("the approval of the superseded proposal is still pending: %+v %v", got, err)
	}
	k.pendingApproval(t, domain.SubjectSolution, second.SolutionID)
	if old.Status != domain.SolutionStatusSuperseded || cur.Status != domain.SolutionStatusProposed {
		t.Fatalf("old=%s new=%s", old.Status, cur.Status)
	}
	if got := k.requestStatus(t, r.ID); got != domain.RequestStatusAwaitingAnalysisApproval {
		t.Fatalf("request = %s", got)
	}
	if !strings.Contains(k.completer.prompts[len(k.completer.prompts)-1], "thử phương án không đổi schema") {
		t.Fatal("feedback missing from the second prompt")
	}
	subjects := env.OutboxSubjects(t, k.tenantID)
	if countOf(subjects, domain.SubjectRequestStatusChanged) != 3 {
		t.Fatalf("expected awaiting, analyzing (revision) and awaiting again, outbox = %v", subjects)
	}
}

func solFlowInvalid(t *testing.T, env SolutionEnv) {
	k := newSolKit(t, env)
	k.completer.replies = []string{`{"schema_version":1,"options":[]}`}
	r := k.seed(t, domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)
	gen, err := k.generate.Execute(k.ctx, usecase.GenerateSolutionInput{RequestID: r.ID})
	if err != nil {
		t.Fatal(err)
	}
	run := k.waitRun(t, gen.RunID)
	if run.Status != domain.RunStatusFailed || run.ErrorCode == nil || *run.ErrorCode != domain.RunErrInvalidOutput || run.RawOutput == nil || run.Attempt != 2 {
		t.Fatalf("run = %+v", run)
	}
	if len(solutionsOf(t, k, r.ID)) != 0 {
		t.Fatal("a draft survived the failure")
	}
	if got := k.requestStatus(t, r.ID); got != domain.RequestStatusAnalyzing || k.opened.count() != 0 {
		t.Fatalf("request=%s approvals=%d", got, k.opened.count())
	}
	list, err := k.list.Execute(k.ctx, usecase.ListSolutionsInput{RequestID: r.ID})
	if err != nil || len(list.Solutions) != 0 || len(list.Runs) != 1 || list.Runs[0].Status != domain.RunStatusFailed {
		t.Fatalf("list = %+v %v", list, err)
	}
	// The user can retry.
	k.completer.mu.Lock()
	k.completer.replies = []string{validSolutionDoc(t)}
	k.completer.prompts = nil
	k.completer.mu.Unlock()
	again, err := k.generate.Execute(k.ctx, usecase.GenerateSolutionInput{RequestID: r.ID})
	if err != nil || again.RunID == gen.RunID {
		t.Fatalf("retry: %+v %v", again, err)
	}
	if run := k.waitRun(t, again.RunID); run.Status != domain.RunStatusSucceeded {
		t.Fatalf("retry run = %+v", run)
	}
}

func solFlowQuestion(t *testing.T, env SolutionEnv) {
	k := newSolKit(t, env)
	k.agent.reply = analysisDoc(t, "answer_valid.json")
	r := k.seed(t, domain.RequestTypeQuestion, domain.RequestStatusAnalyzing)
	gen, err := k.generate.Execute(k.ctx, usecase.GenerateSolutionInput{RequestID: r.ID})
	if err != nil {
		t.Fatal(err)
	}
	run := k.waitRun(t, gen.RunID)
	if run.Status != domain.RunStatusSucceeded || run.Mode != domain.AnalysisModeAgentReadonly || run.RepoCheck != domain.RepoCheckClean || run.Enforcement != domain.EnforcementPromptOnly {
		t.Fatalf("run = %+v", run)
	}
	sol, _ := env.SolutionStore.Get(k.ctx, gen.SolutionID)
	if sol.Kind != domain.SolutionKindAnswer || sol.Status != domain.SolutionStatusProposed || sol.ChosenOption != nil {
		t.Fatalf("solution = %+v", sol)
	}
	if k.opened.count() != 1 || k.opened.list[0].SubjectType != domain.SubjectAnswer {
		t.Fatalf("approvals = %+v", k.opened.list)
	}
	pending := k.pendingApproval(t, domain.SubjectAnswer, sol.ID)
	if res, err := k.approveRPC(pending, pending.SubjectDigest); err != nil || res.GetRequestStatus() != string(domain.RequestStatusCompleted) {
		t.Fatalf("approve: %+v %v", res, err)
	}
	if got := k.requestStatus(t, r.ID); got != domain.RequestStatusCompleted {
		t.Fatalf("request = %s, want completed", got)
	}
	if len(solutionsOf(t, k, r.ID)) != 1 {
		t.Fatal("an answer must not create plans or other artifacts")
	}
	if len(k.agent.inputs) != 1 || k.agent.inputs[0].RepoPath != "/srv/repo" {
		t.Fatalf("agent inputs = %+v", k.agent.inputs)
	}
}

func solFlowHotfix(t *testing.T, env SolutionEnv) {
	k := newSolKit(t, env)
	k.agent.reply = analysisDoc(t, "diagnosis_valid.json")
	r := k.seed(t, domain.RequestTypeHotfix, domain.RequestStatusAnalyzing)
	gen, err := k.generate.Execute(k.ctx, usecase.GenerateSolutionInput{RequestID: r.ID})
	if err != nil {
		t.Fatal(err)
	}
	if run := k.waitRun(t, gen.RunID); run.Status != domain.RunStatusSucceeded {
		t.Fatalf("run = %+v", run)
	}
	sol, _ := env.SolutionStore.Get(k.ctx, gen.SolutionID)
	if sol.Status != domain.SolutionStatusApproved || k.opened.count() != 0 {
		t.Fatalf("solution=%s approvals=%d", sol.Status, k.opened.count())
	}
	if list, _, err := env.Approvals.List(k.ctx, k.tenantID, usecase.ApprovalListFilter{RequestID: r.ID}); err != nil || len(list) != 0 {
		t.Fatalf("a hotfix diagnosis has no human gate, got approvals %+v (%v)", list, err)
	}
	if got := k.requestStatus(t, r.ID); got != domain.RequestStatusAwaitingPlanApproval {
		t.Fatalf("request = %s", got)
	}
	subjects := env.OutboxSubjects(t, k.tenantID)
	if countOf(subjects, domain.SubjectSolutionApproved) != 1 || countOf(subjects, domain.SubjectSolutionProposed) != 1 {
		t.Fatalf("outbox = %v", subjects)
	}
}

func solRecovery(t *testing.T, env SolutionEnv) {
	k := newSolKit(t, env)
	r := k.seed(t, domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)
	run, draft := startInput(t, k, r, domain.AnalysisModeComplete, domain.RunKindSolution, "")
	if _, err := startIn(t, k, run, draft, 0); err != nil {
		t.Fatal(err)
	}
	env.ExpireRun(t, run.ID)
	rec := usecase.NewRecoverInterruptedAnalysisRuns(env.AnalysisRuns, env.SolutionStore, env.Tx, usecase.DefaultAnalysisSettings(), "sweeper-1")
	// Other tests' expired runs may share the database; keep sweeping until ours is handled.
	for i := 0; i < 10; i++ {
		if _, err := rec.RunOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		if got, _ := env.AnalysisRuns.Get(k.ctx, run.ID); got.Status != domain.RunStatusRunning {
			break
		}
	}
	got, _ := env.AnalysisRuns.Get(k.ctx, run.ID)
	if got.Status != domain.RunStatusFailed || got.ErrorCode == nil || *got.ErrorCode != domain.RunErrInterrupted {
		t.Fatalf("run = %+v", got)
	}
	if _, err := env.SolutionStore.Get(k.ctx, draft.ID); err == nil {
		t.Fatal("draft not removed")
	}
	if k.requestStatus(t, r.ID) != domain.RequestStatusAnalyzing {
		t.Fatal("recovery moved the request")
	}
}

func solPersistRollback(t *testing.T, env SolutionEnv) {
	k := newSolKit(t, env)
	r := k.seed(t, domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)
	run, draft := startInput(t, k, r, domain.AnalysisModeComplete, domain.RunKindSolution, "")
	if _, err := startIn(t, k, run, draft, 0); err != nil {
		t.Fatal(err)
	}
	before := len(env.OutboxSubjects(t, k.tenantID))
	boom := errors.New("transition exploded")
	writer := usecase.NewAnalysisResultWriter(env.Requests, env.SolutionStore, env.AnalysisRuns, k.opened, failingTransitioner{err: boom}, env.Tx, env.Outbox, "w-test")
	err := writer.Persist(k.ctx, usecase.PersistAnalysisInput{Run: run, Doc: []byte(validSolutionDoc(t)), Raw: "raw"})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	got, _ := env.SolutionStore.Get(k.ctx, draft.ID)
	stored, _ := env.AnalysisRuns.Get(k.ctx, run.ID)
	if got.Status != domain.SolutionStatusDraft || stored.Status != domain.RunStatusRunning {
		t.Fatalf("not rolled back: solution=%s run=%s", got.Status, stored.Status)
	}
	k.opened.list = nil // the recording opener is not transactional; a real one rolls back with the database
	if after := len(env.OutboxSubjects(t, k.tenantID)); after != before {
		t.Fatalf("outbox grew from %d to %d", before, after)
	}
	if k.requestStatus(t, r.ID) != domain.RequestStatusAnalyzing {
		t.Fatal("request moved")
	}

	// An approval that cannot open rolls back too.
	k.opened.fail = boom
	good := usecase.NewAnalysisResultWriter(env.Requests, env.SolutionStore, env.AnalysisRuns, k.opened, realTransitioner(env.IntakeEnv), env.Tx, env.Outbox, "w-test")
	if err := good.Persist(k.ctx, usecase.PersistAnalysisInput{Run: run, Doc: []byte(validSolutionDoc(t))}); !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	got, _ = env.SolutionStore.Get(k.ctx, draft.ID)
	if got.Status != domain.SolutionStatusDraft || len(env.OutboxSubjects(t, k.tenantID)) != before {
		t.Fatal("an approval failure left partial state")
	}
}

// --- gRPC ---

func solRPCFlow(t *testing.T, env SolutionEnv) {
	k := newSolKit(t, env)
	k.completer.replies = []string{validSolutionDoc(t)}
	client, as := k.client, k.rpc()

	r := k.seed(t, domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)

	_, err := client.GenerateSolution(rpcAs(k.tenantID, ""), &requestv1.GenerateSolutionRequest{RequestId: r.ID})
	wantStatus(t, err, codes.InvalidArgument, "") // no acting user
	_, err = client.GenerateSolution(as, &requestv1.GenerateSolutionRequest{RequestId: r.ID, EngineOverride: "openspec"})
	wantStatus(t, err, codes.InvalidArgument, "native")

	gen, err := client.GenerateSolution(as, &requestv1.GenerateSolutionRequest{RequestId: r.ID})
	if err != nil || gen.GetRunId() == "" || gen.GetSolutionId() == "" {
		t.Fatalf("generate: %+v %v", gen, err)
	}
	k.waitRun(t, gen.GetRunId())

	list, err := client.ListSolutions(as, &requestv1.ListSolutionsRequest{RequestId: r.ID})
	if err != nil || len(list.GetSolutions()) != 1 || len(list.GetRuns()) != 1 {
		t.Fatalf("list: %+v %v", list, err)
	}
	sol := list.GetSolutions()[0]
	if sol.GetStatus() != requestv1.SolutionStatus_SOLUTION_STATUS_PROPOSED || sol.GetKind() != requestv1.SolutionKind_SOLUTION_KIND_SOLUTION || sol.GetChosenOption() != -1 ||
		!strings.Contains(sol.GetOptionsJson(), "Thêm lớp cache") || sol.GetGenerationRunId() != gen.GetRunId() {
		t.Fatalf("solution = %+v", sol)
	}
	if run := list.GetRuns()[0]; run.GetStatus() != "succeeded" || run.GetId() != gen.GetRunId() {
		t.Fatalf("run = %+v", run)
	}

	// ApprovalService lists the pending approval the worker opened.
	approvals, err := k.approvals.ListApprovals(as, &requestv1.ListApprovalsRequest{RequestId: r.ID})
	if err != nil || len(approvals.GetApprovals()) != 1 || approvals.GetApprovals()[0].GetStatus() != requestv1.ApprovalStatus_APPROVAL_STATUS_PENDING {
		t.Fatalf("approvals: %+v %v", approvals, err)
	}
	approval := approvals.GetApprovals()[0]

	_, err = client.ChooseSolutionOption(as, &requestv1.ChooseSolutionOptionRequest{RequestId: r.ID, SolutionId: sol.GetId(), OptionId: "opt-9"})
	wantStatus(t, err, codes.InvalidArgument, "REQUEST_SOLUTION_OPTION_NOT_FOUND")
	chosen, err := client.ChooseSolutionOption(as, &requestv1.ChooseSolutionOptionRequest{RequestId: r.ID, SolutionId: sol.GetId(), OptionId: "opt-1"})
	if err != nil || chosen.GetApprovalDigest() == "" || chosen.GetSolution().GetChosenOption() != 0 {
		t.Fatalf("choose: %+v %v", chosen, err)
	}
	filtered, err := client.ListSolutions(as, &requestv1.ListSolutionsRequest{RequestId: r.ID, Kind: requestv1.SolutionKind_SOLUTION_KIND_ANSWER})
	if err != nil || len(filtered.GetSolutions()) != 0 {
		t.Fatalf("kind filter: %+v %v", filtered, err)
	}
	_, err = client.ListSolutions(rpcAs(newTenant(), k.userID), &requestv1.ListSolutionsRequest{RequestId: r.ID})
	wantStatus(t, err, codes.NotFound, "")

	// The digest the UI got from ChooseSolutionOption is the one Approve needs; an older one is refused.
	_, err = k.approvals.Approve(as, &requestv1.ApproveRequest{Id: approval.GetId(), ExpectedDigest: approval.GetSubjectDigest()})
	wantStatus(t, err, codes.FailedPrecondition, "REQUEST_APPROVAL_DIGEST_MISMATCH")
	res, err := k.approvals.Approve(as, &requestv1.ApproveRequest{Id: approval.GetId(), Comment: "ok", ExpectedDigest: chosen.GetApprovalDigest()})
	if err != nil || res.GetRequestStatus() != string(domain.RequestStatusPlanning) {
		t.Fatalf("approve: %+v %v", res, err)
	}
	if got := k.requestStatus(t, r.ID); got != domain.RequestStatusPlanning {
		t.Fatalf("request = %s", got)
	}
	after, err := client.ListSolutions(as, &requestv1.ListSolutionsRequest{RequestId: r.ID})
	if err != nil || len(after.GetSolutions()) != 1 || after.GetSolutions()[0].GetStatus() != requestv1.SolutionStatus_SOLUTION_STATUS_APPROVED || after.GetSolutions()[0].GetChosenOption() != 0 {
		t.Fatalf("after approve: %+v %v", after, err)
	}
}

// solRPCReject: Reject through ApprovalService sends the request back to the backlog from the analysis stage.
func solRPCReject(t *testing.T, env SolutionEnv) {
	k := newSolKit(t, env)
	k.completer.replies = []string{validSolutionDoc(t)}
	r := k.seed(t, domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)
	gen, err := k.client.GenerateSolution(k.rpc(), &requestv1.GenerateSolutionRequest{RequestId: r.ID})
	if err != nil {
		t.Fatal(err)
	}
	k.waitRun(t, gen.GetRunId())
	if _, err := k.client.ChooseSolutionOption(k.rpc(), &requestv1.ChooseSolutionOptionRequest{RequestId: r.ID, SolutionId: gen.GetSolutionId(), OptionId: "opt-2"}); err != nil {
		t.Fatal(err)
	}
	pending := k.pendingApproval(t, domain.SubjectSolution, gen.GetSolutionId())

	_, err = k.approvals.Reject(k.rpc(), &requestv1.RejectRequest{Id: pending.ID})
	if err == nil {
		t.Fatal("a rejection needs a comment")
	}
	res, err := k.approvals.Reject(k.rpc(), &requestv1.RejectRequest{Id: pending.ID, Comment: "chưa đủ rõ về rủi ro"})
	if err != nil || res.GetRequestStatus() != string(domain.RequestStatusRequestBacklog) {
		t.Fatalf("reject: %+v %v", res, err)
	}
	req, err := env.Requests.Get(k.ctx, r.ID)
	if err != nil || req.Status != domain.RequestStatusRequestBacklog || req.ReturnedFromStage != domain.ReturnStageAnalysis || req.ReturnedCategory != domain.ReturnCategoryRejected {
		t.Fatalf("request = %+v %v", req, err)
	}
	sol, _ := env.SolutionStore.Get(k.ctx, gen.GetSolutionId())
	if sol.Status != domain.SolutionStatusRejected || sol.ChosenOption != nil {
		t.Fatalf("solution = %+v (a rejected choice must not survive)", sol)
	}
	if got, _ := env.Approvals.FindPendingBySubject(k.ctx, k.tenantID, domain.SubjectSolution, sol.ID); got != nil {
		t.Fatal("the approval is still pending after the rejection")
	}
}

// solApproverCanChoose: a user named by the approval policy may choose an option; an outsider may not.
func solApproverCanChoose(t *testing.T, env SolutionEnv) {
	k := newSolKit(t, env)
	k.completer.replies = []string{validSolutionDoc(t)}
	approver, stranger := uuid.NewString(), uuid.NewString()
	pol := policyOf(k.tenantID, func(p *domain.ApprovalPolicy) {
		p.SubjectType = domain.SubjectSolution
		p.Approvers = []domain.Principal{{Kind: domain.PrincipalKindUser, ID: approver}}
	})
	if _, err := env.Policies.Upsert(k.ctx, pol, 0); err != nil {
		t.Fatal(err)
	}
	r := k.seed(t, domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)
	gen, err := k.client.GenerateSolution(k.rpc(), &requestv1.GenerateSolutionRequest{RequestId: r.ID})
	if err != nil {
		t.Fatal(err)
	}
	k.waitRun(t, gen.GetRunId())
	in := &requestv1.ChooseSolutionOptionRequest{RequestId: r.ID, SolutionId: gen.GetSolutionId(), OptionId: "opt-2"}
	_, err = k.client.ChooseSolutionOption(rpcAs(k.tenantID, stranger), in)
	wantStatus(t, err, codes.PermissionDenied, "REQUEST_SOLUTION_FORBIDDEN")
	res, err := k.client.ChooseSolutionOption(rpcAs(k.tenantID, approver), in)
	if err != nil || res.GetSolution().GetChosenOption() != 1 {
		t.Fatalf("approver choose: %+v %v", res, err)
	}
	// Generating is not opened to approvers: only the reporter and admins may start or regenerate.
	_, err = k.client.GenerateSolution(rpcAs(k.tenantID, approver), &requestv1.GenerateSolutionRequest{RequestId: r.ID, Feedback: "again"})
	wantStatus(t, err, codes.PermissionDenied, "REQUEST_SOLUTION_FORBIDDEN")
}

// solSubjectHandlerContract runs the approval engine's shared handler contract on the three handlers with real stores.
func solSubjectHandlerContract(t *testing.T, env SolutionEnv) {
	cases := []struct {
		st   domain.SubjectType
		typ  domain.RequestType
		kind domain.SolutionKind
		doc  func(t *testing.T) string
	}{
		{domain.SubjectSolution, domain.RequestTypeChangeRequest, domain.SolutionKindSolution, validSolutionDoc},
		{domain.SubjectSolution, domain.RequestTypeBug, domain.SolutionKindDiagnosis, func(t *testing.T) string { return analysisDoc(t, "diagnosis_valid.json") }},
		{domain.SubjectFindings, domain.RequestTypeSpike, domain.SolutionKindFindings, func(t *testing.T) string { return analysisDoc(t, "findings_valid.json") }},
		{domain.SubjectAnswer, domain.RequestTypeQuestion, domain.SolutionKindAnswer, func(t *testing.T) string { return analysisDoc(t, "answer_valid.json") }},
	}
	for _, tc := range cases {
		t.Run(string(tc.st)+"/"+string(tc.kind), func(t *testing.T) {
			RunSubjectHandlerContract(t, func(t *testing.T) SubjectHandlerFixture {
				k := newSolKit(t, env)
				r := k.seed(t, tc.typ, domain.RequestStatusAwaitingAnalysisApproval)
				now := time.Now().UTC().Truncate(time.Microsecond)
				s := domain.Solution{ID: uuid.NewString(), TenantID: k.tenantID, RequestID: r.ID, Kind: tc.kind, Status: domain.SolutionStatusProposed,
					OptionsJSON: []byte(tc.doc(t)), Version: 1, CreatedAt: now, UpdatedAt: now}
				if err := env.SolutionStore.Insert(k.ctx, s); err != nil {
					t.Fatal(err)
				}
				return SubjectHandlerFixture{
					Handler: k.handlers[tc.st], Subject: tc.st, Request: r, Ctx: k.ctx,
					Approval: func(subjectID, digest string) domain.Approval {
						by := k.userID
						return domain.Approval{ID: uuid.NewString(), TenantID: k.tenantID, RequestID: r.ID, SubjectType: tc.st, SubjectID: subjectID,
							Stage: string(r.Status), SubjectDigest: digest, DecidedBy: &by, Comment: "c"}
					},
				}
			})
		})
	}
}

// SeedRequestForRLS creates an analyzing request and returns its id, for adapter tests that probe row-level security.
func SeedRequestForRLS(t *testing.T, env IntakeEnv, ctx context.Context) string {
	t.Helper()
	return seedRequest(t, env, ctx, domain.RequestStatusAnalyzing, nil).ID
}
