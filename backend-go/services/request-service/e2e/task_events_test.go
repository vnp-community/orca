//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/internalcaller"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// taskStub is task-service as the real binary sees it: gRPC over TCP, backed by the in-memory fake the contract
// tests share. What task-service would publish on NATS (orca.task.task.statuschanged) arrives through the
// ReportTaskOutcome RPC instead, which is the same use case, so the stack needs no second publisher.
type taskStub struct {
	*contracttest.FakeTaskService
	addr string
	srv  *grpc.Server

	closeQueue sync.Once
	mu         sync.Mutex
	queue      chan contracttest.StatusChange
	delivered  map[string][]*requestv1.ReportTaskOutcomeRequest // by request id, in delivery order
	deliverErr []string
}

func startTaskStub() (*taskStub, error) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	t := &taskStub{FakeTaskService: contracttest.NewFakeTaskService(), addr: lis.Addr().String(), srv: grpc.NewServer(),
		queue: make(chan contracttest.StatusChange, 1024), delivered: map[string][]*requestv1.ReportTaskOutcomeRequest{}}
	t.Register(t.srv)
	go func() { _ = t.srv.Serve(lis) }()
	return t, nil
}

func (t *taskStub) stop() {
	t.srv.Stop()
	t.closeQueue.Do(func() { close(t.queue) })
}

// deliverTo starts forwarding task events to the service. They go through one queue, in order: the service takes its
// Request lock while it dispatches a task, and delivering an event from inside that call would deadlock on it.
func (t *taskStub) deliverTo(grpcAddr string) {
	conn, err := grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}
	client := requestv1.NewRequestServiceClient(conn)
	t.OnChange = func(c contracttest.StatusChange) { t.queue <- c }
	go func() {
		defer func() { _ = conn.Close() }()
		for c := range t.queue {
			t.send(client, &requestv1.ReportTaskOutcomeRequest{
				EventId: uuid.NewString(), RequestId: c.RequestID, TaskId: c.TaskID, TaskType: c.TaskType, PreviousStatus: c.Previous, NewStatus: c.New,
				Cause: c.Cause, ExecutionLinkId: c.LinkID, ErrorMessage: c.Error, OccurredAt: timestamppb.Now(),
			}, c.TenantID)
		}
	}()
}

func serviceCtx(tenantID string) context.Context {
	return metadata.AppendToOutgoingContext(context.Background(), grpcmw.MetadataTenantID, tenantID, internalcaller.MetadataKey, serviceToken)
}

func (t *taskStub) send(client requestv1.RequestServiceClient, req *requestv1.ReportTaskOutcomeRequest, tenantID string) {
	t.mu.Lock()
	t.delivered[req.GetRequestId()] = append(t.delivered[req.GetRequestId()], req)
	t.mu.Unlock()
	ctx, cancel := context.WithTimeout(serviceCtx(tenantID), 20*time.Second)
	defer cancel()
	if _, err := client.ReportTaskOutcome(ctx, req); err != nil {
		t.mu.Lock()
		t.deliverErr = append(t.deliverErr, fmt.Sprintf("%s %s->%s: %v", req.GetCause(), req.GetPreviousStatus(), req.GetNewStatus(), err))
		t.mu.Unlock()
		fmt.Fprintf(os.Stderr, "e2e: ReportTaskOutcome(%s %s->%s) failed: %v\n", req.GetCause(), req.GetPreviousStatus(), req.GetNewStatus(), err)
	}
}

// eventsOf returns the events delivered for a Request, oldest first.
func (t *taskStub) eventsOf(requestID string) []*requestv1.ReportTaskOutcomeRequest {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]*requestv1.ReportTaskOutcomeRequest(nil), t.delivered[requestID]...)
}

func (t *taskStub) deliveryErrors() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]string(nil), t.deliverErr...)
}
