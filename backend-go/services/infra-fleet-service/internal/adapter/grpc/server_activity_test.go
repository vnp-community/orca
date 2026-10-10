package grpc

import (
	"testing"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

type activityStub map[string]int

func (a activityStub) ActiveExecutions(id string) int { return a[id] }

func TestWithActivity_FillsActiveExecutionsFromTheLiveSession(t *testing.T) {
	s := (&Server{}).WithDevServerActivity(activityStub{"busy": 3})
	if got := s.withActivity(&infrafleetv1.DevServer{Id: "busy"}).GetActiveExecutions(); got != 3 {
		t.Fatalf("busy dev server: %d", got)
	}
	if got := s.withActivity(&infrafleetv1.DevServer{Id: "idle"}).GetActiveExecutions(); got != 0 {
		t.Fatalf("idle dev server: %d", got)
	}
}

func TestWithActivity_WithoutASourceLeavesTheFieldZero(t *testing.T) {
	if got := (&Server{}).withActivity(&infrafleetv1.DevServer{Id: "x", ActiveExecutions: 0}).GetActiveExecutions(); got != 0 {
		t.Fatalf("got %d", got)
	}
}
