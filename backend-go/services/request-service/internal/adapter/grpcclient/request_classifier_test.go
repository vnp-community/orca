package grpcclient

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc"
)

type stubResolver struct {
	conn AIConnection
	err  error
}

func (s stubResolver) ResolveForProject(context.Context, string) (AIConnection, error) {
	return s.conn, s.err
}

func in() usecase.ClassificationInput {
	return usecase.ClassificationInput{RequestID: "r", ProjectID: "p", Title: "t", Body: "b"}
}

func TestClassifier_UsesRelayByConnection(t *testing.T) {
	infra := &fakeInfra{replies: []string{validReply}}
	c := NewRelayClassifier(infra, stubResolver{conn: AIConnection{ConnectionID: "proj"}}, nil)
	p, err := c.Classify(ctxWithIdentity(), in())
	if err != nil || p.Type != domain.RequestTypeBug {
		t.Fatalf("%+v %v", p, err)
	}
	if len(infra.relayCalls) != 1 || len(infra.devCalls) != 0 || infra.relayCalls[0].GetMethod() != "ai.complete" || infra.relayCalls[0].GetConnectionId() != "proj" {
		t.Fatalf("%+v", infra.relayCalls)
	}
	var params map[string]any
	_ = json.Unmarshal([]byte(infra.relayCalls[0].GetParamsJson()), &params)
	if params["prompt"] == "" || params["prompt"] == nil {
		t.Fatal("prompt missing")
	}
	if got := infra.lastMetadata.Get("x-orca-tenant-id"); len(got) != 1 {
		t.Fatalf("tenant metadata not forwarded: %v", infra.lastMetadata)
	}
}

func TestClassifier_UsesRelayByDevServer(t *testing.T) {
	infra := &fakeInfra{replies: []string{validReply}}
	c := NewRelayClassifier(infra, stubResolver{conn: AIConnection{DevServerID: "ds-1"}}, nil)
	if _, err := c.Classify(ctxWithIdentity(), in()); err != nil {
		t.Fatal(err)
	}
	if len(infra.devCalls) != 1 || len(infra.relayCalls) != 0 || infra.devCalls[0].GetDevServerId() != "ds-1" || infra.devCalls[0].GetMethod() != "ai.complete" {
		t.Fatalf("%+v", infra.devCalls)
	}
}

func TestClassifier_RetriesOnceOnBadJSON(t *testing.T) {
	infra := &fakeInfra{replies: []string{"I think this is a bug", validReply}}
	c := NewRelayClassifier(infra, stubResolver{conn: AIConnection{ConnectionID: "x"}}, nil)
	if _, err := c.Classify(ctxWithIdentity(), in()); err != nil {
		t.Fatal(err)
	}
	if len(infra.relayCalls) != 2 {
		t.Fatalf("calls = %d", len(infra.relayCalls))
	}
	var second map[string]string
	_ = json.Unmarshal([]byte(infra.relayCalls[1].GetParamsJson()), &second)
	if second["prompt"] == "" || second["prompt"] == infra.relayCalls[0].GetParamsJson() {
		t.Fatal("retry should carry the stricter reminder")
	}
}

func TestClassifier_FailsAfterTwoBadOutputs(t *testing.T) {
	infra := &fakeInfra{replies: []string{"nope", "still nope"}}
	c := NewRelayClassifier(infra, stubResolver{conn: AIConnection{ConnectionID: "x"}}, nil)
	_, err := c.Classify(ctxWithIdentity(), in())
	if !errors.Is(err, domain.ErrProposalInvalid) || len(infra.relayCalls) != 2 {
		t.Fatalf("err=%v calls=%d", err, len(infra.relayCalls))
	}
}

func TestClassifier_ForeignEnumRejected(t *testing.T) {
	bad := `{"type":"rm -rf /","size":"M","urgency":"normal","confidence":1,"reason":"x"}`
	infra := &fakeInfra{replies: []string{bad, bad}}
	c := NewRelayClassifier(infra, stubResolver{conn: AIConnection{ConnectionID: "x"}}, nil)
	if _, err := c.Classify(ctxWithIdentity(), in()); !errors.Is(err, domain.ErrProposalInvalid) {
		t.Fatalf("err = %v", err)
	}
}

func TestClassifier_Timeout(t *testing.T) {
	infra := &fakeInfra{replies: []string{validReply}, relayDelay: time.Second}
	c := NewRelayClassifier(infra, stubResolver{conn: AIConnection{ConnectionID: "x"}}, nil)
	c.timeout = 30 * time.Millisecond
	_, err := c.Classify(ctxWithIdentity(), in())
	if !errors.Is(err, usecase.ErrClassifierTimeout) {
		t.Fatalf("err = %v", err)
	}
}

func TestClassifier_NoDevServerPassesThrough(t *testing.T) {
	infra := &fakeInfra{}
	c := NewRelayClassifier(infra, stubResolver{err: usecase.ErrNoDevServer}, nil)
	if _, err := c.Classify(ctxWithIdentity(), in()); !errors.Is(err, usecase.ErrNoDevServer) || len(infra.relayCalls)+len(infra.devCalls) != 0 {
		t.Fatalf("err = %v", err)
	}
}

func TestAIConnectionResolver_Connected(t *testing.T) {
	infra := &fakeInfra{connected: true}
	r := NewAIConnectionResolver(infra, &fakeProjects{})
	got, err := r.ResolveForProject(ctxWithIdentity(), "proj")
	if err != nil || got != (AIConnection{ConnectionID: "proj"}) {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestAIConnectionResolver_FallbackToDevServer(t *testing.T) {
	infra := &fakeInfra{health: []*infrafleetv1.DevServerHealth{{DevServerId: "ds-9", Reachable: true}}}
	projects := &fakeProjects{repos: []*projectv1.Repo{{DevServerId: "ds-9"}, {DevServerId: "other"}}}
	got, err := NewAIConnectionResolver(infra, projects).ResolveForProject(ctxWithIdentity(), "proj")
	if err != nil || got != (AIConnection{DevServerID: "ds-9"}) {
		t.Fatalf("%+v %v", got, err)
	}
	if len(projects.metadata.Get("x-orca-user-id")) != 1 || len(projects.metadata.Get("x-orca-tenant-id")) != 1 {
		t.Fatalf("project-service needs tenant and user: %v", projects.metadata)
	}
}

func TestAIConnectionResolver_NoRepoAndUnreachable(t *testing.T) {
	r := NewAIConnectionResolver(&fakeInfra{}, &fakeProjects{})
	if _, err := r.ResolveForProject(ctxWithIdentity(), "p"); !errors.Is(err, usecase.ErrNoDevServer) {
		t.Fatalf("no repo: %v", err)
	}
	r = NewAIConnectionResolver(&fakeInfra{}, &fakeProjects{repos: []*projectv1.Repo{{}}})
	if _, err := r.ResolveForProject(ctxWithIdentity(), "p"); !errors.Is(err, usecase.ErrNoDevServer) {
		t.Fatalf("repo without dev server: %v", err)
	}
	infra := &fakeInfra{health: []*infrafleetv1.DevServerHealth{{DevServerId: "ds", Reachable: false}}}
	r = NewAIConnectionResolver(infra, &fakeProjects{repos: []*projectv1.Repo{{DevServerId: "ds"}}})
	if _, err := r.ResolveForProject(ctxWithIdentity(), "p"); !errors.Is(err, usecase.ErrNoDevServer) {
		t.Fatalf("unreachable: %v", err)
	}
	r = NewAIConnectionResolver(&fakeInfra{}, &fakeProjects{repos: []*projectv1.Repo{{DevServerId: "ds"}}})
	if _, err := r.ResolveForProject(ctxWithIdentity(), "p"); !errors.Is(err, usecase.ErrNoDevServer) {
		t.Fatalf("no health sample: %v", err)
	}
}

func TestAIConnectionResolver_FallbackNeedsUser(t *testing.T) {
	r := NewAIConnectionResolver(&fakeInfra{}, &fakeProjects{})
	_, err := r.ResolveForProject(tenantOnly(), "p")
	if err == nil || errors.Is(err, usecase.ErrNoDevServer) {
		t.Fatalf("want a reporter error, got %v", err)
	}
}

var _ grpc.CallOption
