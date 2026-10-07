package wscompat

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

type testOptionalArgs struct {
	Filter string `json:"filter,omitempty"`
}

func (a testOptionalArgs) validate() error {
	return nil
}

type testCodeIntelArgs struct {
	codeIntelSelector
	Depth *int   `json:"depth,omitempty"`
	Path  string `json:"path,omitempty"`
	Ref   string `json:"ref,omitempty"`
	Enum  string `json:"enum,omitempty"`
}

func (a testCodeIntelArgs) validate() error {
	if err := a.codeIntelSelector.validate(); err != nil {
		return err
	}
	if err := checkBoundedInt("depth", a.Depth, 1, 3); err != nil {
		return err
	}
	if err := checkRelPath("path", a.Path); err != nil {
		return err
	}
	if err := checkGitRefLike("ref", a.Ref); err != nil {
		return err
	}
	if err := checkEnum("enum", a.Enum, "active", "archived"); err != nil {
		return err
	}
	return nil
}

func TestDecodeCodeIntelArgs_ShapeAndBasics(t *testing.T) {
	spec := codeIntelChannelSpec{
		Name:         "codeIntel.test",
		MaxArgsBytes: 1024,
	}

	t.Run("empty args session dialect", func(t *testing.T) {
		got, err := decodeCodeIntelArgs[testOptionalArgs](spec, []json.RawMessage{})
		if err != nil {
			t.Fatalf("expected success on empty args, got %v", err)
		}
		if got.Filter != "" {
			t.Errorf("expected empty filter, got %s", got.Filter)
		}
	})

	t.Run("null arg session dialect", func(t *testing.T) {
		got, err := decodeCodeIntelArgs[testOptionalArgs](spec, []json.RawMessage{json.RawMessage("null")})
		if err != nil {
			t.Fatalf("expected success on null arg, got %v", err)
		}
		if got.Filter != "" {
			t.Errorf("expected empty filter, got %s", got.Filter)
		}
	})

	t.Run("multiple args rejected", func(t *testing.T) {
		_, err := decodeCodeIntelArgs[testOptionalArgs](spec, []json.RawMessage{
			json.RawMessage(`{}`),
			json.RawMessage(`{}`),
		})
		if err == nil || !strings.Contains(err.Error(), "expected_single_params_object") {
			t.Fatalf("expected expected_single_params_object, got %v", err)
		}
	})

	t.Run("non-object args rejected", func(t *testing.T) {
		cases := [][]byte{
			[]byte(`"x"`),
			[]byte(`[1]`),
			[]byte(`123`),
		}
		for _, c := range cases {
			_, err := decodeCodeIntelArgs[testOptionalArgs](spec, []json.RawMessage{json.RawMessage(c)})
			if err == nil || !strings.Contains(err.Error(), "not_an_object") {
				t.Errorf("expected not_an_object for %s, got %v", string(c), err)
			}
		}
	})
}

func TestDecodeCodeIntelArgs_ForbiddenKeys(t *testing.T) {
	spec := codeIntelChannelSpec{MaxArgsBytes: 1024}

	forbiddenCases := []struct {
		jsonInput string
		badVal    string
		badKey    string
	}{
		{`{"projectId":"p","worktreeId":"w","tenantId":"tenant-123"}`, "tenant-123", "tenantId"},
		{`{"projectId":"p","worktreeId":"w","TenantId":"tenant-456"}`, "tenant-456", "TenantId"},
		{`{"projectId":"p","worktreeId":"w","workspaceRoot":"/secret/root"}`, "/secret/root", "workspaceRoot"},
		{`{"projectId":"p","worktreeId":"w","cypher":"MATCH (n) RETURN n"}`, "MATCH", "cypher"},
		{`{"projectId":"p","worktreeId":"w","repo":"my-repo"}`, "my-repo", "repo"},
		{`{"projectId":"p","worktreeId":"w","deviceId":"dev-999"}`, "dev-999", "deviceId"},
		{`{"projectId":"p","worktreeId":"w","command":"rm -rf"}`, "rm -rf", "command"},
	}

	for _, tc := range forbiddenCases {
		_, err := decodeCodeIntelArgs[testCodeIntelArgs](spec, []json.RawMessage{json.RawMessage(tc.jsonInput)})
		if err == nil {
			t.Fatalf("expected forbidden key error for %s", tc.jsonInput)
		}
		errStr := err.Error()
		if !strings.Contains(errStr, "not_allowed") {
			t.Errorf("expected not_allowed in %s", errStr)
		}
		if !strings.Contains(errStr, tc.badKey) {
			t.Errorf("expected badKey %s in %s", tc.badKey, errStr)
		}
		// Must NOT contain sensitive parameter value
		if strings.Contains(errStr, tc.badVal) {
			t.Errorf("error leaked parameter value %q: %s", tc.badVal, errStr)
		}
	}
}

func TestDecodeCodeIntelArgs_UnknownFieldAndWrongType(t *testing.T) {
	spec := codeIntelChannelSpec{MaxArgsBytes: 1024}

	t.Run("unknown field", func(t *testing.T) {
		_, err := decodeCodeIntelArgs[testCodeIntelArgs](spec, []json.RawMessage{
			json.RawMessage(`{"projectId":"p","worktreeId":"w","foo":1}`),
		})
		if err == nil || !strings.Contains(err.Error(), "unknown_field") || !strings.Contains(err.Error(), `"foo"`) {
			t.Fatalf("expected unknown_field for foo, got %v", err)
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		_, err := decodeCodeIntelArgs[testCodeIntelArgs](spec, []json.RawMessage{
			json.RawMessage(`{"projectId":"p","worktreeId":"w","depth":"2"}`),
		})
		if err == nil || !strings.Contains(err.Error(), "wrong_type") || !strings.Contains(err.Error(), `"depth"`) {
			t.Fatalf("expected wrong_type for depth, got %v", err)
		}
	})
}

func TestDecodeCodeIntelArgs_TrailingData(t *testing.T) {
	spec := codeIntelChannelSpec{MaxArgsBytes: 1024}

	_, err := decodeCodeIntelArgs[testCodeIntelArgs](spec, []json.RawMessage{
		json.RawMessage(`{"projectId":"p","worktreeId":"w"}{"extra":1}`),
	})
	if err == nil || !strings.Contains(err.Error(), "trailing_data") {
		t.Fatalf("expected trailing_data error, got %v", err)
	}
}

func TestDecodeCodeIntelArgs_SizeLimit(t *testing.T) {
	spec := codeIntelChannelSpec{MaxArgsBytes: 40}

	exact := `{"projectId":"p","worktreeId":"w"}` // 34 bytes
	_, err := decodeCodeIntelArgs[testCodeIntelArgs](spec, []json.RawMessage{json.RawMessage(exact)})
	if err != nil {
		t.Fatalf("expected success for 34 bytes <= 40 bytes, got %v", err)
	}

	overflow := `{"projectId":"p","worktreeId":"w","path":"src/a"}` // 49 bytes > 40 bytes
	_, err = decodeCodeIntelArgs[testCodeIntelArgs](spec, []json.RawMessage{json.RawMessage(overflow)})
	if err == nil || !strings.Contains(err.Error(), "too_large") {
		t.Fatalf("expected too_large error, got %v", err)
	}
}

func TestDecodeCodeIntelArgs_FieldValidations(t *testing.T) {
	spec := codeIntelChannelSpec{MaxArgsBytes: 1024}

	// 1. Depth bounded int [1..3]
	depthBadCases := []int{0, 4, -1}
	for _, d := range depthBadCases {
		payload, _ := json.Marshal(map[string]any{"projectId": "p", "worktreeId": "w", "depth": d})
		_, err := decodeCodeIntelArgs[testCodeIntelArgs](spec, []json.RawMessage{json.RawMessage(payload)})
		if err == nil || !strings.Contains(err.Error(), "out_of_range") {
			t.Errorf("expected out_of_range for depth %d, got %v", d, err)
		}
	}
	depthGoodCases := []int{1, 2, 3}
	for _, d := range depthGoodCases {
		payload, _ := json.Marshal(map[string]any{"projectId": "p", "worktreeId": "w", "depth": d})
		_, err := decodeCodeIntelArgs[testCodeIntelArgs](spec, []json.RawMessage{json.RawMessage(payload)})
		if err != nil {
			t.Errorf("expected valid for depth %d, got %v", d, err)
		}
	}

	// 2. Path traversal checkRelPath
	pathBadCases := []string{
		"../x",
		"/etc",
		"a\\b",
		"%2e%2e/x",
		"．．/x",
	}
	for _, p := range pathBadCases {
		payload, _ := json.Marshal(map[string]any{"projectId": "p", "worktreeId": "w", "path": p})
		_, err := decodeCodeIntelArgs[testCodeIntelArgs](spec, []json.RawMessage{json.RawMessage(payload)})
		if err == nil {
			t.Errorf("expected path error for %s", p)
			continue
		}
		errStr := err.Error()
		if !strings.HasPrefix(errStr, "CODEINTEL_PATH_NOT_ALLOWED") {
			t.Errorf("expected CODEINTEL_PATH_NOT_ALLOWED for %s, got %s", p, errStr)
		}
		// Confirm path value is not leaked
		if strings.Contains(errStr, p) {
			t.Errorf("path leaked in error message %q: %s", p, errStr)
		}
	}

	// 3. Git ref checkGitRefLike
	badRefs := []string{"-rf", "a..b", "a b"}
	for _, r := range badRefs {
		payload, _ := json.Marshal(map[string]any{"projectId": "p", "worktreeId": "w", "ref": r})
		_, err := decodeCodeIntelArgs[testCodeIntelArgs](spec, []json.RawMessage{json.RawMessage(payload)})
		if err == nil || !strings.Contains(err.Error(), "invalid_git_ref") {
			t.Errorf("expected invalid_git_ref for %s, got %v", r, err)
		}
	}
	goodRefs := []string{"origin/main", "HEAD~3", "feature/ünï"}
	for _, r := range goodRefs {
		payload, _ := json.Marshal(map[string]any{"projectId": "p", "worktreeId": "w", "ref": r})
		_, err := decodeCodeIntelArgs[testCodeIntelArgs](spec, []json.RawMessage{json.RawMessage(payload)})
		if err != nil {
			t.Errorf("expected valid ref for %s, got %v", r, err)
		}
	}
}

func TestDecodeCodeIntelArgs_ErrorFormatCompliance(t *testing.T) {
	spec := codeIntelChannelSpec{MaxArgsBytes: 1024}
	stdRegex := regexp.MustCompile(`^(CODEINTEL_[A-Z0-9_]+):\s*(.*?)(?:\s*\|\s*(\{.*\}))?$`)

	// Generate various errors and verify pattern
	errTests := []string{
		`{"projectId":"p","worktreeId":"w","unknownKey":1}`,
		`{"projectId":"p","worktreeId":"w","depth":99}`,
		`{"projectId":"p","worktreeId":"w","path":"../evil"}`,
	}

	for _, input := range errTests {
		_, err := decodeCodeIntelArgs[testCodeIntelArgs](spec, []json.RawMessage{json.RawMessage(input)})
		if err == nil {
			t.Fatalf("expected error for %s", input)
		}
		errStr := err.Error()
		match := stdRegex.FindStringSubmatch(errStr)
		if match == nil {
			t.Fatalf("error %q does not match UI-API standard pattern", errStr)
		}
		if match[3] != "" {
			var parsed map[string]any
			if err := json.Unmarshal([]byte(match[3]), &parsed); err != nil {
				t.Fatalf("json suffix is invalid: %s", match[3])
			}
		}
	}
}

func TestCodeIntelSelector_Validation(t *testing.T) {
	// ProjectID required
	s1 := codeIntelSelector{ProjectID: "", WorktreeID: "w"}
	if err := s1.validate(); err == nil || !strings.Contains(err.Error(), "required") {
		t.Errorf("expected required for projectId, got %v", err)
	}

	// WorktreeID required
	s2 := codeIntelSelector{ProjectID: "p", WorktreeID: ""}
	if err := s2.validate(); err == nil || !strings.Contains(err.Error(), "required") {
		t.Errorf("expected required for worktreeId, got %v", err)
	}

	// Control characters
	s3 := codeIntelSelector{ProjectID: "p\n", WorktreeID: "w"}
	if err := s3.validate(); err == nil || !strings.Contains(err.Error(), "control_characters_not_allowed") {
		t.Errorf("expected control characters error, got %v", err)
	}

	// Too long
	s4 := codeIntelSelector{ProjectID: strings.Repeat("x", 65), WorktreeID: "w"}
	if err := s4.validate(); err == nil || !strings.Contains(err.Error(), "too_long") {
		t.Errorf("expected too_long for projectId, got %v", err)
	}

	// Valid toProtoSelector
	s5 := codeIntelSelector{ProjectID: "proj-1", WorktreeID: "wt-2"}
	if err := s5.validate(); err != nil {
		t.Fatalf("unexpected error for s5: %v", err)
	}
	protoSel := toProtoSelector(s5)
	if protoSel.ProjectId != "proj-1" || protoSel.WorktreeRef != "wt-2" {
		t.Errorf("toProtoSelector mismatch: %+v", protoSel)
	}
}

func TestCodeIntelValidationHelpers(t *testing.T) {
	// checkRFC3339
	if err := checkRFC3339("ts", "2026-10-07T05:00:00Z"); err != nil {
		t.Errorf("valid RFC3339 rejected: %v", err)
	}
	if err := checkRFC3339("ts", "not-a-date"); err == nil {
		t.Errorf("invalid RFC3339 accepted")
	}

	// checkStringMax
	if err := checkStringMax("str", "abc", 2); err == nil {
		t.Errorf("checkStringMax should reject string > 2")
	}
	if err := checkStringMax("str", "ab", 2); err != nil {
		t.Errorf("checkStringMax should accept string <= 2: %v", err)
	}

	// checkPageToken
	if err := checkPageToken("pt", strings.Repeat("a", 513)); err == nil {
		t.Errorf("checkPageToken should reject > 512")
	}
	if err := checkPageToken("pt", strings.Repeat("a", 512)); err != nil {
		t.Errorf("checkPageToken should accept <= 512: %v", err)
	}

	// checkOpaqueID
	if err := checkOpaqueID("id", strings.Repeat("a", 129)); err == nil {
		t.Errorf("checkOpaqueID should reject > 128")
	}
	if err := checkOpaqueID("id", "bad\nid"); err == nil {
		t.Errorf("checkOpaqueID should reject control characters")
	}
}
