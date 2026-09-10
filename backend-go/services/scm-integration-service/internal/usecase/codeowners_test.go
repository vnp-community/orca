package usecase

import (
	"reflect"
	"testing"
)

const fixtureCodeowners = `
# Default owner for everything
*       @global-owner

# Go files
*.go    @go-team

# Docs, recursively
/docs/** @docs-team

# A specific overriding rule for a subpath
/docs/api/ @api-team @org/team-frontend
`

func TestParseCodeowners(t *testing.T) {
	rules := ParseCodeowners(fixtureCodeowners)

	want := []CodeownersRule{
		{Pattern: "*", Owners: []string{"@global-owner"}},
		{Pattern: "*.go", Owners: []string{"@go-team"}},
		{Pattern: "/docs/**", Owners: []string{"@docs-team"}},
		{Pattern: "/docs/api/", Owners: []string{"@api-team", "@org/team-frontend"}},
	}
	if !reflect.DeepEqual(rules, want) {
		t.Fatalf("unexpected rules: got %+v, want %+v", rules, want)
	}
}

func TestParseCodeowners_SkipsBlankLinesAndComments(t *testing.T) {
	rules := ParseCodeowners("\n# just a comment\n\n   \n*.go @go-team\n")
	if len(rules) != 1 || rules[0].Pattern != "*.go" {
		t.Fatalf("expected exactly one parsed rule, got %+v", rules)
	}
}

func TestMatchOwners_LastMatchWinsOverCatchAll(t *testing.T) {
	rules := ParseCodeowners(fixtureCodeowners)

	logins, teams := MatchOwners(rules, []string{"main.go"})
	if len(logins) != 1 || logins[0] != "go-team" {
		t.Errorf("expected *.go's more specific rule to win over the * catch-all, got logins=%v", logins)
	}
	if len(teams) != 0 {
		t.Errorf("expected no teams for main.go, got %v", teams)
	}
}

func TestMatchOwners_DirectoryPrefixMatch(t *testing.T) {
	rules := ParseCodeowners(fixtureCodeowners)

	logins, teams := MatchOwners(rules, []string{"docs/guide.md"})
	if len(logins) != 1 || logins[0] != "docs-team" {
		t.Errorf("expected docs-team to own docs/guide.md, got logins=%v", logins)
	}
	if len(teams) != 0 {
		t.Errorf("expected no teams for docs/guide.md, got %v", teams)
	}
}

func TestMatchOwners_MoreSpecificSubpathOverridesDirectoryRule(t *testing.T) {
	rules := ParseCodeowners(fixtureCodeowners)

	logins, teams := MatchOwners(rules, []string{"docs/api/reference.md"})
	if len(logins) != 1 || logins[0] != "api-team" {
		t.Errorf("expected api-team (last match) to win for docs/api/reference.md, got logins=%v", logins)
	}
	if len(teams) != 1 || teams[0] != "org/team-frontend" {
		t.Errorf("expected org/team-frontend to be extracted as a team, got teams=%v", teams)
	}
}

func TestMatchOwners_NoMatchingRuleSkipped(t *testing.T) {
	rules := []CodeownersRule{{Pattern: "*.go", Owners: []string{"@go-team"}}}
	logins, teams := MatchOwners(rules, []string{"README.md"})
	if len(logins) != 0 || len(teams) != 0 {
		t.Errorf("expected no owners for a file matching no rule, got logins=%v teams=%v", logins, teams)
	}
}

func TestMatchOwners_DeduplicatesAcrossFiles(t *testing.T) {
	rules := []CodeownersRule{{Pattern: "*", Owners: []string{"@global-owner"}}}
	logins, _ := MatchOwners(rules, []string{"a.go", "b.go"})
	if len(logins) != 1 || logins[0] != "global-owner" {
		t.Errorf("expected a deduplicated single owner across files, got %v", logins)
	}
}
