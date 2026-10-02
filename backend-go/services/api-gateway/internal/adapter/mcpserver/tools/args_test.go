package tools

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"
)

func specByName(t *testing.T, name string) *ToolSpec {
	t.Helper()
	for _, s := range AllSpecs() {
		if s.Name == name {
			if err := s.prepare(); err != nil {
				t.Fatal(err)
			}
			return s
		}
	}
	t.Fatalf("no spec %s", name)
	return nil
}

func argsOf(t *testing.T, name, input string) string {
	t.Helper()
	s := specByName(t, name)
	if _, err := validateInput(s, json.RawMessage(input)); err != nil {
		t.Fatalf("%s: input rejected: %v", name, err)
	}
	var out []json.RawMessage
	var err error
	if s.ArgsOverride != nil {
		out, err = s.ArgsOverride(json.RawMessage(input), wscompat.Identity{TenantID: "t", UserID: "u"})
	} else {
		out, err = defaultArgs(s.Fields, s.Consts, json.RawMessage(input))
	}
	if err != nil || len(out) != 1 {
		t.Fatalf("%s: %v %v", name, out, err)
	}
	return string(out[0])
}

// Golden Args mapping: wire keys are what the real handlers read (git.* uses
// "worktree" with an id: prefix, worktree.create uses repo/baseBranch).
func TestArgsMappingGolden(t *testing.T) {
	cases := []struct{ tool, in, want string }{
		{"git_status", `{"worktree_id":"w1"}`, `{"worktree":"id:w1"}`},
		{"task_create", `{"title":"T","parent_id":"p","project_id":"pr"}`, `{"parentId":"p","projectId":"pr","title":"T"}`},
		{"worktree_create", `{"repo_id":"r1","name":"feat","base_branch":"main"}`,
			`{"baseBranch":"main","captureSource":"mcp","name":"feat","origin":"mcp","repo":"r1"}`},
		{"files_readChunk", `{"worktree_id":"w","path":"a.go","offset_bytes":10,"length_bytes":20}`,
			`{"lengthBytes":20,"offsetBytes":10,"path":"a.go","worktreeId":"w"}`},
		{"git_commit", `{"worktree_id":"w","message":"m","paths":["a","b"]}`, `{"message":"m","paths":["a","b"],"worktree":"id:w"}`},
		{"annotation_create", `{"repo_id":"r","file_path":"f","line":3,"content":"c"}`,
			`{"anchor":{"filePath":"f","line":3,"repoId":"r"},"content":"c"}`},
		{"project_list", `{}`, `{}`},
	}
	for _, c := range cases {
		got := argsOf(t, c.tool, c.in)
		// Compare as normalized JSON (key order is the encoder's, but nested
		// anchor is built from a map too).
		var g, w any
		_ = json.Unmarshal([]byte(got), &g)
		_ = json.Unmarshal([]byte(c.want), &w)
		gb, _ := json.Marshal(g)
		wb, _ := json.Marshal(w)
		if string(gb) != string(wb) {
			t.Errorf("%s: got %s want %s", c.tool, gb, wb)
		}
	}
}

func TestInputRejectsUnknownAndIdentityFields(t *testing.T) {
	s := specByName(t, "task_create")
	for _, in := range []string{`{"title":"x","tenantId":"evil"}`, `{"title":"x","user_id":"u"}`, `{"title":""}`, `{}`, `[]`, `{"title":5}`} {
		if _, err := validateInput(s, json.RawMessage(in)); err == nil {
			t.Errorf("input %s must be rejected", in)
		}
	}
	if _, err := validateInput(s, json.RawMessage(`{"title":"ok"}`)); err != nil {
		t.Error(err)
	}
}

func TestEverySchemaIsValidAndIdentityFree(t *testing.T) {
	forbidden := []string{"tenantid", "userid", "tenant_id", "user_id"}
	secretish := []string{"credentialref", "apikey", "password", "secret"}
	for _, s := range AllSpecs() {
		if err := s.prepare(); err != nil {
			t.Fatalf("%s: %v", s.Name, err)
		}
		seen := map[string]bool{}
		for _, f := range s.Fields {
			low := strings.ToLower(f.Name)
			for _, bad := range forbidden {
				if low == bad {
					t.Errorf("%s: input field %s carries identity", s.Name, f.Name)
				}
			}
			for _, bad := range secretish {
				if strings.Contains(strings.ToLower(f.Name+f.Wire), bad) {
					t.Errorf("%s: input field %s looks like a secret", s.Name, f.Name)
				}
			}
			for k := range s.Consts {
				if strings.EqualFold(k, "userId") || strings.EqualFold(k, "tenantId") {
					t.Errorf("%s: const %s carries identity", s.Name, k)
				}
			}
			if seen[f.Name] {
				t.Errorf("%s: duplicate field %s", s.Name, f.Name)
			}
			seen[f.Name] = true
			// A wire userId is fine only as an explicitly named target.
			if (strings.ToLower(f.Wire) == "userid" && !strings.HasPrefix(f.Name, "member_") && !strings.HasPrefix(f.Name, "target_")) ||
				strings.ToLower(f.Wire) == "tenantid" {
				t.Errorf("%s: wire key %s carries identity", s.Name, f.Wire)
			}
		}
		if s.output == nil || s.input == nil {
			t.Errorf("%s: schemas not built", s.Name)
		}
	}
}

func TestJSONSchemaLibraryRecorded(t *testing.T) {
	// The schema library is google/jsonschema-go (the one go-sdk uses); the
	// version is pinned in go.mod and echoed here for the CI log.
	t.Log("jsonschema library: github.com/google/jsonschema-go v0.4.3")
}
