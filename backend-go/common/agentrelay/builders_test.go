package agentrelay

import (
	"errors"
	"reflect"
	"testing"
)

func TestBuilders(t *testing.T) {
	env := map[string]string{"ORCA_REQUEST_ID": "r1"}
	base := Run{StepID: "s", Prompt: "p", WorktreePath: "/w", Env: env, TimeoutMS: 1000, Model: "m", AccountID: "a", InitFile: "i"}
	withHub := base
	withHub.KnowledgeRoot, withHub.ProposeChangesets = "/root", true
	proposeNoRoot := base
	proposeNoRoot.ProposeChangesets = true
	withNonce := base
	withNonce.ResultNonce = "Nonce0123456789ab"
	withTrust := base
	withTrust.TrustPreset = TrustFull

	tests := []struct {
		name  string
		build func() (ExecParams, error)
		want  ExecParams
		err   error
	}{
		{"plain keeps the caller's trust", func() (ExecParams, error) { return Plain(withTrust), nil },
			ExecParams{StepID: "s", Prompt: "p", WorktreePath: "/w", TrustPreset: TrustFull, Env: env, TimeoutMS: 1000, Model: "m", AccountID: "a", InitFile: "i"}, nil},
		{"plain with nonce sends a result block", func() (ExecParams, error) { return Plain(withNonce), nil },
			ExecParams{StepID: "s", Prompt: "p", WorktreePath: "/w", Env: env, TimeoutMS: 1000, Model: "m", AccountID: "a", InitFile: "i", ResultBlock: &ResultBlock{Nonce: "Nonce0123456789ab"}}, nil},
		{"plain with knowledge hub", func() (ExecParams, error) { return Plain(withHub), nil },
			ExecParams{StepID: "s", Prompt: "p", WorktreePath: "/w", Env: env, TimeoutMS: 1000, Model: "m", AccountID: "a", InitFile: "i", KnowledgeRoot: "/root", ProposeChangesets: true}, nil},
		{"propose without a root is dropped", func() (ExecParams, error) { return Plain(proposeNoRoot), nil },
			ExecParams{StepID: "s", Prompt: "p", WorktreePath: "/w", Env: env, TimeoutMS: 1000, Model: "m", AccountID: "a", InitFile: "i"}, nil},
		{"read-only drops the trust preset", func() (ExecParams, error) { return ReadOnly(withTrust), nil },
			ExecParams{StepID: "s", Prompt: "p", WorktreePath: "/w", Env: env, TimeoutMS: 1000, Model: "m", AccountID: "a", InitFile: "i",
				AccessMode: AccessReadOnly, WorkspaceKind: WorkspaceRepoRoot, ReportChanges: true, MaxOutputBytes: AnalysisMaxOutputBytes}, nil},
		{"read-only with hub", func() (ExecParams, error) { return ReadOnly(withHub), nil },
			ExecParams{StepID: "s", Prompt: "p", WorktreePath: "/w", Env: env, TimeoutMS: 1000, Model: "m", AccountID: "a", InitFile: "i", KnowledgeRoot: "/root", ProposeChangesets: true,
				AccessMode: AccessReadOnly, WorkspaceKind: WorkspaceRepoRoot, ReportChanges: true, MaxOutputBytes: AnalysisMaxOutputBytes}, nil},
		{"write profile sends the default trust", func() (ExecParams, error) { return WriteProfile(withTrust, "spec-author") },
			ExecParams{StepID: "s", Prompt: "p", WorktreePath: "/w", TrustPreset: TrustDefault, Env: env, TimeoutMS: 1000, Model: "m", AccountID: "a", InitFile: "i",
				AccessMode: AccessWrite, WorkspaceKind: WorkspaceWorktree, ReportChanges: true, MaxOutputBytes: AnalysisMaxOutputBytes, PermissionProfile: "spec-author"}, nil},
		{"write profile needs a name", func() (ExecParams, error) { return WriteProfile(base, "") }, ExecParams{}, ErrProfileRequired},
		{"contract applies the long limits", func() (ExecParams, error) { return Contract(withNonce), nil },
			ExecParams{StepID: "s", Prompt: "p", WorktreePath: "/w", Env: env, TimeoutMS: 1000, Model: "m", AccountID: "a", InitFile: "i",
				ResultBlock: &ResultBlock{Nonce: "Nonce0123456789ab"}, ReportChanges: true, MaxOutputBytes: ContractMaxOutputBytes}, nil},
		{"contract defaults the timeout", func() (ExecParams, error) { r := withNonce; r.TimeoutMS = 0; return Contract(r), nil },
			ExecParams{StepID: "s", Prompt: "p", WorktreePath: "/w", Env: env, TimeoutMS: ContractTimeoutMS, Model: "m", AccountID: "a", InitFile: "i",
				ResultBlock: &ResultBlock{Nonce: "Nonce0123456789ab"}, ReportChanges: true, MaxOutputBytes: ContractMaxOutputBytes}, nil},
		{"contract sends the nonce even when empty", func() (ExecParams, error) { return Contract(base), nil },
			ExecParams{StepID: "s", Prompt: "p", WorktreePath: "/w", Env: env, TimeoutMS: 1000, Model: "m", AccountID: "a", InitFile: "i",
				ResultBlock: &ResultBlock{}, ReportChanges: true, MaxOutputBytes: ContractMaxOutputBytes}, nil},
		{"max output override", func() (ExecParams, error) { r := base; r.MaxOutputBytes = 99999; return ReadOnly(r), nil },
			ExecParams{StepID: "s", Prompt: "p", WorktreePath: "/w", Env: env, TimeoutMS: 1000, Model: "m", AccountID: "a", InitFile: "i",
				AccessMode: AccessReadOnly, WorkspaceKind: WorkspaceRepoRoot, ReportChanges: true, MaxOutputBytes: 99999}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.build()
			if !errors.Is(err, tc.err) {
				t.Fatalf("err = %v, want %v", err, tc.err)
			}
			if tc.err == nil && !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got  %+v\nwant %+v", got, tc.want)
			}
		})
	}
}

func TestBuilderOutputsPassValidation(t *testing.T) {
	run := Run{Prompt: "p", WorktreePath: "/w", KnowledgeRoot: "/r", ProposeChangesets: true, ResultNonce: "Nonce0123456789ab"}
	wp, err := WriteProfile(run, "spec-author")
	if err != nil {
		t.Fatal(err)
	}
	for name, p := range map[string]ExecParams{"plain": Plain(run), "read-only": ReadOnly(run), "write profile": wp, "contract": Contract(run)} {
		if err := p.validate(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}
