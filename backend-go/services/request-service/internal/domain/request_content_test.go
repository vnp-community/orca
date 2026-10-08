package domain

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func contentWith(t RequestType, tf map[string]any) RequestContent {
	c := RequestContent{Title: "Tiêu đề", Body: "Mô tả đủ dài để vượt ngưỡng hai mươi ký tự.", Type: t, TypeFields: tf}
	_, _ = c.AcceptanceCriteria.Add("Tiêu chí kiểm chứng được", "test")
	return c
}

func violationPaths(vs []Violation) []string {
	var out []string
	for _, v := range vs {
		out = append(out, v.Path)
	}
	sort.Strings(out)
	return out
}

func TestRequiredFields_CoversAllElevenTypes(t *testing.T) {
	for _, rt := range AllRequestTypes() {
		if _, ok := requiredFieldsByType[rt]; !ok {
			t.Errorf("type %s has no row in requiredFieldsByType", rt)
		}
	}
	if len(requiredFieldsByType) != len(AllRequestTypes()) {
		t.Errorf("table has %d rows for %d types", len(requiredFieldsByType), len(AllRequestTypes()))
	}
	if len(RequiredFields(RequestTypeTask)) != 0 {
		t.Error("task has no type-specific keys")
	}
}

func TestValidate_Draft_OnlyTitleAndBodyOK(t *testing.T) {
	c := RequestContent{Title: "Chỉ có tiêu đề", Body: ""}
	for _, rt := range append(AllRequestTypes(), "") {
		if vs := ValidateRequestContent(rt, c, ValidationLevelDraft); len(vs) != 0 {
			t.Errorf("draft %q: %+v", rt, vs)
		}
	}
	if vs := ValidateRequestContent("", RequestContent{}, ValidationLevelDraft); len(vs) != 1 || vs[0].Path != "/title" {
		t.Errorf("an empty title must be refused: %+v", vs)
	}
}

func TestValidate_Draft_ChecksShapeOfPresentFields(t *testing.T) {
	c := RequestContent{Title: "t", Type: RequestTypeBug, TypeFields: map[string]any{"severity": "meh", "repro_steps": "not a list"}}
	paths := violationPaths(ValidateRequestContent(RequestTypeBug, c, ValidationLevelDraft))
	if !reflect.DeepEqual(paths, []string{"/type_fields/repro_steps", "/type_fields/severity"}) {
		t.Fatalf("got %v", paths)
	}
}

func TestValidate_Ready_MissingKeysPerType(t *testing.T) {
	for _, rt := range AllRequestTypes() {
		c := contentWith(rt, map[string]any{})
		var want []string
		for _, r := range RequiredFields(rt) {
			want = append(want, r.Path())
		}
		sort.Strings(want)
		got := violationPaths(ValidateRequestContent(rt, c, ValidationLevelReady))
		if len(want) == 0 {
			want = nil
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %v want %v", rt, got, want)
		}
	}
}

func fullTypeFields(rt RequestType) map[string]any {
	tf := map[string]any{}
	for _, r := range RequiredFields(rt) {
		switch r.Kind {
		case FieldList:
			tf[r.Key] = []any{"một mục"}
		case FieldEnum:
			tf[r.Key] = r.Enum[0]
		case FieldBool:
			tf[r.Key] = false
		case FieldNumber:
			tf[r.Key] = 4.0
		case FieldScalar:
			tf[r.Key] = 3.5
		default:
			tf[r.Key] = "có giá trị"
		}
	}
	return tf
}

func TestValidate_Ready_CompleteContentPassesForEveryType(t *testing.T) {
	for _, rt := range AllRequestTypes() {
		if vs := ValidateRequestContent(rt, contentWith(rt, fullTypeFields(rt)), ValidationLevelReady); len(vs) != 0 {
			t.Errorf("%s: %+v", rt, vs)
		}
	}
}

func TestValidate_Ready_BodyAndACRules(t *testing.T) {
	c := contentWith(RequestTypeTask, nil)
	c.Body = "ngắn   \n\t "
	c.AcceptanceCriteria = AcceptanceCriteria{}
	paths := violationPaths(ValidateRequestContent(RequestTypeTask, c, ValidationLevelReady))
	if !reflect.DeepEqual(paths, []string{"/acceptance_criteria", "/body"}) {
		t.Fatalf("got %v", paths)
	}
	c = contentWith(RequestTypeTask, nil)
	_ = c.AcceptanceCriteria.Retire("AC-1")
	if vs := ValidateRequestContent(RequestTypeTask, c, ValidationLevelReady); len(vs) != 1 || vs[0].Code != CodeContentACRequired {
		t.Fatalf("only retired ACs must not count: %+v", vs)
	}
	if vs := ValidateRequestContent("", contentWith("", nil), ValidationLevelReady); len(vs) != 1 || vs[0].Code != CodeContentTypeRequired {
		t.Fatalf("ready needs a type: %+v", vs)
	}
}

func TestValidate_Ready_BlankStringAndEmptyListAreMissing(t *testing.T) {
	tf := fullTypeFields(RequestTypeBug)
	tf["actual"] = "   "
	tf["repro_steps"] = []any{"", "  "}
	tf["severity"] = ""
	paths := violationPaths(ValidateRequestContent(RequestTypeBug, contentWith(RequestTypeBug, tf), ValidationLevelReady))
	want := []string{"/type_fields/actual", "/type_fields/repro_steps", "/type_fields/severity"}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("got %v", paths)
	}
}

func TestParseRequestContent_RoundTripAndCounter(t *testing.T) {
	c := contentWith(RequestTypeBug, map[string]any{"actual": "x"})
	_, _ = c.AcceptanceCriteria.Add("hai", "")
	_ = c.AcceptanceCriteria.Retire("AC-2")
	back, err := ParseRequestContent(c.Title, c.Body, c.Type, c.AcceptanceCriteriaJSON(), c.TypeFieldsJSON())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back, c) {
		t.Fatalf("round trip differs:\n%+v\n%+v", back, c)
	}
	if back.AcceptanceCriteria.Next != 3 {
		t.Fatalf("counter lost: %d", back.AcceptanceCriteria.Next)
	}
	// A column whose ac_next is stale must not reissue AC-2.
	stale, err := ParseRequestContent("t", "", "", c.AcceptanceCriteriaJSON(), []byte(`{"ac_next":1}`))
	if err != nil || stale.AcceptanceCriteria.Next != 3 {
		t.Fatalf("stale counter: %d %v", stale.AcceptanceCriteria.Next, err)
	}
	if _, err := ParseRequestContent("t", "", "", nil, []byte(`{"ac_next":"x"}`)); err == nil {
		t.Fatal("non-numeric ac_next must fail")
	}
}

func TestRequestContent_SnapshotIsCanonicalAndDigestIgnoresMeta(t *testing.T) {
	c := contentWith(RequestTypeBug, map[string]any{"b": 1, "a": 2})
	snap, err := c.Snapshot(nil)
	if err != nil {
		t.Fatal(err)
	}
	again, _ := CanonicalJSON(snap)
	if string(snap) != string(again) {
		t.Fatal("snapshot must already be canonical")
	}
	if !strings.Contains(string(snap), `"type_fields":{"a":2,"ac_next":2,"b":1}`) {
		t.Fatalf("unexpected snapshot %s", snap)
	}
	d1, _ := c.Digest()
	withMeta, _ := c.Snapshot(map[string]any{"waiver": "x"})
	if DigestOfCanonical(withMeta) == d1 {
		t.Fatal("meta must show up in the snapshot")
	}
	var probe map[string]any
	_ = json.Unmarshal(withMeta, &probe)
	if probe["meta"] == nil {
		t.Fatal("snapshot lost meta")
	}
}

func TestRequestContent_Snapshot_ValidatesAgainstRequestSchema(t *testing.T) {
	r := testRegistry(t)
	c := contentWith(RequestTypeBug, fullTypeFields(RequestTypeBug))
	snap, _ := c.Snapshot(map[string]any{"waiver": map[string]any{"reason": "x"}})
	if vs := r.ValidateDocument(ArtifactKindRequest, snap); len(vs) != 0 {
		t.Fatalf("%+v\n%s", vs, snap)
	}
	empty := RequestContent{Title: "t"}
	snap, _ = empty.Snapshot(nil)
	if vs := r.ValidateDocument(ArtifactKindRequest, snap); len(vs) != 0 {
		t.Fatalf("%+v", vs)
	}
}

func TestRequestContent_Apply(t *testing.T) {
	c := contentWith(RequestTypeBug, map[string]any{"actual": "a", "environment": "prod"})
	title := "  Tiêu đề mới "
	acs := []ACInput{{ID: "AC-1", Text: "đã sửa"}, {Text: "thêm"}}
	got, err := c.Apply(ContentPatch{Title: &title, AcceptanceCriteria: &acs, TypeFields: map[string]any{"actual": nil, "expected": "e"}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Tiêu đề mới" || len(got.AcceptanceCriteria.Items) != 2 {
		t.Fatalf("%+v", got)
	}
	if _, has := got.TypeFields["actual"]; has || got.TypeFields["expected"] != "e" || got.TypeFields["environment"] != "prod" {
		t.Fatalf("type fields = %v", got.TypeFields)
	}
	if c.Title != "Tiêu đề" || c.TypeFields["actual"] != "a" {
		t.Fatal("Apply must not mutate the receiver")
	}
	if _, err := c.Apply(ContentPatch{TypeFields: map[string]any{"ac_next": 9}}); err == nil {
		t.Fatal("ac_next is server managed")
	}
}

func TestNewRequest_ContentDefaults(t *testing.T) {
	r, err := NewRequest(NewRequestInput{TenantID: "t", Title: "Hello", SourceProvider: "manual", ReporterID: "u"})
	if err != nil {
		t.Fatal(err)
	}
	if r.ContentRevision != 1 || r.ContentSchemaVersion != 1 || string(r.AcceptanceCriteriaJSON) != "[]" || string(r.TypeFieldsJSON) != "{}" || len(r.ContentDigest) != 64 {
		t.Fatalf("%+v", r)
	}
	c, _ := ContentFromRequest(r)
	if d, _ := c.Digest(); d != r.ContentDigest {
		t.Fatal("the stored digest must equal the digest of the content")
	}
	r2, err := r.WithContent(RequestContent{Title: "Khác", Body: "b"})
	if err != nil || r2.ContentDigest == r.ContentDigest || r2.Title != "Khác" {
		t.Fatalf("%+v %v", r2, err)
	}
	if r.Title != "Hello" {
		t.Fatal("WithContent must not mutate r")
	}
}

func TestParseRevisionCause(t *testing.T) {
	for _, c := range AllRevisionCauses() {
		if got, err := ParseRevisionCause(string(c)); err != nil || got != c {
			t.Errorf("%s: %v", c, err)
		}
	}
	if _, err := ParseRevisionCause("magic"); err == nil {
		t.Error("unknown cause must fail")
	}
	if RevisionActorFrom(ActorKindAgent) != RevisionActorAI || RevisionActorFrom(ActorKindSystem) != RevisionActorSystem || RevisionActorFrom(ActorKindUser) != RevisionActorUser {
		t.Error("actor mapping")
	}
}
