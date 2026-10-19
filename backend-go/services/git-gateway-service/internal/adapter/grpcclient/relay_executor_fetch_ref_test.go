package grpcclient

import (
	"encoding/json"
	"strings"
	"testing"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// gitAnswers scripts the agent's git.exec by the git subcommand and the first revision it is asked about.
type gitAnswers map[string]string

func (a gitAnswers) respond(req *infrafleetv1.RelayRequest) (*infrafleetv1.RelayResponse, error) {
	var p struct {
		Args []string `json:"args"`
	}
	_ = json.Unmarshal([]byte(req.GetParamsJson()), &p)
	key := strings.Join(p.Args, " ")
	for prefix, body := range a {
		if strings.HasPrefix(key, prefix) {
			return &infrafleetv1.RelayResponse{ResultJson: body}, nil
		}
	}
	return &infrafleetv1.RelayResponse{ResultJson: `{"stdout":"","stderr":"unscripted ` + key + `","exitCode":2}`}, nil
}

func fetchRefExecutor(a gitAnswers) (*RelayExecutor, *fakeInfraFleetServiceClient) {
	fake := &fakeInfraFleetServiceClient{relayRespFunc: a.respond}
	return NewRelayExecutor(fake), fake
}

func TestRelayExecutor_FetchAndResolveRef_UsesAllowedGitExecNeverTheMissingFetchRefMethod(t *testing.T) {
	r, fake := fetchRefExecutor(gitAnswers{
		"fetch origin release/x":       `{"stdout":"","stderr":"","exitCode":0}`,
		"rev-parse --verify --quiet F": `{"stdout":"deadbeef\n","stderr":"","exitCode":0}`,
	})
	sha, err := r.FetchAndResolveRef(ctxWithTenant(t), "/repo", "release/x")
	if err != nil || sha != "deadbeef" {
		t.Fatalf("sha=%q err=%v", sha, err)
	}
	for _, req := range fake.gotRelayRequests {
		if req.GetMethod() != "git.exec" {
			t.Errorf("the agent has no %s: only git.exec may be relayed", req.GetMethod())
		}
	}
	if len(fake.gotRelayRequests) != 2 || !strings.Contains(fake.gotRelayRequests[1].GetParamsJson(), "FETCH_HEAD^{commit}") {
		t.Errorf("fetch, then FETCH_HEAD as a commit: %+v", fake.gotRelayRequests)
	}
}

func TestRelayExecutor_FetchAndResolveRef_FailedFetch_FallsBackToTheLocalCommit(t *testing.T) {
	r, _ := fetchRefExecutor(gitAnswers{
		"fetch origin":                 `{"stdout":"","stderr":"fatal: couldn't find remote ref local-only","exitCode":128}`,
		"rev-parse --verify --quiet l": `{"stdout":"cafe01\n","stderr":"","exitCode":0}`,
	})
	sha, err := r.FetchAndResolveRef(ctxWithTenant(t), "/repo", "local-only")
	if err != nil || sha != "cafe01" {
		t.Fatalf("sha=%q err=%v", sha, err)
	}
}

func TestRelayExecutor_FetchAndResolveRef_MissingEverywhere_ReturnsTheFetchErrorClassifiedAsMissing(t *testing.T) {
	r, _ := fetchRefExecutor(gitAnswers{
		"fetch origin":       `{"stdout":"","stderr":"fatal: couldn't find remote ref nope","exitCode":128}`,
		"rev-parse --verify": `{"stdout":"","stderr":"","exitCode":1}`,
	})
	_, err := r.FetchAndResolveRef(ctxWithTenant(t), "/repo", "nope")
	if err == nil || !domain.IsRemoteRefMissing(err) {
		t.Fatalf("want a missing-ref error, got %v", err)
	}
}

func TestRelayExecutor_FetchAndResolveRef_NetworkFailure_IsNotMissing(t *testing.T) {
	r, _ := fetchRefExecutor(gitAnswers{
		"fetch origin":       `{"stdout":"","stderr":"fatal: unable to access 'https://x/': Could not resolve host: x","exitCode":128}`,
		"rev-parse --verify": `{"stdout":"","stderr":"","exitCode":1}`,
	})
	_, err := r.FetchAndResolveRef(ctxWithTenant(t), "/repo", "release/x")
	if err == nil || domain.IsRemoteRefMissing(err) || !domain.IsRemoteUnreachable(err) {
		t.Fatalf("got %v", err)
	}
}

func TestRelayExecutor_FetchAndResolveRef_OptionLikeRef_NeverRelayed(t *testing.T) {
	r, fake := fetchRefExecutor(gitAnswers{})
	if _, err := r.FetchAndResolveRef(ctxWithTenant(t), "/repo", "--upload-pack=x"); err == nil {
		t.Fatal("expected a refusal")
	}
	if len(fake.gotRelayRequests) != 0 {
		t.Fatal("nothing may reach the agent")
	}
}
