package main

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/adapter/grpcclient"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type wiringStore struct {
	usecase.RequestRepository
	usecase.RequestLocker
	usecase.TxRunner
	usecase.OutboxWriter
}

type wiringBaseRefWriter struct{}

func (wiringBaseRefWriter) SetBaseRef(context.Context, string, string, int64) (domain.Request, error) {
	return domain.Request{}, nil
}

type wiringPresence struct{}

func (wiringPresence) Has(context.Context, string) (bool, domain.BaseRefLockReason, error) {
	return false, domain.BaseRefUnlocked, nil
}
func (wiringPresence) HasMany(context.Context, []string) (map[string]domain.BaseRefLockReason, error) {
	return nil, nil
}

func baseRefStores() *requestStores {
	s := &wiringStore{}
	return &requestStores{requests: s, locker: s, tx: s, outboxWriter: s, baseRefs: wiringBaseRefWriter{}, worktreeRecords: wiringPresence{}}
}

func TestWireBaseRef_BuildsWhenEveryPortIsThere(t *testing.T) {
	w, err := wireBaseRef(slog.New(slog.NewTextHandler(io.Discard, nil)), baseRefStores(), grpcclient.UnavailableTaskClient{}, grpcclient.UnavailableBaseRefChecker{})
	if err != nil || w.Set == nil || w.Locks == nil || w.Checker == nil {
		t.Fatalf("%+v %v", w, err)
	}
}

func TestWireBaseRef_AMissingPortStopsStartup(t *testing.T) {
	cases := map[string]struct {
		mutate  func(s *requestStores)
		tasks   usecase.TaskClient
		checker usecase.BaseRefChecker
		want    string
	}{
		"base ref writer":   {mutate: func(s *requestStores) { s.baseRefs = nil }, tasks: grpcclient.UnavailableTaskClient{}, checker: grpcclient.UnavailableBaseRefChecker{}, want: "base ref writer"},
		"presence store":    {mutate: func(s *requestStores) { s.worktreeRecords = nil }, tasks: grpcclient.UnavailableTaskClient{}, checker: grpcclient.UnavailableBaseRefChecker{}, want: "worktree presence"},
		"task client":       {mutate: func(*requestStores) {}, tasks: nil, checker: grpcclient.UnavailableBaseRefChecker{}, want: "task client"},
		"checker":           {mutate: func(*requestStores) {}, tasks: grpcclient.UnavailableTaskClient{}, checker: nil, want: "checker"},
		"locker (use case)": {mutate: func(s *requestStores) { s.locker = nil }, tasks: grpcclient.UnavailableTaskClient{}, checker: grpcclient.UnavailableBaseRefChecker{}, want: "locker"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			stores := baseRefStores()
			c.mutate(stores)
			w, err := wireBaseRef(slog.Default(), stores, c.tasks, c.checker)
			if err == nil || w != nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want an error mentioning %q, got %v", c.want, err)
			}
		})
	}
}
