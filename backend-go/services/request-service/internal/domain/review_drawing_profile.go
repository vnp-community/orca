package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// A drawing profile is data with a schema, never code: it picks among the choices Orca approved and cannot name a skill, a
// script, a template or a colour. The default is part of the code, so a stored profile only says what differs from it.

const (
	DrawingProfileSchema = "review-drawing-profile/1"
	DrawingProfileBase   = "orca-default"

	MaxProfileNodes = 20
	MaxProfileLanes = 7
	minProfileNodes = 3
	minProfileLanes = 2
)

// Where a resolved value came from.
const (
	SourceDefault = "default"
	SourceTenant  = "tenant"
	SourceProject = "project"
)

type SectionProfile struct {
	Enabled  *bool       `json:"enabled,omitempty"`
	Type     DiagramType `json:"type,omitempty"`
	Level    string      `json:"level,omitempty"`
	Variants []string    `json:"variants,omitempty"`
	MaxNodes *int        `json:"maxNodes,omitempty"`
	MaxLanes *int        `json:"maxLanes,omitempty"`
}

type StyleProfile struct {
	Variant  string            `json:"variant,omitempty"`
	Detail   string            `json:"detail,omitempty"`
	Audience string            `json:"audience,omitempty"`
	Size     string            `json:"size,omitempty"`
	Motion   *bool             `json:"motion,omitempty"`
	Sketchy  *bool             `json:"sketchy,omitempty"`
	Icons    *bool             `json:"icons,omitempty"`
	Roles    map[string]string `json:"roles,omitempty"`
}

// DrawingProfile is one stored layer (tenant or project). Every field is optional: absent means "as the layer below".
type DrawingProfile struct {
	Schema   string                           `json:"schema"`
	Name     string                           `json:"name,omitempty"`
	Base     string                           `json:"base,omitempty"`
	Sections map[ReviewSection]SectionProfile `json:"sections,omitempty"`
	Style    *StyleProfile                    `json:"style,omitempty"`
	Language string                           `json:"language,omitempty"`
}

type ResolvedSection struct {
	Enabled  bool        `json:"enabled"`
	Type     DiagramType `json:"type"`
	Level    string      `json:"level,omitempty"`
	Variants []string    `json:"variants"`
	MaxNodes int         `json:"maxNodes"`
}

type ResolvedStyle struct {
	Variant  string            `json:"variant"`
	Detail   string            `json:"detail"`
	Audience string            `json:"audience"`
	Size     string            `json:"size"`
	Motion   bool              `json:"motion"`
	Sketchy  bool              `json:"sketchy"`
	Icons    bool              `json:"icons"`
	Roles    map[string]string `json:"roles"`
}

// ResolvedProfile is the profile in force and, for every field, the layer that decided it.
type ResolvedProfile struct {
	Sections map[ReviewSection]ResolvedSection `json:"sections"`
	Style    ResolvedStyle                     `json:"style"`
	Language string                            `json:"language"`
	// Sources maps a path such as "sections.flow.maxNodes" or "style.detail" to default|tenant|project.
	Sources map[string]string `json:"-"`
}

var drawnSections = []ReviewSection{SectionArchitecture, SectionFlow, SectionBehavior, SectionData}

// roleTokens are the names of the Orca colour tokens a role may point at. No hex colour is accepted in this version.
var roleTokens = map[string]bool{"accent": true, "diff-add": true, "diff-mod": true, "diff-del": true, "muted": true, "neutral": true}

var roleNames = map[string]bool{"accent": true, "diff-add": true, "diff-mod": true, "diff-del": true}

// DefaultDrawingProfile is orca-default: immutable, in the code. A profile that is stored changes it only by what it says.
func DefaultDrawingProfile() ResolvedProfile {
	sec := func(t DiagramType, level string) ResolvedSection {
		return ResolvedSection{Enabled: true, Type: t, Level: level, Variants: []string{DiagramVariantDesign, DiagramVariantDelta}, MaxNodes: MaxDiagramNodes}
	}
	flow := sec(DiagramSequence, LevelContainer)
	flow.MaxNodes = MaxDiagramLanes
	p := ResolvedProfile{
		Sections: map[ReviewSection]ResolvedSection{
			SectionArchitecture: sec(DiagramArchitecture, LevelContainer), SectionFlow: flow,
			SectionBehavior: sec(DiagramStateMachine, ""), SectionData: sec(DiagramDatabase, ""),
		},
		Style: ResolvedStyle{Variant: "minimal", Detail: "balanced", Audience: "mixed", Size: "medium",
			Roles: map[string]string{"accent": "token:accent", "diff-add": "token:diff-add", "diff-mod": "token:diff-mod", "diff-del": "token:diff-del"}},
		Language: "en",
		Sources:  map[string]string{},
	}
	return p
}

// ParseDrawingProfile reads one layer and refuses what it does not know: an unknown key is an error with its path, not a value
// that is quietly ignored.
func ParseDrawingProfile(raw []byte) (DrawingProfile, error) {
	var p DrawingProfile
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return DrawingProfile{}, ErrReviewProfileInvalid("$", jsonProblem(err))
	}
	if dec.More() {
		return DrawingProfile{}, ErrReviewProfileInvalid("$", "more than one JSON value")
	}
	if err := p.Validate(); err != nil {
		return DrawingProfile{}, err
	}
	return p, nil
}

func jsonProblem(err error) string {
	msg := err.Error()
	if i := strings.Index(msg, "unknown field "); i >= 0 {
		return "unknown field " + strings.TrimSpace(msg[i+len("unknown field "):])
	}
	return "not a drawing profile: " + msg
}

// Validate checks a layer against the approved choices. The error names the path of the field, which a form shows next to it.
func (p DrawingProfile) Validate() error {
	bad := func(path, why string) error { return ErrReviewProfileInvalid(path, why) }
	if p.Schema != DrawingProfileSchema {
		return bad("schema", fmt.Sprintf("must be %q", DrawingProfileSchema))
	}
	if p.Base != "" && p.Base != DrawingProfileBase {
		return bad("base", fmt.Sprintf("must be %q or left out", DrawingProfileBase))
	}
	if len([]rune(p.Name)) > 60 {
		return bad("name", "at most 60 characters")
	}
	if p.Language != "" && p.Language != "en" && p.Language != "vi" {
		return bad("language", "must be en or vi")
	}
	def := DefaultDrawingProfile()
	for _, name := range sortedSections(p.Sections) {
		s := p.Sections[name]
		path := "sections." + string(name)
		want, ok := def.Sections[name]
		if !ok {
			return bad(path, "not a drawn section; use architecture, flow, behavior or data")
		}
		if s.Type != "" && s.Type != want.Type {
			return bad(path+".type", fmt.Sprintf("only %q is approved for %s", want.Type, name))
		}
		if s.Level != "" && s.Level != want.Level {
			if want.Level == LevelContainer {
				return bad(path+".level", "must stay container: the review shows the boundaries of services, not their parts")
			}
			return bad(path+".level", "this section has no level")
		}
		if s.Variants != nil {
			if len(s.Variants) == 0 {
				return bad(path+".variants", "at least one of design, delta")
			}
			for _, v := range s.Variants {
				if v != DiagramVariantDesign && v != DiagramVariantDelta {
					return bad(path+".variants", fmt.Sprintf("%q is not design or delta", v))
				}
			}
		}
		if s.MaxNodes != nil && (*s.MaxNodes < minProfileNodes || *s.MaxNodes > MaxProfileNodes) {
			return bad(path+".maxNodes", fmt.Sprintf("from %d to %d", minProfileNodes, MaxProfileNodes))
		}
		if s.MaxLanes != nil {
			if name != SectionFlow {
				return bad(path+".maxLanes", "only the flow has lanes")
			}
			if *s.MaxLanes < minProfileLanes || *s.MaxLanes > MaxProfileLanes {
				return bad(path+".maxLanes", fmt.Sprintf("from %d to %d", minProfileLanes, MaxProfileLanes))
			}
		}
		if name == SectionFlow && s.MaxNodes != nil {
			return bad(path+".maxNodes", "a flow is limited by maxLanes")
		}
	}
	if st := p.Style; st != nil {
		if err := st.validate(); err != nil {
			return err
		}
	}
	return nil
}

func (st StyleProfile) validate() error {
	in := func(path, v string, ok ...string) error {
		if v == "" {
			return nil
		}
		for _, o := range ok {
			if v == o {
				return nil
			}
		}
		return ErrReviewProfileInvalid(path, fmt.Sprintf("%q is not one of %s", v, strings.Join(ok, ", ")))
	}
	if st.Variant == "full" {
		return ErrReviewProfileInvalid("style.variant", "full is not offered yet; use minimal")
	}
	for _, c := range []error{
		in("style.variant", st.Variant, "minimal"), in("style.detail", st.Detail, "faithful", "balanced", "simplified"),
		in("style.audience", st.Audience, "engineer", "mixed", "executive"), in("style.size", st.Size, "small", "medium", "large"),
	} {
		if c != nil {
			return c
		}
	}
	for _, role := range sortedKeysOf(st.Roles) {
		path := "style.roles." + role
		if !roleNames[role] {
			return ErrReviewProfileInvalid(path, "not a role; use accent, diff-add, diff-mod or diff-del")
		}
		v := st.Roles[role]
		name, found := strings.CutPrefix(v, "token:")
		if !found || !roleTokens[name] {
			return ErrReviewProfileInvalid(path, "must name an Orca colour token such as token:diff-add; a colour value is not accepted")
		}
	}
	return nil
}

func sortedSections(m map[ReviewSection]SectionProfile) []ReviewSection {
	out := make([]ReviewSection, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func sortedKeysOf(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Layer is a stored profile and the scope it came from.
type Layer struct {
	Scope   string // tenant or project
	Profile DrawingProfile
}

// ResolveDrawingProfile applies the layers over orca-default in the order given (tenant first, then project), by field, and
// remembers which layer decided each field.
func ResolveDrawingProfile(layers ...Layer) ResolvedProfile {
	out := DefaultDrawingProfile()
	for _, l := range layers {
		p := l.Profile
		for _, name := range sortedSections(p.Sections) {
			s, cur := p.Sections[name], out.Sections[name]
			prefix := "sections." + string(name) + "."
			if s.Enabled != nil {
				cur.Enabled, out.Sources[prefix+"enabled"] = *s.Enabled, l.Scope
			}
			if s.Variants != nil {
				cur.Variants, out.Sources[prefix+"variants"] = append([]string(nil), s.Variants...), l.Scope
			}
			if s.MaxNodes != nil {
				cur.MaxNodes, out.Sources[prefix+"maxNodes"] = *s.MaxNodes, l.Scope
			}
			if s.MaxLanes != nil {
				cur.MaxNodes, out.Sources[prefix+"maxLanes"] = *s.MaxLanes, l.Scope
			}
			out.Sections[name] = cur
		}
		if st := p.Style; st != nil {
			set := func(dst *string, v, key string) {
				if v != "" {
					*dst, out.Sources["style."+key] = v, l.Scope
				}
			}
			set(&out.Style.Variant, st.Variant, "variant")
			set(&out.Style.Detail, st.Detail, "detail")
			set(&out.Style.Audience, st.Audience, "audience")
			set(&out.Style.Size, st.Size, "size")
			flag := func(dst *bool, v *bool, key string) {
				if v != nil {
					*dst, out.Sources["style."+key] = *v, l.Scope
				}
			}
			flag(&out.Style.Motion, st.Motion, "motion")
			flag(&out.Style.Sketchy, st.Sketchy, "sketchy")
			flag(&out.Style.Icons, st.Icons, "icons")
			for role, tok := range st.Roles {
				out.Style.Roles[role], out.Sources["style.roles."+role] = tok, l.Scope
			}
		}
		if p.Language != "" {
			out.Language, out.Sources["language"] = p.Language, l.Scope
		}
	}
	return out
}

// Draws says whether the profile wants this variant of the section drawn.
func (r ResolvedProfile) Draws(section ReviewSection, variant string) bool {
	s, ok := r.Sections[section]
	if !ok || !s.Enabled {
		return false
	}
	for _, v := range s.Variants {
		if v == variant {
			return true
		}
	}
	return false
}

// SpecDigest is the digest of a stored layer, for the audit entry and the optimistic check of an edit.
func (p DrawingProfile) SpecDigest() (string, error) { return DigestValue(p) }
