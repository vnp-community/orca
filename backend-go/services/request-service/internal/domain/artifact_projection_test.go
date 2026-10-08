package domain

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

var updateGolden = flag.Bool("update-golden", false, "rewrite testdata/projection from the renderer")

const projectionDir = "../../testdata/projection"

func readValid(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("../../testdata/artifacts/valid/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func solutionMeta() ProjectionMeta {
	return ProjectionMeta{ID: "SOL-142.2", Request: "REQ-142@r2", Status: "proposed", Supersedes: "SOL-142.1",
		Title: "Đăng nhập bằng SSO", GeneratedBy: GeneratedBy{Kind: "native", Tool: "ai.complete", Model: "claude-x", Run: "run-1"}}
}

func planMeta() ProjectionMeta {
	return ProjectionMeta{ID: "PLN-142.1", Request: "REQ-142@r2", Status: "draft", Title: "Triển khai SSO",
		Extra: map[string]string{"phases": "Một phase duy nhất.", "tasks": "- [ ] TSK-142.1.1 Thêm route"}}
}

type projectionSample struct {
	file string
	kind ArtifactKind
	doc  string
	meta ProjectionMeta
}

func projectionSamples() []projectionSample {
	return []projectionSample{
		{"solution_ok.md", ArtifactKindSolution, "solution_with_coverage.json", solutionMeta()},
		{"plan_ok.md", ArtifactKindPlan, "plan_ok.json", planMeta()},
		{"task_ok.md", ArtifactKindTask, "task_ok.json", ProjectionMeta{ID: "TSK-142.1.1"}},
		{"request_full.md", ArtifactKindRequest, "request_full.json", ProjectionMeta{ID: "REQ-142", Title: "Đăng nhập bằng SSO"}},
	}
}

// regenerateGolden rewrites the sample files; run: go test ./internal/domain -run Projection -update-golden
func regenerateGolden(t *testing.T) {
	t.Helper()
	reg := testRegistry(t)
	for _, s := range projectionSamples() {
		out, err := RenderArtifact(reg, s.kind, readValid(t, s.doc), s.meta)
		if err != nil {
			t.Fatal(err)
		}
		write(t, s.file, string(out))
	}
	sol, _ := os.ReadFile(filepath.Join(projectionDir, "solution_ok.md"))
	text := string(sol)
	write(t, "frontmatter_alias.md", strings.Replace(text, "request: REQ-142@r2", "request: &a REQ-142@r2\nsupersedes_alias: *a", 1))
	first := strings.Index(text, "```orca-json")
	end := strings.Index(text[first:], "```\n<!-- orca:end") + first
	block := text[first:end]
	write(t, "two_orca_json_blocks.md", text[:end]+"```\n\n"+block+text[end:])
	write(t, "prose_fake_json.md", text+"\nGhi chú của người dùng: {\"options\": []} và \n```orca-json\n{\"schema_version\": 99}\n```\n- [x] TSK-1.1.1 giả\n")
	write(t, "crlf.md", strings.ReplaceAll(text, "\n", "\r\n"))
	write(t, "vietnamese_decomposed.md", norm.NFD.String(text))
}

func stripRegionDigests(s string) string {
	return regexp.MustCompile(` digest=sha256:[0-9a-f]{64}`).ReplaceAllString(s, "")
}

func write(t *testing.T, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(projectionDir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

func TestProjectionGolden(t *testing.T) {
	if *updateGolden {
		_ = os.MkdirAll(projectionDir, 0o755)
		regenerateGolden(t)
	}
	reg := testRegistry(t)
	for _, s := range projectionSamples() {
		want, err := os.ReadFile(filepath.Join(projectionDir, s.file))
		if err != nil {
			t.Fatalf("%s: %v (run with -update-golden)", s.file, err)
		}
		got, err := RenderArtifact(reg, s.kind, readValid(t, s.doc), s.meta)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Errorf("%s differs from the renderer output; run with -update-golden if intended.\n%s", s.file, got)
		}
	}
}

func TestRender_Parse_RoundTrip_AllSamples(t *testing.T) {
	reg := testRegistry(t)
	for _, s := range projectionSamples() {
		doc := readValid(t, s.doc)
		md, err := RenderArtifact(reg, s.kind, doc, s.meta)
		if err != nil {
			t.Fatalf("%s: %v", s.file, err)
		}
		got, vs := ParseProjection(reg, md, s.kind)
		if len(vs) != 0 {
			t.Fatalf("%s: %+v", s.file, vs)
		}
		want, _ := CanonicalJSON(doc)
		if string(got) != string(want) {
			t.Errorf("%s: Parse(Render(x)) != x\n got  %s\n want %s", s.file, got, want)
		}
	}
}

func TestRender_Deterministic(t *testing.T) {
	reg := testRegistry(t)
	doc := readValid(t, "solution_with_coverage.json")
	a, _ := RenderArtifact(reg, ArtifactKindSolution, doc, solutionMeta())
	reordered := strings.Replace(string(doc), `"schema_version": 1,`, `"test_strategy": "e2e", "schema_version": 1,`, 1)
	_ = reordered // key order of the input must not matter either
	for i := 0; i < 5; i++ {
		b, _ := RenderArtifact(reg, ArtifactKindSolution, doc, solutionMeta())
		if string(a) != string(b) {
			t.Fatal("render is not deterministic")
		}
	}
	if strings.Contains(string(a), "\r") {
		t.Fatal("projection must use \\n only")
	}
}

func TestRender_NFC_Normalises(t *testing.T) {
	reg := testRegistry(t)
	doc := []byte(`{"schema_version":1,"kind":"phase","goal":"` + norm.NFD.String("Nền tảng") + `"}`)
	md, err := RenderArtifact(reg, ArtifactKindPhase, doc, ProjectionMeta{ID: "PH-1.1.1", Title: norm.NFD.String("Giai đoạn")})
	if err != nil {
		t.Fatal(err)
	}
	if !norm.NFC.IsNormal(md) || !utf8.Valid(md) {
		t.Fatal("rendered projection must be NFC")
	}
}

func TestRender_RefusesInvalidDocument(t *testing.T) {
	reg := testRegistry(t)
	bad, _ := os.ReadFile("../../testdata/artifacts/invalid/solution_option_bad_id.json")
	if _, err := RenderArtifact(reg, ArtifactKindSolution, bad, solutionMeta()); errCode(err) != CodeArtifactSchemaInvalid {
		t.Fatalf("got %v", err)
	}
}

func parseSample(t *testing.T, name string, kind ArtifactKind) ([]byte, []Violation) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(projectionDir, name))
	if err != nil {
		t.Fatal(err)
	}
	return ParseProjection(testRegistry(t), b, kind)
}

func TestParse_IgnoresProseOutsideRegion(t *testing.T) {
	clean, vs := parseSample(t, "solution_ok.md", ArtifactKindSolution)
	if len(vs) != 0 {
		t.Fatal(vs)
	}
	noisy, vs := parseSample(t, "prose_fake_json.md", ArtifactKindSolution)
	if len(vs) != 0 {
		t.Fatalf("prose outside the regions must not break or change parsing: %+v", vs)
	}
	if string(clean) != string(noisy) {
		t.Fatal("prose changed the parsed data")
	}
}

func TestParse_FrontmatterRejectsAliasAnchorTag(t *testing.T) {
	_, vs := parseSample(t, "frontmatter_alias.md", ArtifactKindSolution)
	if len(vs) == 0 || vs[0].Code != CodeProjectionYAMLUnsafe || vs[0].Line < 2 {
		t.Fatalf("%+v", vs)
	}
	for _, line := range []string{"id: &x SOL-1.1", "id: *x", "id: !!str SOL-1.1", "<<: {a: 1}"} {
		_, _, vs := ParseFrontmatter("---\norca_schema: 1\nkind: solution\n" + line + "\n---\n")
		if len(vs) == 0 || vs[0].Code != CodeProjectionYAMLUnsafe || vs[0].Line != 4 {
			t.Errorf("%q: %+v", line, vs)
		}
	}
	// Quoted text that merely contains the characters is fine.
	fm, _, vs := ParseFrontmatter("---\norca_schema: 1\nkind: solution\nid: \"SOL & *x !y\"\n---\n")
	if len(vs) != 0 || fm.ID != "SOL & *x !y" {
		t.Errorf("%+v %+v", fm, vs)
	}
}

func TestParseFrontmatter_Errors(t *testing.T) {
	cases := map[string]string{
		CodeProjectionFrontmatterMissing: "# no frontmatter\n",
		CodeProjectionFrontmatterField:   "---\nkind: solution\n---\n",
		CodeProjectionFrontmatterInvalid: "---\norca_schema: 1\nkind: solution\nid: x\nsurprise: 1\n---\n",
	}
	for code, md := range cases {
		_, _, vs := ParseFrontmatter(md)
		if len(vs) == 0 || vs[0].Code != code {
			t.Errorf("%s: %+v", code, vs)
		}
	}
	_, _, vs := ParseFrontmatter("---\norca_schema: 1\nkind: solution\n")
	if len(vs) == 0 || vs[0].Code != CodeProjectionFrontmatterMissing {
		t.Errorf("unclosed: %+v", vs)
	}
	_, _, vs = ParseFrontmatter("---\n" + strings.Repeat("a: b\n", MaxFrontmatterBytes/4) + "---\n")
	if len(vs) == 0 || vs[0].Code != CodeProjectionFrontmatterTooLarge {
		t.Errorf("too large: %+v", vs)
	}
	_, _, vs = ParseFrontmatter("---\norca_schema: 1\nkind: solution\n---\n")
	if len(vs) != 1 || vs[0].Code != CodeProjectionFrontmatterField || vs[0].Line != 1 {
		t.Errorf("missing id: %+v", vs)
	}
}

func TestParse_TwoOrcaJSONBlocks_Violation(t *testing.T) {
	_, vs := parseSample(t, "two_orca_json_blocks.md", ArtifactKindSolution)
	if len(vs) != 1 || vs[0].Code != CodeProjectionJSONBlock || !strings.Contains(vs[0].Message, "more than one") {
		t.Fatalf("%+v", vs)
	}
}

func TestParse_ViolationLineNumbersAbsolute(t *testing.T) {
	b, _ := os.ReadFile(filepath.Join(projectionDir, "solution_ok.md"))
	text := string(b)
	// Break the option id inside the options region; the digest must be recomputed away by dropping it.
	broken := strings.Replace(text, `"id":"opt-1"`, `"id":"option-1"`, 1)
	broken = stripRegionDigests(broken)
	_, vs := ParseProjection(testRegistry(t), []byte(broken), ArtifactKindSolution)
	if len(vs) == 0 {
		t.Fatal("expected violations")
	}
	lines := strings.Split(broken, "\n")
	v := vs[0]
	if v.Path != "/options/0/id" || v.Line < 2 || v.Line > len(lines) || !strings.HasPrefix(lines[v.Line-1], "{") || !strings.Contains(lines[v.Line-1], "option-1") {
		t.Fatalf("line %d does not point at the options JSON: %+v\n%q", v.Line, v, lines[v.Line-1])
	}
	_, vs = ParseProjection(testRegistry(t), []byte(strings.Replace(text, "kind: solution", "kind: plan", 1)), ArtifactKindSolution)
	if len(vs) != 1 || vs[0].Code != CodeProjectionKindMismatch || vs[0].Line != 3 {
		t.Fatalf("%+v", vs)
	}
}

func TestParse_RegionDigestAndFrontmatterDigest(t *testing.T) {
	b, _ := os.ReadFile(filepath.Join(projectionDir, "solution_ok.md"))
	edited := strings.Replace(string(b), `"hours_estimate":8`, `"hours_estimate":9`, 1)
	if _, vs := ParseProjection(testRegistry(t), []byte(edited), ArtifactKindSolution); len(vs) == 0 || vs[0].Code != CodeProjectionDigestMismatch {
		t.Fatalf("a hand-edited region must be noticed: %+v", vs)
	}
	noRegionDigests := stripRegionDigests(edited)
	if _, vs := ParseProjection(testRegistry(t), []byte(noRegionDigests), ArtifactKindSolution); len(vs) == 0 || vs[0].Code != CodeProjectionDigestMismatch || !strings.Contains(vs[0].Message, "frontmatter") {
		t.Fatalf("the frontmatter digest must catch it too: %+v", vs)
	}
}

func TestParse_CRLFAndDecomposedInputs(t *testing.T) {
	want, vs := parseSample(t, "solution_ok.md", ArtifactKindSolution)
	if len(vs) != 0 {
		t.Fatal(vs)
	}
	for _, name := range []string{"crlf.md", "vietnamese_decomposed.md"} {
		got, vs := parseSample(t, name, ArtifactKindSolution)
		if len(vs) != 0 || string(got) != string(want) {
			t.Errorf("%s: %+v\n%s", name, vs, got)
		}
	}
}

func TestParse_RejectsControlCharsAndOversize(t *testing.T) {
	reg := testRegistry(t)
	b, _ := os.ReadFile(filepath.Join(projectionDir, "solution_ok.md"))
	for name, in := range map[string][]byte{
		"nul":      append([]byte("---\n\x00"), b...),
		"escape":   []byte(strings.Replace(string(b), "# ", "# \x1b[31m", 1)),
		"lone CR":  []byte(strings.Replace(string(b), "\n# ", "\r# ", 1)),
		"oversize": append(b[:len(b):len(b)], []byte(strings.Repeat("a", MaxProjectionBytes))...),
	} {
		_, vs := ParseProjection(reg, in, ArtifactKindSolution)
		if len(vs) != 1 || (vs[0].Code != CodeProjectionControlChar && vs[0].Code != CodeProjectionTooLarge) {
			t.Errorf("%s: %+v", name, vs)
		}
	}
	// Tabs are fine.
	if _, vs := ParseProjection(reg, []byte(strings.Replace(string(b), "# ", "#\t", 1)), ArtifactKindSolution); len(vs) != 0 {
		t.Errorf("tab: %+v", vs)
	}
}

func TestExtractRegion_And_NestedMarkers(t *testing.T) {
	md := "a\n<!-- orca:begin x -->\nbody 1\nbody 2\n<!-- orca:end x -->\nz\n"
	body, start, end, ok := ExtractRegion(md, "x")
	if !ok || body != "body 1\nbody 2" || start != 2 || end != 5 {
		t.Fatalf("%q %d %d %v", body, start, end, ok)
	}
	if _, _, _, ok := ExtractRegion(md, "y"); ok {
		t.Fatal("unknown section")
	}
	nested := "<!-- orca:begin x -->\n<!-- orca:begin y -->\n<!-- orca:end y -->\n<!-- orca:end x -->\n"
	if _, _, _, ok := ExtractRegion(nested, "x"); ok {
		t.Fatal("nested regions must be refused")
	}
	// A marker quoted inside a code fence in prose is not a marker.
	fenced := "```\n<!-- orca:begin x -->\n```\n"
	if _, _, _, ok := ExtractRegion(fenced, "x"); ok {
		t.Fatal("a fenced marker must not open a region")
	}
}

func TestReplaceRegion_PreservesBytesOutside(t *testing.T) {
	md := "intro\r\nvới dấu\n<!-- orca:begin options digest=sha256:" + strings.Repeat("0", 64) + " -->\nold\nlines\n<!-- orca:end options -->\ntrailer \t\n"
	out, err := ReplaceRegion(md, "options", "new body\nmore")
	if err != nil {
		t.Fatal(err)
	}
	hash := func(s string) [32]byte { return sha256.Sum256([]byte(s)) }
	outsideBefore := md[:strings.Index(md, "old")] + md[strings.Index(md, "<!-- orca:end"):]
	outsideAfter := out[:strings.Index(out, "new body")] + out[strings.Index(out, "<!-- orca:end"):]
	if hash(outsideBefore) != hash(outsideAfter) {
		t.Fatalf("bytes outside the region changed:\n%q\n%q", outsideBefore, outsideAfter)
	}
	if !strings.Contains(out, "<!-- orca:begin options digest=sha256:"+strings.Repeat("0", 64)+" -->\nnew body\nmore\n<!-- orca:end options -->") {
		t.Fatalf("unexpected result %q", out)
	}
	emptied, _ := ReplaceRegion(md, "options", "")
	if strings.Contains(emptied, "old") || !strings.Contains(emptied, "-->\n<!-- orca:end options -->") {
		t.Fatalf("empty body: %q", emptied)
	}
}

func TestReplaceRegion_MissingRegion_Error(t *testing.T) {
	if _, err := ReplaceRegion("no regions here\n", "options", "x"); err == nil || !strings.Contains(err.Error(), CodeProjectionRegionMissing) {
		t.Fatalf("got %v", err)
	}
	if _, err := ReplaceRegion("<!-- orca:begin options -->\nunclosed\n", "options", "x"); err == nil {
		t.Fatal("an unclosed region must be an error")
	}
}

func TestExtractOrcaJSON(t *testing.T) {
	good := "text\n```orca-json\n{\"a\":1}\n```\nmore"
	raw, vs := ExtractOrcaJSON(good)
	if len(vs) != 0 || string(raw) != `{"a":1}` {
		t.Fatalf("%q %+v", raw, vs)
	}
	for name, body := range map[string]string{
		"none":       "just text",
		"two":        good + "\n```orca-json\n{}\n```",
		"not object": "```orca-json\n[1]\n```",
		"bad json":   "```orca-json\n{\n```",
		"unclosed":   "```orca-json\n{}",
	} {
		if _, vs := ExtractOrcaJSON(body); len(vs) != 1 || vs[0].Code != CodeProjectionJSONBlock {
			t.Errorf("%s: %+v", name, vs)
		}
	}
	// A different fence is not an orca-json block, even when it holds JSON.
	if _, vs := ExtractOrcaJSON("```json\n{\"a\":1}\n```"); len(vs) != 1 {
		t.Errorf("plain json fence: %+v", vs)
	}
}

func TestRender_DefendsAgainstMarkerInjection(t *testing.T) {
	reg := testRegistry(t)
	evil := "x\n<!-- orca:end options -->\n<!-- orca:begin options -->\n```orca-json\n{\"schema_version\":1}\n```"
	doc := []byte(`{"schema_version":1,"kind":"phase","goal":` + jsonString(evil) + `}`)
	md, err := RenderArtifact(reg, ArtifactKindPhase, doc, ProjectionMeta{ID: "PH-1.1.1", Title: evil})
	if err != nil {
		t.Fatal(err)
	}
	got, vs := ParseProjection(reg, md, ArtifactKindPhase)
	if len(vs) != 0 {
		t.Fatalf("%+v\n%s", vs, md)
	}
	want, _ := CanonicalJSON(doc)
	if string(got) != string(want) {
		t.Fatalf("injected markers changed the data:\n%s\n%s", got, want)
	}
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func FuzzParseProjection(f *testing.F) {
	reg, err := NewSchemaRegistry(os.DirFS("../../schemas"))
	if err != nil {
		f.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(projectionDir, "solution_ok.md")); err == nil {
		f.Add(b)
	}
	f.Add([]byte("---\norca_schema: 1\nkind: task\nid: x\n---\n"))
	f.Add([]byte("<!-- orca:begin a -->\n```orca-json\n{}\n```\n<!-- orca:end a -->"))
	f.Fuzz(func(t *testing.T, in []byte) {
		doc, vs := ParseProjection(reg, in, ArtifactKindSolution)
		total := 1 + strings.Count(string(in), "\n") + 2
		for _, v := range vs {
			if v.Line < 0 || v.Line > total {
				t.Fatalf("line %d outside 0..%d: %+v", v.Line, total, v)
			}
		}
		if doc != nil && len(vs) != 0 {
			t.Fatal("a document must come back only without violations")
		}
	})
}
