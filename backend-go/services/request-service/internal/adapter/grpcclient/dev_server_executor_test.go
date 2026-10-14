package grpcclient

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestDevServerExecutor_SendsBinaryAndArgsAsSeparateFields(t *testing.T) {
	infra := &fakeAgentInfra{result: `{"stdout":"1.3.0\n","stderr":"","exitCode":0}`}
	res, err := NewDevServerExecutor(infra).Exec(ctxWithIdentity(), usecase.AnalysisConnection{ConnectionID: "c1"},
		usecase.ExecInput{Binary: "openspec", Args: []string{"--version", "; rm -rf /"}, Cwd: "/srv/repo", TimeoutMs: 1000})
	if err != nil || res.ExitCode != 0 || res.Stdout != "1.3.0\n" {
		t.Fatalf("res %+v err %v", res, err)
	}
	if len(infra.relay) != 1 || infra.relay[0].GetMethod() != "agent.exec" || infra.relay[0].GetConnectionId() != "c1" {
		t.Fatalf("relay calls: %+v", infra.relay)
	}
	p := paramsOf(t, infra.relay[0].GetParamsJson())
	if p["binary"] != "openspec" || p["cwd"] != "/srv/repo" || p["timeoutMs"].(float64) != 1000 {
		t.Fatalf("params: %v", p)
	}
	args := p["args"].([]any)
	if len(args) != 2 || args[1] != "; rm -rf /" {
		t.Fatalf("args must travel untouched as an array: %v", args)
	}
	if _, ok := p["command"]; ok {
		t.Fatalf("no shell command string may be sent: %v", p)
	}
}

func TestDevServerExecutor_RoutesByDevServerWhenThereIsNoConnection(t *testing.T) {
	infra := &fakeAgentInfra{result: `{"exitCode":0}`}
	if _, err := NewDevServerExecutor(infra).Exec(ctxWithIdentity(), usecase.AnalysisConnection{DevServerID: "ds-1"}, usecase.ExecInput{Binary: "x"}); err != nil {
		t.Fatal(err)
	}
	if len(infra.dev) != 1 || infra.dev[0].GetDevServerId() != "ds-1" || len(infra.relay) != 0 {
		t.Fatalf("dev=%+v relay=%+v", infra.dev, infra.relay)
	}
}

func TestDevServerExecutor_ClampsTheTimeoutToTheAgentCeiling(t *testing.T) {
	infra := &fakeAgentInfra{result: `{"exitCode":0}`}
	ex := NewDevServerExecutor(infra)
	for _, ask := range []int{0, -5, 99 * 60 * 1000} {
		if _, err := ex.Exec(ctxWithIdentity(), usecase.AnalysisConnection{ConnectionID: "c"}, usecase.ExecInput{Binary: "x", TimeoutMs: ask}); err != nil {
			t.Fatal(err)
		}
	}
	for i, call := range infra.relay {
		if got := paramsOf(t, call.GetParamsJson())["timeoutMs"].(float64); got != agentExecCeilingMs {
			t.Fatalf("call %d: timeoutMs = %v", i, got)
		}
	}
}

func TestDevServerExecutor_MapsResults(t *testing.T) {
	cases := map[string]struct {
		result string
		want   usecase.ExecResult
	}{
		"non-zero exit":                      {`{"stdout":"","stderr":"boom","exitCode":2}`, usecase.ExecResult{Stderr: "boom", ExitCode: 2}},
		"spawn error":                        {`{"exitCode":null,"spawnError":"ENOENT"}`, usecase.ExecResult{ExitCode: -1, SpawnError: "ENOENT"}},
		"timed out":                          {`{"exitCode":null,"timedOut":true}`, usecase.ExecResult{ExitCode: -1, TimedOut: true}},
		"no exit code means never completed": {`{"stdout":"x"}`, usecase.ExecResult{Stdout: "x", ExitCode: -1}},
	}
	for name, tc := range cases {
		got, err := NewDevServerExecutor(&fakeAgentInfra{result: tc.result}).Exec(ctxWithIdentity(), usecase.AnalysisConnection{ConnectionID: "c"}, usecase.ExecInput{Binary: "x"})
		if err != nil || got != tc.want {
			t.Fatalf("%s: got %+v err %v want %+v", name, got, err, tc.want)
		}
	}
}

func TestDevServerExecutor_Errors(t *testing.T) {
	ex := func(infra *fakeAgentInfra, ctx context.Context, bin string) (usecase.ExecResult, error) {
		return NewDevServerExecutor(infra).Exec(ctx, usecase.AnalysisConnection{ConnectionID: "c"}, usecase.ExecInput{Binary: bin})
	}
	if _, err := ex(&fakeAgentInfra{}, ctxWithIdentity(), ""); err == nil {
		t.Fatal("an empty binary must be refused before any call")
	}
	if _, err := ex(&fakeAgentInfra{result: "not json"}, ctxWithIdentity(), "x"); err == nil {
		t.Fatal("garbage must error")
	}
	if _, err := ex(&fakeAgentInfra{err: errors.New("boom")}, ctxWithIdentity(), "x"); err == nil || !strings.Contains(err.Error(), "relay agent.exec") {
		t.Fatalf("got %v", err)
	}
	res, err := ex(&fakeAgentInfra{err: status.Error(codes.DeadlineExceeded, "slow")}, ctxWithIdentity(), "x")
	if err != nil || !res.TimedOut {
		t.Fatalf("a relay deadline is a timed out result, got %+v %v", res, err)
	}
	if _, err := ex(&fakeAgentInfra{}, context.Background(), "x"); err == nil {
		t.Fatal("a call without a tenant must fail")
	}
}

func TestDevServerExecutor_ParamsAreValidJSONWithAnEmptyArgsArray(t *testing.T) {
	infra := &fakeAgentInfra{result: `{"exitCode":0}`}
	if _, err := NewDevServerExecutor(infra).Exec(ctxWithIdentity(), usecase.AnalysisConnection{ConnectionID: "c"}, usecase.ExecInput{Binary: "x"}); err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	_ = json.Unmarshal([]byte(infra.relay[0].GetParamsJson()), &m)
	if string(m["args"]) != "[]" {
		t.Fatalf("args = %s", m["args"])
	}
}
