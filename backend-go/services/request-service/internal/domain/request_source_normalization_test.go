package domain

import (
	"errors"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
)

func codeOf(err error) string {
	var ae *apperrors.AppError
	if errors.As(err, &ae) {
		return ae.Code
	}
	return ""
}

func TestNormalizeSourceRef_Jira(t *testing.T) {
	cases := []struct {
		name         string
		in           SourceRef
		wantRef      string
		wantSite     string
		wantErrCode  string
		wantUnchSite bool
	}{
		{"uppercases key", SourceRef{Provider: SourceProviderJira, Ref: "eng-1", Site: "https://acme.atlassian.net"}, "ENG-1", "https://acme.atlassian.net", "", false},
		{"canonical site", SourceRef{Provider: SourceProviderJira, Ref: " ENG-1 ", Site: "HTTPS://Acme.Atlassian.NET/"}, "ENG-1", "https://acme.atlassian.net", "", false},
		{"empty site stays empty", SourceRef{Provider: SourceProviderJira, Ref: "eng-2"}, "ENG-2", "", "", false},
		{"workspace id kept as typed", SourceRef{Provider: SourceProviderJira, Ref: "eng-3", Site: "WorkSpace-42"}, "ENG-3", "WorkSpace-42", "", false},
		{"missing ref", SourceRef{Provider: SourceProviderJira, Site: "x"}, "", "", "REQUEST_SOURCE_REF_REQUIRED", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := NormalizeSourceRef(c.in)
			if c.wantErrCode != "" {
				if codeOf(err) != c.wantErrCode {
					t.Fatalf("want %s, got %v", c.wantErrCode, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Ref != c.wantRef || got.Site != c.wantSite {
				t.Fatalf("got %+v, want ref=%q site=%q", got, c.wantRef, c.wantSite)
			}
		})
	}
}

func TestNormalizeSourceRef_Linear(t *testing.T) {
	got, err := NormalizeSourceRef(SourceRef{Provider: SourceProviderLinear, Ref: "abc-9", Site: " team "})
	if err != nil || got.Ref != "ABC-9" || got.Site != "team" {
		t.Fatalf("got %+v err=%v", got, err)
	}
}

func TestNormalizeSourceRef_GitHub(t *testing.T) {
	got, err := NormalizeSourceRef(SourceRef{Provider: SourceProviderGithub, Ref: "Owner/Repo#12"})
	if err != nil || got.Ref != "owner/repo#12" {
		t.Fatalf("got %+v err=%v", got, err)
	}
	got, err = NormalizeSourceRef(SourceRef{Provider: SourceProviderGithub, Ref: "o/r#007"})
	if err != nil || got.Ref != "o/r#7" {
		t.Fatalf("leading zeros must canonicalise, got %+v err=%v", got, err)
	}
	for _, bad := range []string{"owner/repo", "owner/repo#0", "owner/repo#abc", "owner/repo#-3", "#5", "owner/repo#"} {
		if _, err := NormalizeSourceRef(SourceRef{Provider: SourceProviderGithub, Ref: bad}); codeOf(err) != "REQUEST_SOURCE_REF_INVALID" {
			t.Errorf("ref %q: want REQUEST_SOURCE_REF_INVALID, got %v", bad, err)
		}
	}
}

func TestNormalizeSourceRef_GitLabSubgroups(t *testing.T) {
	got, err := NormalizeSourceRef(SourceRef{Provider: SourceProviderGitlab, Ref: "Group/Sub/Repo#9", Site: "https://GitLab.example.com/"})
	if err != nil || got.Ref != "group/sub/repo#9" || got.Site != "https://gitlab.example.com" {
		t.Fatalf("got %+v err=%v", got, err)
	}
}

func TestNormalizeSourceRef_Webhook(t *testing.T) {
	if _, err := NormalizeSourceRef(SourceRef{Provider: SourceProviderWebhook, Site: "sentry", Ref: "evt-1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := NormalizeSourceRef(SourceRef{Provider: SourceProviderWebhook, Ref: "evt-1"}); codeOf(err) != "REQUEST_SOURCE_SITE_REQUIRED" {
		t.Fatalf("missing site: %v", err)
	}
	if _, err := NormalizeSourceRef(SourceRef{Provider: SourceProviderWebhook, Site: "sentry"}); codeOf(err) != "REQUEST_SOURCE_REF_REQUIRED" {
		t.Fatalf("missing ref: %v", err)
	}
}

func TestNormalizeSourceRef_ManualAndMCPRejectRef(t *testing.T) {
	for _, p := range []SourceProvider{SourceProviderManual, SourceProviderMCP} {
		if _, err := NormalizeSourceRef(SourceRef{Provider: p}); err != nil {
			t.Errorf("%s with empty ref: %v", p, err)
		}
		if _, err := NormalizeSourceRef(SourceRef{Provider: p, Ref: "x"}); codeOf(err) != "REQUEST_SOURCE_REF_INVALID" {
			t.Errorf("%s with ref: %v", p, err)
		}
	}
}

func TestNormalizeSourceRef_UnknownProviderAndLongRef(t *testing.T) {
	if _, err := NormalizeSourceRef(SourceRef{Provider: "svn", Ref: "x"}); codeOf(err) != "REQUEST_SOURCE_PROVIDER_INVALID" {
		t.Fatalf("got %v", err)
	}
	if _, err := NormalizeSourceRef(SourceRef{Provider: SourceProviderJira, Ref: strings.Repeat("a", 256)}); codeOf(err) != "REQUEST_SOURCE_REF_INVALID" {
		t.Fatalf("got %v", err)
	}
}

func TestNormalizeTitle(t *testing.T) {
	if got, err := NormalizeTitle("  hello  "); err != nil || got != "hello" {
		t.Fatalf("got %q %v", got, err)
	}
	if _, err := NormalizeTitle("   "); codeOf(err) != "REQUEST_TITLE_REQUIRED" {
		t.Fatalf("blank: %v", err)
	}
	// 500 multi-byte runes are fine; byte length would exceed 500.
	if _, err := NormalizeTitle(strings.Repeat("ệ", 500)); err != nil {
		t.Fatalf("500 runes: %v", err)
	}
	if _, err := NormalizeTitle(strings.Repeat("ệ", 501)); codeOf(err) != "REQUEST_TITLE_TOO_LONG" {
		t.Fatalf("501 runes: %v", err)
	}
}

func TestNormalizeBody(t *testing.T) {
	if _, err := NormalizeBody(strings.Repeat("ệ", 100000)); err != nil {
		t.Fatalf("100000 runes: %v", err)
	}
	if _, err := NormalizeBody(strings.Repeat("a", 100001)); codeOf(err) != "REQUEST_BODY_TOO_LARGE" {
		t.Fatalf("100001: %v", err)
	}
}

func TestBuildIdempotencyKey(t *testing.T) {
	reporter := "11111111-1111-1111-1111-111111111111"
	key, ok, err := BuildIdempotencyKey(SourceRef{Provider: SourceProviderJira, Site: "s", Ref: "ENG-1"}, reporter, "ignored")
	if err != nil || !ok || key != (IdempotencyKey{Provider: SourceProviderJira, Site: "s", Ref: "ENG-1"}) {
		t.Fatalf("jira: %+v %v %v", key, ok, err)
	}
	if _, ok, err := BuildIdempotencyKey(SourceRef{Provider: SourceProviderManual}, reporter, ""); ok || err != nil {
		t.Fatalf("manual without client id must not claim: ok=%v err=%v", ok, err)
	}
	key, ok, err = BuildIdempotencyKey(SourceRef{Provider: SourceProviderManual}, reporter, " c-1 ")
	if err != nil || !ok || key.Site != "user:"+reporter || key.Ref != "c-1" {
		t.Fatalf("manual: %+v %v %v", key, ok, err)
	}
	key, ok, _ = BuildIdempotencyKey(SourceRef{Provider: SourceProviderMCP}, reporter, "c-1")
	if !ok || key.Provider != SourceProviderMCP {
		t.Fatalf("mcp: %+v %v", key, ok)
	}
	key, ok, _ = BuildIdempotencyKey(SourceRef{Provider: SourceProviderWebhook, Site: "sentry", Ref: "e1"}, reporter, "")
	if !ok || key.Site != "sentry" || key.Ref != "e1" {
		t.Fatalf("webhook: %+v %v", key, ok)
	}
	if _, _, err := BuildIdempotencyKey(SourceRef{Provider: SourceProviderManual}, "", "c"); codeOf(err) != "REQUEST_REPORTER_REQUIRED" {
		t.Fatalf("manual without reporter: %v", err)
	}
}

func TestIdempotencyKey_TwoJiraSitesDiffer(t *testing.T) {
	a, _ := NormalizeSourceRef(SourceRef{Provider: SourceProviderJira, Ref: "eng-1", Site: "https://a.atlassian.net"})
	b, _ := NormalizeSourceRef(SourceRef{Provider: SourceProviderJira, Ref: "ENG-1", Site: "https://b.atlassian.net"})
	ka, _, _ := BuildIdempotencyKey(a, "", "")
	kb, _, _ := BuildIdempotencyKey(b, "", "")
	if ka == kb {
		t.Fatal("different Jira sites must give different keys")
	}
	a2, _ := NormalizeSourceRef(SourceRef{Provider: SourceProviderJira, Ref: "eng-1", Site: "https://A.atlassian.net/"})
	ka2, _, _ := BuildIdempotencyKey(a2, "", "")
	if ka != ka2 {
		t.Fatal("eng-1 and ENG-1 on the same site must give the same key")
	}
}

func TestSourceHints_IsZero(t *testing.T) {
	if !(SourceHints{}).IsZero() || (SourceHints{Labels: []string{"x"}}).IsZero() || (SourceHints{TypeHint: "bug"}).IsZero() {
		t.Fatal("IsZero wrong")
	}
}
