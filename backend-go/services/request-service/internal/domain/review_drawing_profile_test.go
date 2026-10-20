package domain

import (
	"strings"
	"testing"
)

func profileCode(err error) bool {
	return err != nil && strings.Contains(err.Error(), "drawing profile")
}

func TestDrawingProfile_DefaultIsStableAndPresetsAreSeparatePerSection(t *testing.T) {
	a, b := DefaultDrawingProfile(), DefaultDrawingProfile()
	for _, sec := range drawnSections {
		for _, v := range []string{DiagramVariantDesign, DiagramVariantDelta} {
			da, err := a.PresetDigest(sec, v)
			db, _ := b.PresetDigest(sec, v)
			if err != nil || da == "" || da != db {
				t.Fatalf("%s/%s: %s %s %v", sec, v, da, db, err)
			}
		}
	}
	seen := map[string]bool{}
	for _, sec := range drawnSections {
		for _, v := range []string{DiagramVariantDesign, DiagramVariantDelta} {
			d, _ := a.PresetDigest(sec, v)
			if seen[d] {
				t.Errorf("two presets share a digest: %s/%s", sec, v)
			}
			seen[d] = true
		}
	}
	if !a.Draws(SectionFlow, DiagramVariantDelta) || a.Draws(ReviewSection("config"), DiagramVariantDelta) {
		t.Error("the default draws four sections and nothing else")
	}
	if a.Sections[SectionArchitecture].Level != LevelContainer || a.Sections[SectionFlow].MaxNodes != MaxDiagramLanes {
		t.Errorf("defaults: %+v", a.Sections)
	}
}

func TestDrawingProfile_ChangingOneSectionRedrawsOnlyThatSection(t *testing.T) {
	base := DefaultDrawingProfile()
	three := 8
	override, err := ParseDrawingProfile([]byte(`{"schema":"review-drawing-profile/1","sections":{"flow":{"maxLanes":4}}}`))
	if err != nil {
		t.Fatal(err)
	}
	_ = three
	changed := ResolveDrawingProfile(Layer{Scope: SourceTenant, Profile: override})
	for _, sec := range drawnSections {
		for _, v := range []string{DiagramVariantDesign, DiagramVariantDelta} {
			x, _ := base.PresetDigest(sec, v)
			y, _ := changed.PresetDigest(sec, v)
			if (sec == SectionFlow) == (x == y) {
				t.Errorf("%s/%s: same=%v, but only flow should change", sec, v, x == y)
			}
		}
	}
	if changed.Sections[SectionFlow].MaxNodes != 4 || changed.Sources["sections.flow.maxLanes"] != SourceTenant {
		t.Errorf("resolved: %+v sources=%v", changed.Sections[SectionFlow], changed.Sources)
	}
	// A style change touches every section.
	styled, _ := ParseDrawingProfile([]byte(`{"schema":"review-drawing-profile/1","style":{"detail":"simplified"}}`))
	s := ResolveDrawingProfile(Layer{Scope: SourceProject, Profile: styled})
	x, _ := base.PresetDigest(SectionData, DiagramVariantDelta)
	y, _ := s.PresetDigest(SectionData, DiagramVariantDelta)
	if x == y {
		t.Error("a style change must change every preset")
	}
}

func TestDrawingProfile_ProjectOverridesTenantByField(t *testing.T) {
	tenant, _ := ParseDrawingProfile([]byte(`{"schema":"review-drawing-profile/1","style":{"detail":"faithful","audience":"engineer"},"sections":{"data":{"enabled":false}},"language":"vi"}`))
	project, _ := ParseDrawingProfile([]byte(`{"schema":"review-drawing-profile/1","style":{"detail":"simplified"},"sections":{"data":{"enabled":true}}}`))
	r := ResolveDrawingProfile(Layer{SourceTenant, tenant}, Layer{SourceProject, project})
	if r.Style.Detail != "simplified" || r.Sources["style.detail"] != SourceProject {
		t.Errorf("project wins: %+v %v", r.Style, r.Sources)
	}
	if r.Style.Audience != "engineer" || r.Sources["style.audience"] != SourceTenant || r.Language != "vi" {
		t.Errorf("fields the project leaves alone come from the tenant: %+v", r.Style)
	}
	if !r.Sections[SectionData].Enabled || r.Sources["sections.data.enabled"] != SourceProject {
		t.Errorf("section: %+v", r.Sections[SectionData])
	}
	if r.Style.Variant != "minimal" || r.Sources["style.variant"] != "" {
		t.Errorf("untouched fields stay default: %+v %v", r.Style, r.Sources)
	}
	if len(r.Style.Roles) != 4 {
		t.Errorf("roles: %v", r.Style.Roles)
	}
}

func TestDrawingProfile_Rejections(t *testing.T) {
	cases := map[string]struct{ doc, path string }{
		"unknown key":              {`{"schema":"review-drawing-profile/1","skill":"mine"}`, "$"},
		"another schema":           {`{"schema":"review-drawing-profile/2"}`, "schema"},
		"another base":             {`{"schema":"review-drawing-profile/1","base":"other"}`, "base"},
		"a type outside the list":  {`{"schema":"review-drawing-profile/1","sections":{"flow":{"type":"data-flow"}}}`, "sections.flow.type"},
		"a component level":        {`{"schema":"review-drawing-profile/1","sections":{"architecture":{"level":"component"}}}`, "sections.architecture.level"},
		"a level on a table":       {`{"schema":"review-drawing-profile/1","sections":{"data":{"level":"container"}}}`, "sections.data.level"},
		"too many nodes":           {`{"schema":"review-drawing-profile/1","sections":{"architecture":{"maxNodes":21}}}`, "sections.architecture.maxNodes"},
		"too few nodes":            {`{"schema":"review-drawing-profile/1","sections":{"architecture":{"maxNodes":1}}}`, "sections.architecture.maxNodes"},
		"lanes on the wrong place": {`{"schema":"review-drawing-profile/1","sections":{"data":{"maxLanes":3}}}`, "sections.data.maxLanes"},
		"too many lanes":           {`{"schema":"review-drawing-profile/1","sections":{"flow":{"maxLanes":8}}}`, "sections.flow.maxLanes"},
		"nodes on the flow":        {`{"schema":"review-drawing-profile/1","sections":{"flow":{"maxNodes":5}}}`, "sections.flow.maxNodes"},
		"a section not drawn":      {`{"schema":"review-drawing-profile/1","sections":{"config":{}}}`, "sections.config"},
		"an empty variant list":    {`{"schema":"review-drawing-profile/1","sections":{"flow":{"variants":[]}}}`, "sections.flow.variants"},
		"a variant that is not":    {`{"schema":"review-drawing-profile/1","sections":{"flow":{"variants":["full"]}}}`, "sections.flow.variants"},
		"the full style":           {`{"schema":"review-drawing-profile/1","style":{"variant":"full"}}`, "style.variant"},
		"a detail level":           {`{"schema":"review-drawing-profile/1","style":{"detail":"pixel"}}`, "style.detail"},
		"a hex colour":             {`{"schema":"review-drawing-profile/1","style":{"roles":{"accent":"#ff0000"}}}`, "style.roles.accent"},
		"an unknown token":         {`{"schema":"review-drawing-profile/1","style":{"roles":{"accent":"token:hotpink"}}}`, "style.roles.accent"},
		"a role that is not":       {`{"schema":"review-drawing-profile/1","style":{"roles":{"border":"token:accent"}}}`, "style.roles.border"},
		"a language":               {`{"schema":"review-drawing-profile/1","language":"fr"}`, "language"},
		"two documents":            {`{"schema":"review-drawing-profile/1"}{}`, "$"},
		"not json":                 {`nope`, "$"},
	}
	for name, c := range cases {
		_, err := ParseDrawingProfile([]byte(c.doc))
		if !profileCode(err) {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if !strings.Contains(err.Error(), c.path) {
			t.Errorf("%s: the error should name %q: %v", name, c.path, err)
		}
	}
	for _, ok := range []string{
		`{"schema":"review-drawing-profile/1"}`,
		`{"schema":"review-drawing-profile/1","base":"orca-default","name":"mine","sections":{"architecture":{"type":"architecture-delta","level":"container","maxNodes":20},"flow":{"maxLanes":7,"variants":["delta"]}},"style":{"detail":"faithful","audience":"executive","size":"large","motion":true,"sketchy":true,"roles":{"diff-add":"token:accent"}},"language":"vi"}`,
	} {
		if _, err := ParseDrawingProfile([]byte(ok)); err != nil {
			t.Errorf("%s: %v", ok, err)
		}
	}
}

func TestDrawingProfile_SpecDigestIgnoresKeyOrder(t *testing.T) {
	a, _ := ParseDrawingProfile([]byte(`{"schema":"review-drawing-profile/1","language":"vi","style":{"detail":"faithful","size":"large"}}`))
	b, _ := ParseDrawingProfile([]byte(`{"style":{"size":"large","detail":"faithful"},"language":"vi","schema":"review-drawing-profile/1"}`))
	da, _ := a.SpecDigest()
	db, _ := b.SpecDigest()
	if da == "" || da != db {
		t.Errorf("%s %s", da, db)
	}
}
