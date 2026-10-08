package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func testRegistry(t *testing.T) *SchemaRegistry {
	t.Helper()
	r, err := NewSchemaRegistry(os.DirFS("../../schemas"))
	if err != nil {
		t.Fatalf("compile schemas: %v", err)
	}
	return r
}

func kindOfSample(name string) ArtifactKind {
	return ArtifactKind(strings.SplitN(filepath.Base(name), "_", 2)[0])
}

func TestSchemaRegistry_ValidSamples_AllKinds(t *testing.T) {
	r := testRegistry(t)
	files, _ := filepath.Glob("../../testdata/artifacts/valid/*.json")
	seen := map[ArtifactKind]bool{}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		kind := kindOfSample(f)
		seen[kind] = true
		if vs := r.ValidateDocument(kind, raw); len(vs) != 0 {
			t.Errorf("%s: unexpected violations %+v", f, vs)
		}
	}
	for _, k := range AllArtifactKinds() {
		if !seen[k] {
			t.Errorf("no valid sample for kind %s", k)
		}
	}
}

func TestSchemaRegistry_InvalidSamples_ReturnPointerAndCode(t *testing.T) {
	r := testRegistry(t)
	cases := map[string]string{
		"request_missing_title":                "",
		"request_ac_bad_id":                    "/acceptance_criteria/0/id",
		"request_ac_text_too_long":             "/acceptance_criteria/0/text",
		"request_ac_duplicate_id":              "/acceptance_criteria/1/id",
		"request_unknown_type":                 "/type",
		"solution_option_bad_id":               "/options/0/id",
		"solution_duplicate_option_id":         "/options/1/id",
		"solution_title_too_long":              "/options/0/title",
		"solution_out_of_scope_without_note":   "/requirement_coverage/1",
		"solution_open_question_legacy_string": "/open_questions/0",
		"solution_too_many_options":            "/options",
		"task_bad_check_kind":                  "/checks/0/kind",
		"task_satisfies_bad":                   "/satisfies/0",
		"plan_missing_implements":              "",
		"diagnosis_confidence_range":           "/root_cause/confidence",
		"findings_missing_question":            "",
		"answer_too_long":                      "/answer_markdown",
		"phase_unknown_field":                  "",
	}
	files, _ := filepath.Glob("../../testdata/artifacts/invalid/*.json")
	if len(files) != len(cases) {
		t.Fatalf("%d invalid samples on disk, %d expectations", len(files), len(cases))
	}
	for name, wantPath := range cases {
		raw, err := os.ReadFile("../../testdata/artifacts/invalid/" + name + ".json")
		if err != nil {
			t.Fatal(err)
		}
		vs := r.ValidateDocument(kindOfSample(name), raw)
		if len(vs) == 0 {
			t.Errorf("%s: expected violations", name)
			continue
		}
		found := false
		for _, v := range vs {
			if v.Code != CodeArtifactSchemaInvalid {
				t.Errorf("%s: code = %s", name, v.Code)
			}
			if v.Path == wantPath {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: no violation at %q, got %+v", name, wantPath, vs)
		}
	}
}

func TestValidate_ReportsAllErrors_NotFirstOnly(t *testing.T) {
	r := testRegistry(t)
	doc := `{"schema_version":1,"title":"","body":"","type":"epic","acceptance_criteria":[{"id":"x","text":"","status":"zzz"}],"type_fields":{}}`
	vs := r.ValidateDocument(ArtifactKindRequest, []byte(doc))
	paths := map[string]bool{}
	for _, v := range vs {
		paths[v.Path] = true
	}
	for _, want := range []string{"/title", "/type", "/acceptance_criteria/0/id", "/acceptance_criteria/0/text", "/acceptance_criteria/0/status"} {
		if !paths[want] {
			t.Errorf("missing violation at %s in %+v", want, vs)
		}
	}
}

func TestValidate_Deterministic(t *testing.T) {
	r := testRegistry(t)
	doc := []byte(`{"schema_version":1,"title":"","body":1,"acceptance_criteria":"no","type_fields":[]}`)
	first := r.ValidateDocument(ArtifactKindRequest, doc)
	for i := 0; i < 5; i++ {
		if !reflect.DeepEqual(first, r.ValidateDocument(ArtifactKindRequest, doc)) {
			t.Fatal("violation order changed between runs")
		}
	}
}

func TestValidate_CapsAt50Violations(t *testing.T) {
	r := testRegistry(t)
	var items []string
	for i := 0; i < 120; i++ {
		items = append(items, `{"id":"bad","text":"","status":"zzz"}`)
	}
	// 50 items is the schema cap; the rest is extra noise on purpose, all still reported up to the limit.
	doc := fmt.Sprintf(`{"schema_version":1,"title":"t","body":"","acceptance_criteria":[%s],"type_fields":{}}`, strings.Join(items, ","))
	vs := r.ValidateDocument(ArtifactKindRequest, []byte(doc))
	if len(vs) != MaxViolations {
		t.Fatalf("got %d violations, want exactly %d", len(vs), MaxViolations)
	}
}

func TestValidate_VersionUnsupported(t *testing.T) {
	r := testRegistry(t)
	vs := r.Validate(ArtifactKindTask, 2, []byte(`{"schema_version":2,"objective":"x"}`))
	if len(vs) != 1 || vs[0].Code != CodeArtifactSchemaVersionUnsupported {
		t.Fatalf("got %+v", vs)
	}
}

func TestValidate_TooLarge(t *testing.T) {
	r := testRegistry(t)
	big := fmt.Sprintf(`{"schema_version":1,"objective":%q}`, strings.Repeat("a", MaxArtifactBytes(ArtifactKindTask)))
	vs := r.ValidateDocument(ArtifactKindTask, []byte(big))
	if len(vs) != 1 || vs[0].Code != CodeArtifactLimitExceeded {
		t.Fatalf("got %+v", vs)
	}
}

func TestValidate_SyntaxErrorHasLine(t *testing.T) {
	r := testRegistry(t)
	vs := r.Validate(ArtifactKindTask, 1, []byte("{\n\"schema_version\": 1,\n\"objective\": ,\n}"))
	if len(vs) != 1 || vs[0].Line < 2 {
		t.Fatalf("got %+v", vs)
	}
}

func TestUpgrade_ChainV0ToV1_Pure(t *testing.T) {
	r := testRegistry(t)
	r.RegisterUpgrader(ArtifactKindPhase, 0, func(raw []byte) ([]byte, error) {
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, err
		}
		m["goal"] = m["title"]
		delete(m, "title")
		m["schema_version"] = 1
		return json.Marshal(m)
	})
	in := []byte(`{"schema_version":0,"title":"Nền tảng"}`)
	keep := string(in)
	a, err := r.Upgrade(ArtifactKindPhase, 0, in)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := r.Upgrade(ArtifactKindPhase, 0, in)
	if string(a) != string(b) || string(in) != keep {
		t.Fatal("upgrade must be pure")
	}
	if vs := r.ValidateDocument(ArtifactKindPhase, a); len(vs) != 0 {
		t.Fatalf("upgraded doc invalid: %+v", vs)
	}
	if _, err := r.Upgrade(ArtifactKindTask, 0, in); err == nil {
		t.Fatal("missing upgrader must be an error")
	}
	if _, err := r.Upgrade(ArtifactKindTask, 5, in); err == nil {
		t.Fatal("future version must be an error")
	}
}

func TestProvenance_RejectsSecretFields(t *testing.T) {
	ok := `{"generator":{"kind":"native","tool":"ai.complete","model":"m","model_source":"agent_response"},"prompt":{"template":"solution_prompt","version":"v1"},"actor":{"id":"u","kind":"ai"},"generated_at":"2026-10-07T00:00:00Z"}`
	if _, err := ParseProvenance([]byte(ok)); err != nil {
		t.Fatalf("valid provenance rejected: %v", err)
	}
	for _, field := range []string{`"credential_ref":"cred-1"`, `"api_key":"sk-x"`, `"env":{"A":"b"}`} {
		bad := strings.Replace(ok, `{"generator"`, `{`+field+`,"generator"`, 1)
		if _, err := ParseProvenance([]byte(bad)); err == nil {
			t.Errorf("provenance with %s must be rejected", field)
		}
	}
	rt := reflect.TypeOf(Provenance{})
	var walk func(rt reflect.Type)
	walk = func(rt reflect.Type) {
		for i := 0; i < rt.NumField(); i++ {
			f := rt.Field(i)
			n := strings.ToLower(f.Name)
			if strings.Contains(n, "credential") || strings.Contains(n, "key") || n == "env" || strings.Contains(n, "secret") {
				t.Errorf("Provenance carries forbidden field %s", f.Name)
			}
			if f.Type.Kind() == reflect.Struct && f.Type != reflect.TypeOf(time.Time{}) {
				walk(f.Type)
			}
		}
	}
	walk(rt)
}

func TestComputeProvenanceInputDigest_ChangesWithPromptVersion(t *testing.T) {
	base := InputDigestParts{RequestSnapshot: map[string]any{"title": "Nguyễn"}, PriorArtifactDigests: []string{"a"}, PromptVersion: "v1", ProjectContextDigest: "c"}
	d1, err := ComputeProvenanceInputDigest(base)
	if err != nil {
		t.Fatal(err)
	}
	again, _ := ComputeProvenanceInputDigest(base)
	if d1 != again || !strings.HasPrefix(d1, "sha256:") {
		t.Fatalf("digest unstable or malformed: %s", d1)
	}
	changed := base
	changed.PromptVersion = "v2"
	d2, _ := ComputeProvenanceInputDigest(changed)
	if d1 == d2 {
		t.Fatal("prompt version must change the digest")
	}
}

func BenchmarkValidatePlan(b *testing.B) {
	r, err := NewSchemaRegistry(os.DirFS("../../schemas"))
	if err != nil {
		b.Fatal(err)
	}
	raw, _ := os.ReadFile("../../testdata/artifacts/valid/plan_ok.json")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.ValidateDocument(ArtifactKindPlan, raw)
	}
}

type noNetwork struct{ t *testing.T }

func (n noNetwork) RoundTrip(r *http.Request) (*http.Response, error) {
	n.t.Errorf("schema compilation tried to reach the network: %s", r.URL)
	return nil, errors.New("no network in tests")
}

// Compilation must resolve every $ref from the embedded files: a $id on https://orca.local is a name, not an address.
func TestSchemaRegistry_NoNetworkNeededToCompile(t *testing.T) {
	prev := http.DefaultTransport
	http.DefaultTransport = noNetwork{t}
	defer func() { http.DefaultTransport = prev }()
	if _, err := NewSchemaRegistry(os.DirFS("../../schemas")); err != nil {
		t.Fatal(err)
	}
}

func TestSchemaRegistry_MissingSchemaFileIsAnError(t *testing.T) {
	if _, err := NewSchemaRegistry(fstest.MapFS{}); err == nil {
		t.Fatal("an incomplete schema directory must fail at startup, not at first use")
	}
}
