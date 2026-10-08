//go:build integration

package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/internalcaller"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/common/testutil"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
	postgresadapter "github.com/stablyai/orca-go/services/request-service/internal/adapter/postgres"
	"github.com/stablyai/orca-go/services/request-service/internal/config"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"strings"
)

func startMigratedPostgresDSN(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "postgres:16-alpine",
			ExposedPorts: []string{"5432/tcp"},
			Env:          map[string]string{"POSTGRES_USER": "orca", "POSTGRES_PASSWORD": "orca", "POSTGRES_DB": "request"},
			WaitingFor:   wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
		},
		Started: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Terminate(context.Background()) })
	host, _ := c.Host(ctx)
	port, _ := c.MappedPort(ctx, "5432")
	dsn := fmt.Sprintf("postgres://orca:orca@%s:%s/request?sslmode=disable", host, port.Port())
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(ctx) }()
	for _, s := range contracttest.MigrationScripts(t, "postgres", "up") {
		if _, err := conn.Exec(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	return dsn
}

// With approval enabled the real ApprovalService and ApprovalPolicyAdminService answer through run()'s wiring.
func TestRun_ServesApprovalServicesWhenEnabled(t *testing.T) {
	cfg := testConfig(t, startMigratedPostgresDSN(t))
	cfg.ApprovalEnabled = true
	cfg.RequestFlowEnabled = true // RequestApproval advances the flow, so the gate must be open for this tenant
	cfg.ApprovalSweepInterval = 200 * time.Millisecond
	assertServiceStartsAndServes(t, cfg, func() {
		conn, err := grpc.NewClient(fmt.Sprintf("127.0.0.1:%d", cfg.GRPCPort), grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithUnaryInterceptor(internalcaller.ClientInterceptor(testGatewayToken)))
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		// RequestApproval is an internal RPC: only a sibling service (service token) may call it.
		internalConn, err := grpc.NewClient(fmt.Sprintf("127.0.0.1:%d", cfg.GRPCPort), grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithUnaryInterceptor(internalcaller.ClientInterceptor(testServiceToken)))
		if err != nil {
			t.Fatal(err)
		}
		defer internalConn.Close()
		tenantID, admin, member := uuid.NewString(), uuid.NewString(), uuid.NewString()
		as := func(user, role string) context.Context {
			return metadata.AppendToOutgoingContext(context.Background(), grpcmw.MetadataTenantID, tenantID, grpcmw.MetadataUserID, user, grpcmw.MetadataRole, role)
		}
		flow := requestv1.NewRequestServiceClient(conn)
		if _, err := flow.SetRequestFlowSettings(as(member, "user"), &requestv1.SetRequestFlowSettingsRequest{Enabled: true}); status.Code(err) != codes.PermissionDenied {
			t.Fatalf("non-admin SetRequestFlowSettings = %v", err)
		}
		if _, err := flow.SetRequestFlowSettings(as(admin, "admin"), &requestv1.SetRequestFlowSettingsRequest{Enabled: true}); err != nil {
			t.Fatalf("admin SetRequestFlowSettings = %v", err)
		}
		policies := requestv1.NewApprovalPolicyAdminServiceClient(conn)
		pol := &requestv1.ApprovalPolicy{SubjectType: requestv1.ApprovalSubjectType_APPROVAL_SUBJECT_TYPE_PLAN, Enabled: true, Approvers: []string{"reporter", "role:admin"}}
		if _, err := policies.UpsertApprovalPolicy(as(member, "user"), &requestv1.UpsertApprovalPolicyRequest{Policy: pol}); status.Code(err) != codes.PermissionDenied {
			t.Fatalf("non-admin upsert = %v", err)
		}
		saved, err := policies.UpsertApprovalPolicy(as(admin, "admin"), &requestv1.UpsertApprovalPolicyRequest{Policy: pol})
		if err != nil || saved.GetPolicy().GetVersion() != 1 || saved.GetPolicy().GetCreatedBy() != admin || saved.GetPolicy().GetTenantId() != tenantID {
			t.Fatalf("admin upsert = %v %v", saved, err)
		}
		list, err := policies.ListApprovalPolicies(as(admin, "admin"), &requestv1.ListApprovalPoliciesRequest{})
		if err != nil || len(list.GetPolicies()) != 1 {
			t.Fatalf("list = %v %v", list, err)
		}
		other := metadata.AppendToOutgoingContext(context.Background(), grpcmw.MetadataTenantID, uuid.NewString(), grpcmw.MetadataUserID, admin, grpcmw.MetadataRole, "admin")
		if l, err := policies.ListApprovalPolicies(other, &requestv1.ListApprovalPoliciesRequest{}); err != nil || len(l.GetPolicies()) != 0 {
			t.Fatalf("another tenant's admin must not see the policy: %v %v", l, err)
		}
		svc := requestv1.NewApprovalServiceClient(conn)
		inbox, err := svc.ListPendingForUser(as(member, "user"), &requestv1.ListPendingForUserRequest{})
		if err != nil || len(inbox.GetApprovals()) != 0 {
			t.Fatalf("empty inbox = %v %v", inbox, err)
		}
		internalSvc := requestv1.NewApprovalServiceClient(internalConn)
		if _, err := svc.RequestApproval(as(member, "user"), &requestv1.RequestApprovalRequest{RequestId: uuid.NewString(), SubjectType: requestv1.ApprovalSubjectType_APPROVAL_SUBJECT_TYPE_PLAN}); status.Code(err) != codes.PermissionDenied {
			t.Fatalf("the gateway token must not open RequestApproval, got %v", err)
		}
		if _, err := internalSvc.RequestApproval(as(member, "user"), &requestv1.RequestApprovalRequest{RequestId: uuid.NewString(), SubjectType: requestv1.ApprovalSubjectType_APPROVAL_SUBJECT_TYPE_PLAN}); status.Code(err) != codes.FailedPrecondition {
			t.Fatalf("RequestApproval for PLAN must be refused, got %v", err)
		}
		if _, err := internalSvc.RequestApproval(as(member, "user"), &requestv1.RequestApprovalRequest{RequestId: uuid.NewString(), SubjectType: requestv1.ApprovalSubjectType_APPROVAL_SUBJECT_TYPE_PRE_DEPLOY}); status.Code(err) != codes.NotFound {
			t.Fatalf("RequestApproval on an unknown request = %v", err)
		}
		if _, err := svc.Approve(as(member, "user"), &requestv1.ApproveRequest{Id: uuid.NewString(), ExpectedDigest: "x"}); status.Code(err) != codes.NotFound {
			t.Fatalf("Approve of an unknown approval = %v", err)
		}
		time.Sleep(700 * time.Millisecond) // a few sweeper ticks must run cleanly on an empty table
	})
}

// The service must come up with default settings: approval closed, real GetRequest.
func TestRun_StartsWithDefaultConfig(t *testing.T) {
	cfg := testConfig(t, startMigratedPostgresDSN(t))
	assertServiceStartsAndServes(t, cfg, nil)
}

func TestRun_StartsWithMySQL(t *testing.T) {
	assertServiceStartsAndServes(t, testConfig(t, startMigratedMySQLDSN(t)), nil)
}

// startMigratedMySQLDSN returns the service-form DSN (mysql://user:pw@host:port/db) of a fully migrated database.
func startMigratedMySQLDSN(t *testing.T) string {
	t.Helper()
	dsn := testutil.StartMySQL(t, "request")
	db, err := sql.Open("mysql", strings.TrimPrefix(dsn, "mysql://")+"?multiStatements=true")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for i, s := range contracttest.MigrationScripts(t, "mysql", "up") {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("mysql migration %d: %v", i, err)
		}
	}
	// testutil returns the driver form root:pw@tcp(host:port)/db; the service takes mysql://user:pw@host:port/db.
	return strings.NewReplacer("@tcp(", "@", ")/", "/").Replace(dsn)
}

// An outbox row written through the repository must reach the REQUEST stream via the relay run() starts.
func TestRun_RelaysOutboxRowToNATS(t *testing.T) {
	dsn := startMigratedPostgresDSN(t)
	natsURL := testutil.StartNATS(t)
	cfg := testConfig(t, dsn)
	cfg.NATSURL = natsURL

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	repo := postgresadapter.New(pool)
	tenantID := uuid.NewString()
	ctx := tenant.WithTenantID(context.Background(), tenantID)
	ev := domain.OutboxEvent{ID: uuid.NewString(), TenantID: tenantID, Subject: "orca.request.request.created", OccurredAt: time.Now().UTC(), Version: 1, Payload: []byte(`{"request_id":"r1"}`)}
	if err := repo.InTx(ctx, func(txCtx context.Context) error { return repo.InsertOutboxEvent(txCtx, ev) }); err != nil {
		t.Fatal(err)
	}

	assertServiceStartsAndServes(t, cfg, func() {
		nc, err := nats.Connect(natsURL)
		if err != nil {
			t.Fatal(err)
		}
		defer nc.Close()
		js, err := jetstream.New(nc)
		if err != nil {
			t.Fatal(err)
		}
		cctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cons, err := js.OrderedConsumer(cctx, "REQUEST", jetstream.OrderedConsumerConfig{})
		if err != nil {
			t.Fatalf("REQUEST stream missing: %v", err)
		}
		msg, err := cons.Next(jetstream.FetchMaxWait(25 * time.Second))
		if err != nil {
			t.Fatalf("no message relayed: %v", err)
		}
		var got struct {
			ID       string `json:"id"`
			TenantID string `json:"tenant_id"`
		}
		if err := json.Unmarshal(msg.Data(), &got); err != nil {
			t.Fatal(err)
		}
		if msg.Subject() != ev.Subject || got.ID != ev.ID || got.TenantID != tenantID {
			t.Fatalf("relayed %s %+v, want %s id=%s tenant=%s", msg.Subject(), got, ev.Subject, ev.ID, tenantID)
		}
	})
}

// assertServiceStartsAndServes boots run(), checks health and the real RPCs, runs extra, then shuts down.
func assertServiceStartsAndServes(t *testing.T, cfg config.Config, extra func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, cfg) }()

	readyURL := fmt.Sprintf("http://127.0.0.1:%d/readyz", cfg.HTTPPort)
	deadline := time.Now().Add(40 * time.Second)
	for {
		resp, err := http.Get(readyURL)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		select {
		case err := <-done:
			t.Fatalf("service exited early: %v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("service never became ready")
		}
		time.Sleep(200 * time.Millisecond)
	}
	if resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/healthz", cfg.HTTPPort)); err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("/healthz: %v %v", resp, err)
	}

	conn, err := grpc.NewClient(fmt.Sprintf("127.0.0.1:%d", cfg.GRPCPort), grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(internalcaller.ClientInterceptor(testGatewayToken)))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	hc, err := grpc_health_v1.NewHealthClient(conn).Check(context.Background(), &grpc_health_v1.HealthCheckRequest{})
	if err != nil || hc.GetStatus() != grpc_health_v1.HealthCheckResponse_SERVING {
		t.Fatalf("grpc health = %v %v", hc, err)
	}

	// A global admin keeps these probes independent of project-service membership.
	rpcCtx := metadata.AppendToOutgoingContext(context.Background(), grpcmw.MetadataTenantID, uuid.NewString(), grpcmw.MetadataUserID, uuid.NewString(), grpcmw.MetadataRole, "admin")
	_, err = requestv1.NewRequestServiceClient(conn).GetRequest(rpcCtx, &requestv1.GetRequestRequest{Id: uuid.NewString()})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("GetRequest on a fresh DB must be NotFound, got %v", err)
	}
	list, err := requestv1.NewRequestServiceClient(conn).ListRequests(rpcCtx, &requestv1.ListRequestsRequest{})
	if err != nil || len(list.GetRequests()) != 0 {
		t.Fatalf("ListRequests = %v %v", list, err)
	}
	_, err = requestv1.NewApprovalServiceClient(conn).GetApproval(rpcCtx, &requestv1.GetApprovalRequest{Id: uuid.NewString()})
	if cfg.ApprovalEnabled {
		if status.Code(err) != codes.NotFound {
			t.Fatalf("ApprovalService enabled: GetApproval on an unknown id must be NotFound, got %v", err)
		}
	} else if status.Code(err) != codes.Unimplemented {
		t.Fatalf("ApprovalService must not be served when approval is off, got %v", err)
	}

	if extra != nil {
		extra()
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("clean shutdown expected, got %v", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("service did not stop after context cancel")
	}
}
