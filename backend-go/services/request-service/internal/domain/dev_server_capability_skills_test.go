package domain

import (
	"testing"
	"time"
)

func parseSkills(t *testing.T, skills string) DevServerCapability {
	t.Helper()
	c, err := ParseCapabilityProfile("agent", false, true, "2.2.0", 2, nil, []byte(`{"schemaVersion":1,"skills":`+skills+`}`), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestParseCapabilityProfile_ReadsSkills(t *testing.T) {
	c := parseSkills(t, `[{"name":"tdd-knowledge","installed":true,"sha256":"aaaa"},{"name":"tdd-orca-task","installed":false}]`)
	if !c.SkillsKnown || !c.Skills["tdd-knowledge"].Installed || c.Skills["tdd-knowledge"].SHA256 != "aaaa" || c.Skills["tdd-orca-task"].Installed {
		t.Fatalf("%+v", c.Skills)
	}
	old, _ := ParseCapabilityProfile("agent", false, true, "2.1.0", 2, nil, []byte(`{"schemaVersion":1}`), time.Time{})
	if old.SkillsKnown || old.SkillHash(KnowledgeSkillNames) != "" {
		t.Fatal("an agent that predates the field reports nothing, and that is not the same as not installed")
	}
}

func TestSkillHash(t *testing.T) {
	all := `[{"name":"tdd-orca-task","installed":true,"sha256":"4"},{"name":"tdd-knowledge","installed":true,"sha256":"1"},` +
		`{"name":"tdd-knowledge-writeback","installed":true,"sha256":"2"},{"name":"tdd-openspec-author","installed":true,"sha256":"3"}]`
	got := parseSkills(t, all).SkillHash(KnowledgeSkillNames)
	if got != "tdd-knowledge=1,tdd-knowledge-writeback=2,tdd-openspec-author=3,tdd-orca-task=4" {
		t.Fatalf("sorted by name so the hash does not depend on report order: %q", got)
	}
	missing := parseSkills(t, `[{"name":"tdd-knowledge","installed":true,"sha256":"1"}]`)
	if missing.SkillHash(KnowledgeSkillNames) != "" {
		t.Fatal("one missing skill means no hash: the run did not work under the full instructions")
	}
	notInstalled := parseSkills(t, `[{"name":"tdd-knowledge","installed":false}]`)
	if notInstalled.SkillHash([]string{"tdd-knowledge"}) != "" {
		t.Fatal("an uninstalled skill has no hash")
	}
}
