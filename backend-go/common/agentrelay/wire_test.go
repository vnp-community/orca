package agentrelay

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var updateWire = flag.Bool("update", false, "rewrite testdata/wire from the legacy structs")

func marshalJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

// canonical re-encodes JSON with sorted keys, so two documents compare by content and not by key order.
func canonical(t *testing.T, raw []byte) []byte {
	t.Helper()
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("unmarshal %s: %v", raw, err)
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("indent: %v", err)
	}
	return append(out, '\n')
}

type wireCase struct {
	name string
	// legacy is the JSON the pre-consolidation struct of that flow produced.
	legacy any
	// shared is what the shared package produces for the same flow.
	shared any
	// sameBytes is set where the legacy key order is preserved too (request-service).
	sameBytes bool
}

func mustWriteProfile(t *testing.T, r Run, profile string) ExecParams {
	t.Helper()
	p, err := WriteProfile(r, profile)
	if err != nil {
		t.Fatalf("WriteProfile: %v", err)
	}
	return p
}

func requestRun(in legacyRequestInput) Run {
	return Run{
		StepID: in.StepID, Prompt: in.Prompt, WorktreePath: in.RepoPath, TimeoutMS: in.TimeoutMS,
		Env:           map[string]string{"ORCA_REQUEST_ID": in.RequestID, "ORCA_PROJECT_ID": in.ProjectID},
		KnowledgeRoot: in.KnowledgeRoot, ProposeChangesets: in.ProposeChangesets, ResultNonce: in.ResultNonce,
	}
}

func wireCases(t *testing.T) []wireCase {
	t.Helper()
	base := legacyRequestInput{StepID: "step-1", Prompt: "Analyse the repo", RepoPath: "/srv/repo", RequestID: "req-1", ProjectID: "proj-1", TimeoutMS: 600000}
	readonly := base
	readonly.ReadOnlyEnforced = true
	readonlyHub := readonly
	readonlyHub.KnowledgeRoot, readonlyHub.ProposeChangesets = "/srv/repo", true
	writeProfile := base
	writeProfile.WriteProfile, writeProfile.RepoPath = "spec-author", "/srv/wt/req-1"
	writeHub := writeProfile
	writeHub.KnowledgeRoot = "/srv/repo"
	requestContract := base
	requestContract.ResultNonce = "AbCdEfGh01234567"
	requestPlain := base
	requestPlain.TimeoutMS = 0

	taskEnv := map[string]string{"ORCA_TASK_ID": "t-1", "ORCA_PROJECT_ID": "p-1", "ORCA_USER_ID": "u-1", "PATH": "/opt/bin"}
	taskPlain := legacyTaskParams{Prompt: "Do task", WorktreePath: "/srv/wt", StepID: "req:1", Env: map[string]string{"ORCA_TASK_ID": "t-1", "ORCA_PROJECT_ID": "p-1"}, TrustPreset: "full"}
	taskProfile := taskPlain
	taskProfile.Model, taskProfile.Env, taskProfile.InitFile = "claude-sonnet", taskEnv, "# Project\nname: demo"
	taskContract := taskProfile
	legacyApplyContract(&taskContract, "Nonce0123456789ab")
	taskContractHub := taskContract
	taskContractHub.KnowledgeRoot = "/srv/repo"

	taskRun := Run{Prompt: "Do task", WorktreePath: "/srv/wt", StepID: "req:1", TrustPreset: "full", Env: map[string]string{"ORCA_TASK_ID": "t-1", "ORCA_PROJECT_ID": "p-1"}}
	taskProfileRun := taskRun
	taskProfileRun.Model, taskProfileRun.Env, taskProfileRun.InitFile = "claude-sonnet", taskEnv, "# Project\nname: demo"
	taskContractRun := taskProfileRun
	taskContractRun.ResultNonce = "Nonce0123456789ab"
	taskContractHubRun := taskContractRun
	taskContractHubRun.KnowledgeRoot = "/srv/repo"

	wfBare := legacyWorkflowParams{Prompt: "Review", WorktreePath: "/srv/wt", TrustPreset: "default", AccountID: "acc-1", Model: "claude-opus"}
	wfProfile := wfBare
	wfProfile.Env, wfProfile.InitFile = map[string]string{"ORCA_USER_ID": "u-1", "ORCA_PROJECT_NAME": "demo"}, "# Project"

	return []wireCase{
		{"request_readonly", legacyBuildRequestParams(readonly), ReadOnly(requestRun(readonly)), true},
		{"request_readonly_knowledge", legacyBuildRequestParams(readonlyHub), ReadOnly(requestRun(readonlyHub)), true},
		{"request_write_profile", legacyBuildRequestParams(writeProfile), mustWriteProfile(t, requestRun(writeProfile), "spec-author"), true},
		{"request_write_profile_knowledge", legacyBuildRequestParams(writeHub), mustWriteProfile(t, requestRun(writeHub), "spec-author"), true},
		{"request_contract", legacyBuildRequestParams(requestContract), Plain(withTrust(requestRun(requestContract), TrustDefault)), true},
		{"request_plain", legacyBuildRequestParams(requestPlain), Plain(withTrust(requestRun(requestPlain), TrustDefault)), true},
		{"task_plain", taskPlain, Plain(taskRun), false},
		{"task_profile", taskProfile, Plain(taskProfileRun), false},
		{"task_contract", taskContract, Contract(taskContractRun), false},
		{"task_contract_knowledge", taskContractHub, Contract(taskContractHubRun), false},
		{"workflow_bare", wfBare, Plain(Run{Prompt: "Review", WorktreePath: "/srv/wt", TrustPreset: "default", AccountID: "acc-1", Model: "claude-opus"}), false},
		{"workflow_profile", wfProfile, Plain(Run{Prompt: "Review", WorktreePath: "/srv/wt", TrustPreset: "default", AccountID: "acc-1", Model: "claude-opus", Env: map[string]string{"ORCA_USER_ID": "u-1", "ORCA_PROJECT_NAME": "demo"}, InitFile: "# Project"}), false},
		{"orchestration", legacyOrchestrationParams{Prompt: "Fix the login bug", WorktreePath: "/srv/wt/t-9", StepID: "handle-9"}, Plain(Run{Prompt: "Fix the login bug", WorktreePath: "/srv/wt/t-9", StepID: "handle-9"}), false},
		{"complete_request_account", legacyRequestComplete("acc-7", "Classify this"), CompleteParams{Prompt: "Classify this", AccountID: "acc-7"}, false},
		{"complete_request_plain", legacyRequestComplete("", "Write a solution"), CompleteParams{Prompt: "Write a solution"}, false},
		{"complete_git", legacyGitComplete("Write a commit message"), CompleteParams{Prompt: "Write a commit message"}, false},
	}
}

func withTrust(r Run, trust string) Run {
	r.TrustPreset = trust
	return r
}

// TestWireEquivalence proves the shared builders send the JSON each service sent before: same keys and values for every
// flow, the same bytes where the key order is kept, and the frozen golden file for each.
func TestWireEquivalence(t *testing.T) {
	for _, tc := range wireCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			legacy, shared := marshalJSON(t, tc.legacy), marshalJSON(t, tc.shared)
			golden := filepath.Join("testdata", "wire", tc.name+".json")
			if *updateWire {
				if err := os.WriteFile(golden, canonical(t, legacy), 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("read golden (run with -update to create): %v", err)
			}
			if got := canonical(t, legacy); !bytes.Equal(got, want) {
				t.Errorf("legacy struct drifted from golden:\n got %s\nwant %s", got, want)
			}
			if got := canonical(t, shared); !bytes.Equal(got, want) {
				t.Errorf("shared package differs from the legacy wire format:\n got %s\nwant %s", got, want)
			}
			if tc.sameBytes && !bytes.Equal(legacy, shared) {
				t.Errorf("key order changed:\nlegacy %s\nshared %s", legacy, shared)
			}
		})
	}
}

func TestWireGoldenFilesAreAllUsed(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join("testdata", "wire"))
	if err != nil {
		t.Fatal(err)
	}
	used := map[string]bool{}
	for _, tc := range wireCases(t) {
		used[tc.name+".json"] = true
	}
	for _, e := range entries {
		if !used[e.Name()] {
			t.Errorf("orphan golden file %s", e.Name())
		}
	}
	if len(entries) != len(used) {
		t.Errorf("golden files = %d, cases = %d", len(entries), len(used))
	}
}

// TestCompleteNeverCarriesCredentials keeps the plaintext key of ai.complete out of this package, in the type, on the
// wire and in the source.
func TestCompleteNeverCarriesCredentials(t *testing.T) {
	const forbidden = "resolvedapikey"
	typ := reflect.TypeOf(CompleteParams{})
	for i := range typ.NumField() {
		f := typ.Field(i)
		if strings.Contains(strings.ToLower(f.Name+f.Tag.Get("json")), "apikey") || strings.Contains(strings.ToLower(f.Name+f.Tag.Get("json")), "secret") {
			t.Errorf("CompleteParams.%s looks like a credential", f.Name)
		}
	}
	full := marshalJSON(t, CompleteParams{Prompt: "p", Format: FormatJSON, TaskID: "t", Model: "m", MaxTokens: 10, AccountID: "a"})
	if strings.Contains(strings.ToLower(string(full)), forbidden) {
		t.Errorf("wire JSON carries a key: %s", full)
	}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(strings.ToLower(string(src)), forbidden) {
			t.Errorf("%s mentions the credential field", f)
		}
	}
}
