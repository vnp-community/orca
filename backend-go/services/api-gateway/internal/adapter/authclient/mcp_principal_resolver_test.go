package authclient

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"

	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

type fakeResolveClient struct {
	authv1.AuthServiceClient
	mu    sync.Mutex
	resp  *authv1.ResolveMcpPrincipalResponse
	err   error
	calls atomic.Int32
	gate  chan struct{}
}

func (f *fakeResolveClient) ResolveMcpPrincipal(_ context.Context, _ *authv1.ResolveMcpPrincipalRequest, _ ...grpc.CallOption) (*authv1.ResolveMcpPrincipalResponse, error) {
	f.calls.Add(1)
	if f.gate != nil {
		<-f.gate
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.resp, f.err
}

func (f *fakeResolveClient) set(resp *authv1.ResolveMcpPrincipalResponse, err error) {
	f.mu.Lock()
	f.resp, f.err = resp, err
	f.mu.Unlock()
}

var in1 = usecase.McpResolveInput{JTI: "j1", UserID: "u", TenantID: "t", TokenUse: "mcp_pat"}

func newTestResolver(c *fakeResolveClient) (*McpPrincipalResolver, *time.Time) {
	now := time.Unix(1000, 0)
	r := NewMcpPrincipalResolver(c, 30*time.Second)
	r.now = func() time.Time { return now }
	return r, &now
}

func TestMcpPrincipalResolver_CachesFor30sThenSeesDemotion(t *testing.T) {
	c := &fakeResolveClient{resp: &authv1.ResolveMcpPrincipalResponse{Active: true, Role: "admin"}}
	r, now := newTestResolver(c)
	for i := 0; i < 3; i++ {
		if res, err := r.Resolve(context.Background(), in1); err != nil || res.Role != "admin" {
			t.Fatalf("%v %v", res, err)
		}
	}
	if c.calls.Load() != 1 {
		t.Fatalf("calls = %d, want 1 (cached)", c.calls.Load())
	}
	c.set(&authv1.ResolveMcpPrincipalResponse{Active: true, Role: "user"}, nil) // demoted
	*now = now.Add(29 * time.Second)
	if res, _ := r.Resolve(context.Background(), in1); res.Role != "admin" {
		t.Fatal("still within the 30s window")
	}
	*now = now.Add(2 * time.Second)
	if res, _ := r.Resolve(context.Background(), in1); res.Role != "user" || c.calls.Load() != 2 {
		t.Fatalf("after TTL the demotion must be visible: %+v calls=%d", res, c.calls.Load())
	}
}

func TestMcpPrincipalResolver_RevocationVisibleWithinNegativeAndTTL(t *testing.T) {
	c := &fakeResolveClient{resp: &authv1.ResolveMcpPrincipalResponse{Active: true, Role: "user"}}
	r, now := newTestResolver(c)
	_, _ = r.Resolve(context.Background(), in1)
	c.set(&authv1.ResolveMcpPrincipalResponse{Active: false, InactiveReason: "revoked"}, nil)
	r.Invalidate("j1") // same-replica revoke is immediate
	if res, _ := r.Resolve(context.Background(), in1); res.Active || res.InactiveReason != "revoked" {
		t.Fatalf("%+v", res)
	}
	// Inactive answers are cached only 5s so a re-activated user recovers quickly.
	c.set(&authv1.ResolveMcpPrincipalResponse{Active: true, Role: "user"}, nil)
	*now = now.Add(6 * time.Second)
	if res, _ := r.Resolve(context.Background(), in1); !res.Active {
		t.Fatal("negative entry should have expired after 5s")
	}
}

func TestMcpPrincipalResolver_ErrorsAreNotCachedOrRetried(t *testing.T) {
	c := &fakeResolveClient{err: errors.New("unavailable")}
	r, _ := newTestResolver(c)
	if _, err := r.Resolve(context.Background(), in1); err == nil {
		t.Fatal("want error")
	}
	if c.calls.Load() != 1 {
		t.Fatalf("no automatic retry: calls=%d", c.calls.Load())
	}
	c.set(&authv1.ResolveMcpPrincipalResponse{Active: true, Role: "user"}, nil)
	if res, err := r.Resolve(context.Background(), in1); err != nil || !res.Active {
		t.Fatalf("error must not be cached: %v %v", res, err)
	}
}

func TestMcpPrincipalResolver_SingleflightCollapsesConcurrentMisses(t *testing.T) {
	c := &fakeResolveClient{resp: &authv1.ResolveMcpPrincipalResponse{Active: true, Role: "user"}, gate: make(chan struct{})}
	r, _ := newTestResolver(c)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, _ = r.Resolve(context.Background(), in1) }()
	}
	time.Sleep(50 * time.Millisecond)
	close(c.gate)
	wg.Wait()
	if n := c.calls.Load(); n != 1 {
		t.Fatalf("calls = %d, want 1", n)
	}
}
