package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

type handshakeStub struct {
	info HandshakeInfo
	ok   bool
}

func (h handshakeStub) LastHandshakeInfo(string) (HandshakeInfo, bool) { return h.info, h.ok }

type frozenClock time.Time

func (c frozenClock) Now() time.Time { return time.Time(c) }

func recorderFor(health domain.DevServerHealthStatus, src HandshakeSource) (*RecordDevServerHandshake, *fakeDevServerRepository) {
	repo := &fakeDevServerRepository{byID: map[string]domain.DevServer{"ds-1": {ID: "ds-1", TenantID: "t1", HealthStatus: health}}}
	return NewRecordDevServerHandshake(repo, src, frozenClock(time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC))), repo
}

func TestRecordDevServerHandshake_PendingBecomesHealthyWithTheAgentsFacts(t *testing.T) {
	info := HandshakeInfo{Platform: "darwin", Arch: "arm64", NodeVersion: "v22.1.0", AgentVersion: "5.0.0"}
	uc, repo := recorderFor(domain.DevServerHealthPending, handshakeStub{info: info, ok: true})

	if err := uc.Execute(context.Background(), "t1", "ds-1"); err != nil {
		t.Fatal(err)
	}
	if repo.updateProvisionResultCalls != 1 || repo.lastProvisionStatus != domain.DevServerHealthHealthy || repo.lastProvisionInfo.Platform != "darwin" || repo.lastProvisionInfo.NodeVersion != "v22.1.0" {
		t.Fatalf("calls=%d status=%q info=%+v", repo.updateProvisionResultCalls, repo.lastProvisionStatus, repo.lastProvisionInfo)
	}
}

func TestRecordDevServerHandshake_AVerdictFromTheHealthPollSurvivesAReconnect(t *testing.T) {
	uc, repo := recorderFor(domain.DevServerHealthDegraded, handshakeStub{info: HandshakeInfo{Platform: "linux"}, ok: true})
	if err := uc.Execute(context.Background(), "t1", "ds-1"); err != nil {
		t.Fatal(err)
	}
	if repo.lastProvisionStatus != domain.DevServerHealthDegraded || repo.lastProvisionInfo.Platform != "linux" {
		t.Fatalf("status=%q info=%+v", repo.lastProvisionStatus, repo.lastProvisionInfo)
	}
}

func TestRecordDevServerHandshake_SessionAlreadyGoneWritesNothing(t *testing.T) {
	uc, repo := recorderFor(domain.DevServerHealthPending, handshakeStub{})
	if err := uc.Execute(context.Background(), "t1", "ds-1"); err != nil || repo.updateProvisionResultCalls != 0 {
		t.Fatalf("err=%v calls=%d", err, repo.updateProvisionResultCalls)
	}
}

func TestRecordDevServerHandshake_StoreFailuresAreReported(t *testing.T) {
	uc, repo := recorderFor(domain.DevServerHealthPending, handshakeStub{info: HandshakeInfo{Platform: "linux"}, ok: true})
	repo.updateProvisionResultErr = errors.New("db down")
	if err := uc.Execute(context.Background(), "t1", "ds-1"); err == nil {
		t.Fatal("expected an error")
	}
	repo.getErr = errors.New("db down")
	if err := uc.Execute(context.Background(), "t1", "ds-1"); err == nil {
		t.Fatal("expected an error")
	}
}
