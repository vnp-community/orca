package grpc

import (
	"context"
	"log/slog"
	"net"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/internalcaller"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const (
	gatewayToken = "gateway-secret"
	serviceToken = "service-secret"
)

type chainEnv struct {
	conn *grpc.ClientConn
}

// startChain runs the real interceptor chain on bufconn in front of the real Server (unwired RPCs answer
// Unimplemented, which proves a call got past every guard).
func startChain(t *testing.T, cfg SecurityChainConfig) *chainEnv {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	log := slog.New(slog.NewTextHandler(discard{}, nil))
	opts := append([]grpc.ServerOption{grpcmw.ChainUnary(log)}, SecurityChain(cfg)...)
	srv := grpc.NewServer(opts...)
	repo := &stubRequestRepository{byID: map[string]domain.Request{}}
	requestv1.RegisterRequestServiceServer(srv, newTestServer(repo))
	requestv1.RegisterApprovalServiceServer(srv, &requestv1.UnimplementedApprovalServiceServer{})
	requestv1.RegisterAiBudgetAdminServiceServer(srv, &requestv1.UnimplementedAiBudgetAdminServiceServer{})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return &chainEnv{conn: conn}
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }

func newChainConfig(t *testing.T) SecurityChainConfig {
	t.Helper()
	return SecurityChainConfig{
		GatewayToken: gatewayToken,
		ServiceToken: serviceToken,
		Authorize:    newAuthorizer(realPolicy(t), newAccessRoles(), &recordingAuditor{}),
		Limiter:      usecase.NewRateLimiter(domain.DefaultLimits, nil),
	}
}

func withMD(token, user, role, actor string) context.Context {
	md := metadata.Pairs(grpcmw.MetadataTenantID, testTenant, grpcmw.MetadataUserID, user)
	if token != "" {
		md.Set(internalcaller.MetadataKey, token)
	}
	if role != "" {
		md.Set(grpcmw.MetadataRole, role)
	}
	if actor != "" {
		md.Set(grpcmw.MetadataActorType, actor)
	}
	return metadata.NewOutgoingContext(context.Background(), md)
}

func TestChain_PublicRPCNeedsGatewayToken(t *testing.T) {
	env := startChain(t, newChainConfig(t))
	c := requestv1.NewRequestServiceClient(env.conn)

	_, err := c.GetRequestFlow(withMD("", userOwner, "", ""), &requestv1.GetRequestFlowRequest{})
	if status.Code(err) != codes.PermissionDenied || !strings.Contains(err.Error(), "INTERNAL_CALLER_REQUIRED") {
		t.Fatalf("no token: %v", err)
	}
	_, err = c.GetRequestFlow(withMD("wrong", userOwner, "", ""), &requestv1.GetRequestFlowRequest{})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("wrong token: %v", err)
	}
	// Past the guards the unwired handler answers Unimplemented.
	_, err = c.GetRequestFlow(withMD(gatewayToken, userOwner, "", ""), &requestv1.GetRequestFlowRequest{})
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("valid token should reach the handler, got %v", err)
	}
}

func TestChain_InternalRPCNeedsServiceTokenNotGatewayToken(t *testing.T) {
	env := startChain(t, newChainConfig(t))
	c := requestv1.NewRequestServiceClient(env.conn)
	req := &requestv1.ReportTaskOutcomeRequest{RequestId: testRequest}

	if _, err := c.ReportTaskOutcome(withMD(gatewayToken, userOwner, "admin", ""), req); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("the gateway token must not open an internal RPC: %v", err)
	}
	if _, err := c.ReportTaskOutcome(withMD("", userOwner, "admin", ""), req); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("no token: %v", err)
	}
	if _, err := c.ReportTaskOutcome(withMD(serviceToken, userOwner, "", ""), req); status.Code(err) != codes.Unimplemented {
		t.Fatalf("service token reaches the handler, got %v", err)
	}
	// And the service token does not open public RPCs.
	if _, err := c.GetRequestFlow(withMD(serviceToken, userOwner, "", ""), &requestv1.GetRequestFlowRequest{}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("service token on a public RPC: %v", err)
	}
}

func TestChain_EmptyConfiguredTokensDenyEverything(t *testing.T) {
	cfg := newChainConfig(t)
	cfg.GatewayToken, cfg.ServiceToken = "", ""
	env := startChain(t, cfg)
	c := requestv1.NewRequestServiceClient(env.conn)
	for _, tok := range []string{"", "anything", gatewayToken} {
		if _, err := c.GetRequestFlow(withMD(tok, userAdmin, "admin", ""), &requestv1.GetRequestFlowRequest{}); status.Code(err) != codes.PermissionDenied {
			t.Errorf("token %q on public RPC: %v", tok, err)
		}
		if _, err := c.ReportTaskOutcome(withMD(tok, userAdmin, "admin", ""), &requestv1.ReportTaskOutcomeRequest{}); status.Code(err) != codes.PermissionDenied {
			t.Errorf("token %q on internal RPC: %v", tok, err)
		}
	}
	stream, err := c.ExportTenantRequests(withMD("", userAdmin, "admin", ""), &requestv1.ExportTenantRequestsRequest{})
	if err == nil {
		_, err = stream.Recv()
	}
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("stream with no token: %v", err)
	}
}

func TestChain_AgentCannotApproveStartPhaseOrChangeSettings(t *testing.T) {
	env := startChain(t, newChainConfig(t))
	rs := requestv1.NewRequestServiceClient(env.conn)
	as := requestv1.NewApprovalServiceClient(env.conn)
	ai := requestv1.NewAiBudgetAdminServiceClient(env.conn)
	ctx := func() context.Context { return withMD(gatewayToken, userOwner, "admin", "agent") }

	_, err := as.Approve(ctx(), &requestv1.ApproveRequest{Id: testApproval})
	expectForbidden(t, "Approve", err)
	_, err = rs.StartPhase(ctx(), &requestv1.StartPhaseRequest{RequestId: testRequest})
	expectForbidden(t, "StartPhase", err)
	_, err = rs.SetRequestFlowSettings(ctx(), &requestv1.SetRequestFlowSettingsRequest{})
	expectForbidden(t, "SetRequestFlowSettings", err)
	_, err = ai.UpsertAiBudget(ctx(), &requestv1.UpsertAiBudgetRequest{})
	expectForbidden(t, "UpsertAiBudget", err)

	// The same admin as a person is allowed through.
	if _, err := rs.SetRequestFlowSettings(withMD(gatewayToken, userAdmin, "admin", "user"), &requestv1.SetRequestFlowSettingsRequest{}); status.Code(err) != codes.Unimplemented {
		t.Fatalf("admin user should reach the handler, got %v", err)
	}
}

func expectForbidden(t *testing.T, what string, err error) {
	t.Helper()
	if status.Code(err) != codes.PermissionDenied || !strings.Contains(err.Error(), "REQUEST_FORBIDDEN") {
		t.Errorf("%s as agent: want REQUEST_FORBIDDEN, got %v", what, err)
	}
}

func TestChain_StreamAdminOnly(t *testing.T) {
	env := startChain(t, newChainConfig(t))
	c := requestv1.NewRequestServiceClient(env.conn)
	recv := func(ctx context.Context) error {
		s, err := c.ExportTenantRequests(ctx, &requestv1.ExportTenantRequestsRequest{})
		if err != nil {
			return err
		}
		_, err = s.Recv()
		return err
	}
	if err := recv(withMD(gatewayToken, userOwner, "", "user")); status.Code(err) != codes.PermissionDenied || !strings.Contains(err.Error(), "REQUEST_FORBIDDEN") {
		t.Errorf("project owner is not tenant admin: %v", err)
	}
	if err := recv(withMD(gatewayToken, userAdmin, "admin", "agent")); status.Code(err) != codes.PermissionDenied {
		t.Errorf("agent behind admin: %v", err)
	}
	if err := recv(withMD(gatewayToken, userAdmin, "admin", "user")); status.Code(err) != codes.Unimplemented {
		t.Errorf("admin reaches the handler (unwired -> Unimplemented): %v", err)
	}
}

func TestChain_AIRateLimitReturnsRetryInfo(t *testing.T) {
	cfg := newChainConfig(t)
	cfg.Limiter = usecase.NewRateLimiter(map[string]domain.Limit{domain.RateAI: {PerSecond: 0.1, Burst: 1}}, nil)
	env := startChain(t, cfg)
	c := requestv1.NewRequestServiceClient(env.conn)
	call := func() error {
		_, err := c.GenerateSolution(withMD(gatewayToken, userOwner, "", "user"), &requestv1.GenerateSolutionRequest{RequestId: testRequest})
		return err
	}
	if err := call(); status.Code(err) != codes.Unimplemented {
		t.Fatalf("first call passes the limiter: %v", err)
	}
	err := call()
	if status.Code(err) != codes.ResourceExhausted || !strings.Contains(err.Error(), "REQUEST_RATE_LIMITED") {
		t.Fatalf("second call: %v", err)
	}
	var retry *errdetails.RetryInfo
	for _, d := range status.Convert(err).Details() {
		if r, ok := d.(*errdetails.RetryInfo); ok {
			retry = r
		}
	}
	if retry == nil || retry.GetRetryDelay().AsDuration() <= 0 {
		t.Fatalf("RetryInfo with a positive delay expected, got %v", retry)
	}
	// The read class is separate: a GetRequest is not limited by the exhausted ai bucket.
	if _, err := c.GetRequest(withMD(gatewayToken, userOwner, "", "user"), &requestv1.GetRequestRequest{Id: testRequest}); status.Code(err) == codes.ResourceExhausted {
		t.Fatalf("read class must not share the ai bucket: %v", err)
	}
}

func TestChain_OpenRequestCapRefusesCreate(t *testing.T) {
	cfg := newChainConfig(t)
	cfg.Gate = usecase.NewConcurrencyGate(domain.ConcurrencyCaps{OpenRequestsPerTenant: 2000}, fixedCounter{open: 2000})
	env := startChain(t, cfg)
	c := requestv1.NewRequestServiceClient(env.conn)
	_, err := c.CreateRequest(withMD(gatewayToken, userOwner, "", "user"), &requestv1.CreateRequestRequest{ProjectId: testProject, Title: "t"})
	if status.Code(err) != codes.ResourceExhausted || !strings.Contains(err.Error(), "concurrency") {
		t.Fatalf("want concurrency refusal, got %v", err)
	}
}

type fixedCounter struct{ open, running int }

func (f fixedCounter) CountOpenRequests(context.Context) (int, error)    { return f.open, nil }
func (f fixedCounter) CountRunning(context.Context, string) (int, error) { return f.running, nil }

func TestChain_HookPointsRunInTheDocumentedOrder(t *testing.T) {
	var order []string
	mark := func(name string) grpc.UnaryServerInterceptor {
		return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
			order = append(order, name)
			return h(ctx, req)
		}
	}
	cfg := newChainConfig(t)
	cfg.AfterGuard = []grpc.UnaryServerInterceptor{mark("flow-gate")}
	cfg.AfterAuthz = []grpc.UnaryServerInterceptor{mark("audit")}
	env := startChain(t, cfg)
	c := requestv1.NewRequestServiceClient(env.conn)
	// Refused by authorization: the flow gate ran, the audit hook did not.
	_, _ = c.StartPhase(withMD(gatewayToken, userMember, "", "user"), &requestv1.StartPhaseRequest{RequestId: testRequest})
	if strings.Join(order, ",") != "flow-gate" {
		t.Fatalf("order after a refusal: %v", order)
	}
	order = nil
	_, _ = c.GetRequestFlow(withMD(gatewayToken, userOwner, "", "user"), &requestv1.GetRequestFlowRequest{})
	if strings.Join(order, ",") != "flow-gate,audit" {
		t.Fatalf("order for an allowed call: %v", order)
	}
	// A call without the token never reaches the hooks.
	order = nil
	_, _ = c.GetRequestFlow(withMD("", userOwner, "", "user"), &requestv1.GetRequestFlowRequest{})
	if len(order) != 0 {
		t.Fatalf("hooks ran for an unauthenticated call: %v", order)
	}
}

// Real FlowGate and AuditRPC in the documented slots: the gate answers before authorization (a gated call is
// not an authz decision), and the RPC audit only sees calls that passed authorization.
func TestChain_RealFlowGateRunsBeforeAuthzAndAuditRPCAfterIt(t *testing.T) {
	on := false
	rec := &captureRPCAudit{}
	cfg := newChainConfig(t)
	cfg.AfterGuard = []grpc.UnaryServerInterceptor{FlowGate(func(context.Context) (bool, error) { return on, nil })}
	cfg.AfterAuthz = []grpc.UnaryServerInterceptor{AuditRPC(rec)}
	c := requestv1.NewRequestServiceClient(startChain(t, cfg).conn)
	start := func() error {
		_, err := c.StartPhase(withMD(gatewayToken, userMember, "", "user"), &requestv1.StartPhaseRequest{RequestId: testRequest})
		return err
	}
	if err := start(); status.Code(err) == codes.PermissionDenied || status.Code(err) == codes.Unimplemented {
		t.Fatalf("flow off must answer from the gate before authz, got %v", err)
	}
	on = true
	if err := start(); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("flow on: authz refuses the member, got %v", err)
	}
	if len(rec.events) != 0 {
		t.Fatalf("AuditRPC must not see a call authz refused: %v", rec.events)
	}
}

type captureRPCAudit struct{ events []usecase.RPCAuditEvent }

func (c *captureRPCAudit) Record(_ context.Context, e usecase.RPCAuditEvent) {
	c.events = append(c.events, e)
}
