package wscompat

import (
	"encoding/json"
	"strings"
	"testing"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

func TestRequestViews_NilAndUnknownEnumsDoNotPanic(t *testing.T) {
	_ = RequestViewOf(nil, true)
	_ = SolutionViewOf(nil)
	_ = ApprovalViewOf(nil)
	s := SolutionViewOf(&requestv1.Solution{Kind: requestv1.SolutionKind(99), Status: requestv1.SolutionStatus(-3), OptionsJson: `{broken`})
	if s.Kind != "" || s.Status != "" || string(s.Options) != "null" || s.ChosenOptionID != "" {
		t.Errorf("view = %+v", s)
	}
	a := ApprovalViewOf(&requestv1.Approval{SubjectType: requestv1.ApprovalSubjectType(42)})
	if a.SubjectType != "" || a.Status != "" {
		t.Errorf("approval = %+v", a)
	}
}

func TestRequestView_NullableFieldsAndBody(t *testing.T) {
	b, _ := json.Marshal(RequestViewOf(&requestv1.Request{Id: "r", Body: "secret body"}, false))
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	for _, k := range []string{"type", "typeSource", "size", "urgency", "confidence"} {
		if v, present := m[k]; !present || v != nil {
			t.Errorf("%s = %v (present=%v), want explicit null", k, v, present)
		}
	}
	if _, has := m["body"]; has || strings.Contains(string(b), "secret body") {
		t.Errorf("body must be omitted without withBody: %s", b)
	}
	b, _ = json.Marshal(RequestViewOf(&requestv1.Request{Id: "r", Body: ""}, true))
	if !strings.Contains(string(b), `"body":""`) {
		t.Errorf("an empty body must still be present on request.get: %s", b)
	}
}

func TestChosenOptionID(t *testing.T) {
	doc := `{"options":[{"id":"opt-0"},{"id":"opt-1"}]}`
	for _, c := range []struct {
		chosen int32
		want   string
	}{{-1, ""}, {0, "opt-0"}, {1, "opt-1"}, {2, ""}} {
		if got := chosenOptionID(doc, c.chosen); got != c.want {
			t.Errorf("chosen %d -> %q, want %q", c.chosen, got, c.want)
		}
	}
	if chosenOptionID(`[]`, 0) != "" {
		t.Error("a document of another shape must drop the field, not fail")
	}
}

func TestResolveRequestSource_Table(t *testing.T) {
	cases := []struct {
		name         string
		origin       *ToolOrigin
		in           RequestSourceInput
		wantProvider string
		wantSite     string
		wantErr      string
	}{
		{"no source is manual", nil, RequestSourceInput{}, "manual", "", ""},
		{"jira kept", nil, RequestSourceInput{Provider: "jira", Ref: "A-1", Site: "acme"}, "jira", "acme", ""},
		{"github case-insensitive", nil, RequestSourceInput{Provider: " GitHub ", Ref: "o/r#1"}, "github", "", ""},
		{"gitlab kept", nil, RequestSourceInput{Provider: "gitlab", Ref: "g/p#2"}, "gitlab", "", ""},
		{"linear kept", nil, RequestSourceInput{Provider: "linear", Ref: "L-3"}, "linear", "", ""},
		{"explicit manual refused", nil, RequestSourceInput{Provider: "manual"}, "", "", "REQUEST_SOURCE_FORBIDDEN"},
		{"explicit mcp refused", nil, RequestSourceInput{Provider: "mcp"}, "", "", "REQUEST_SOURCE_FORBIDDEN"},
		{"explicit webhook refused", nil, RequestSourceInput{Provider: "webhook"}, "", "", "REQUEST_SOURCE_FORBIDDEN"},
		{"origin wins over a tracker claim", &ToolOrigin{ClientName: "cursor"}, RequestSourceInput{Provider: "jira", Ref: "A-1", Site: "evil"}, "mcp", "cursor", ""},
		{"origin wins over a forbidden claim", &ToolOrigin{ClientName: "cursor"}, RequestSourceInput{Provider: "webhook"}, "mcp", "cursor", ""},
		{"origin without client name", &ToolOrigin{}, RequestSourceInput{}, "mcp", "unknown-mcp-client", ""},
	}
	for _, c := range cases {
		got, err := ResolveRequestSource(c.origin, c.in)
		if c.wantErr != "" {
			if err == nil || !strings.HasPrefix(err.Error(), c.wantErr) {
				t.Errorf("%s: err = %v, want %s", c.name, err, c.wantErr)
			}
			continue
		}
		if err != nil || got.GetProvider() != c.wantProvider || got.GetSite() != c.wantSite {
			t.Errorf("%s: got %+v err=%v", c.name, got, err)
		}
		if c.wantProvider == "mcp" && (got.GetRef() != "" || got.GetUrl() != "") {
			t.Errorf("%s: an mcp source has no ref or url: %+v", c.name, got)
		}
	}
}
