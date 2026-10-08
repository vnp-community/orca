package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// projectionSection is one Orca-owned region of a projection. Keys are the top-level members of the
// artifact document it carries; Keys == nil on the last section means "whatever no other section took".
type projectionSection struct {
	Name    string
	Heading string
	Keys    []string
	// Prose sections (no data) are filled from ProjectionMeta.Extra; the plan's phases and tasks come from task-service.
	Prose bool
}

var projectionLayouts = map[ArtifactKind][]projectionSection{
	ArtifactKindSolution: {
		{Name: "summary", Heading: "Summary", Keys: []string{"recommendation"}},
		{Name: "options", Heading: "Options", Keys: []string{"options"}},
		{Name: "requirement-coverage", Heading: "Requirement coverage", Keys: []string{"requirement_coverage"}},
		{Name: "assumptions", Heading: "Assumptions", Keys: []string{"assumptions"}},
		{Name: "open-questions", Heading: "Open questions", Keys: []string{"open_questions"}},
		{Name: "details", Heading: "Details"},
	},
	ArtifactKindPlan: {
		{Name: "goal", Heading: "Goal", Keys: []string{"goal", "scope", "implements", "assumptions", "satisfies_all"}},
		{Name: "phases", Heading: "Phases", Prose: true},
		{Name: "tasks", Heading: "Tasks", Prose: true},
		{Name: "risks", Heading: "Risks", Keys: []string{"risks"}},
		{Name: "rollback", Heading: "Rollback", Keys: []string{"rollback", "verification_strategy"}},
		{Name: "details", Heading: "Details"},
	},
}

func layoutFor(kind ArtifactKind) []projectionSection {
	if l, ok := projectionLayouts[kind]; ok {
		return l
	}
	return []projectionSection{{Name: "document", Heading: "Document"}}
}

// ProjectionMeta is what a projection says about its artifact beyond the document itself.
type ProjectionMeta struct {
	ID          string
	Request     string // e.g. REQ-142@r2
	Status      string
	Supersedes  string
	Title       string
	GeneratedBy GeneratedBy
	// Extra holds the markdown of prose sections by section name (never parsed back).
	Extra map[string]string
}

// RenderArtifact writes the deterministic Markdown/YAML projection of doc. It refuses a document
// that fails its schema, so a projection never carries data the model would not accept.
func RenderArtifact(reg *SchemaRegistry, kind ArtifactKind, doc []byte, meta ProjectionMeta) ([]byte, error) {
	if vs := reg.ValidateDocument(kind, doc); len(vs) > 0 {
		return nil, ErrProjectionDocumentInvalid(vs)
	}
	canon, err := CanonicalJSON(doc)
	if err != nil {
		return nil, err
	}
	var members map[string]json.RawMessage
	if err := json.Unmarshal(canon, &members); err != nil {
		return nil, err
	}
	version, _ := DeclaredSchemaVersion(canon)
	fm := Frontmatter{OrcaSchema: version, Kind: kind, ID: meta.ID, Request: meta.Request, Status: meta.Status,
		Supersedes: meta.Supersedes, Digest: "sha256:" + DigestOfCanonical(canon), GeneratedBy: meta.GeneratedBy}
	head, err := renderFrontmatter(fm)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	b.WriteString(head)
	title := strings.TrimSpace(NormalizeNFC(meta.Title))
	if title == "" {
		title = meta.ID
	}
	b.WriteString("# " + oneLine(title) + "\n")

	taken := map[string]bool{}
	for _, s := range layoutFor(kind) {
		for _, k := range s.Keys {
			taken[k] = true
		}
	}
	for _, s := range layoutFor(kind) {
		keys := s.Keys
		if keys == nil && !s.Prose {
			for k := range members {
				if !taken[k] || len(layoutFor(kind)) == 1 {
					keys = append(keys, k)
				}
			}
			sort.Strings(keys)
		}
		part := map[string]json.RawMessage{}
		for _, k := range keys {
			if v, ok := members[k]; ok {
				part[k] = v
			}
		}
		if len(part) == 0 && !s.Prose {
			continue
		}
		b.WriteString("\n")
		if s.Prose {
			fmt.Fprintf(&b, "<!-- orca:begin %s -->\n## %s\n", s.Name, s.Heading)
			if extra := strings.TrimSpace(NormalizeNFC(meta.Extra[s.Name])); extra != "" {
				b.WriteString(indentProse(extra) + "\n")
			}
			fmt.Fprintf(&b, "<!-- orca:end %s -->\n", s.Name)
			continue
		}
		partJSON, err := marshalPart(part)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&b, "<!-- orca:begin %s digest=sha256:%s -->\n## %s\n", s.Name, DigestOfCanonical(partJSON), s.Heading)
		writeOutline(&b, part)
		fmt.Fprintf(&b, "\n%s\n%s\n```\n<!-- orca:end %s -->\n", orcaJSONFence, partJSON, s.Name)
	}
	return b.Bytes(), nil
}

func ErrProjectionDocumentInvalid(vs []Violation) error {
	return ErrArtifactSchemaInvalid(vs)
}

func renderFrontmatter(fm Frontmatter) (string, error) {
	var b strings.Builder
	line := func(k, v string) {
		if v != "" {
			fmt.Fprintf(&b, "%s: %s\n", k, yamlScalar(v))
		}
	}
	b.WriteString("---\n")
	fmt.Fprintf(&b, "orca_schema: %d\n", fm.OrcaSchema)
	line("kind", string(fm.Kind))
	line("id", fm.ID)
	line("request", fm.Request)
	line("status", fm.Status)
	line("supersedes", fm.Supersedes)
	line("digest", fm.Digest)
	g := fm.GeneratedBy
	if g != (GeneratedBy{}) {
		var parts []string
		add := func(k, v string) {
			if v != "" {
				parts = append(parts, k+": "+yamlScalar(v))
			}
		}
		add("kind", g.Kind)
		add("tool", g.Tool)
		add("model", g.Model)
		add("run", g.Run)
		fmt.Fprintf(&b, "generated_by: {%s}\n", strings.Join(parts, ", "))
	}
	b.WriteString("---\n")
	return b.String(), nil
}

// yamlScalar double-quotes anything that is not a plain identifier-like token, so no value can become an anchor, tag or key.
func yamlScalar(s string) string {
	s = oneLine(NormalizeNFC(s))
	plain := s != ""
	for _, r := range s {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("-_.@#/", r)) {
			plain = false
		}
	}
	if plain && !strings.ContainsAny(s[:1], "-#@") && !isYAMLNumberLike(s) {
		return s
	}
	b, _ := json.Marshal(s)
	return string(b)
}

func isYAMLNumberLike(s string) bool {
	switch strings.ToLower(s) {
	case "true", "false", "null", "yes", "no", "on", "off", "~":
		return true
	}
	for _, r := range s {
		if !(unicode.IsDigit(r) || r == '.' || r == '-' || r == 'e' || r == 'E' || r == '+') {
			return false
		}
	}
	return true
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

func marshalPart(part map[string]json.RawMessage) ([]byte, error) {
	raw, err := json.Marshal(part)
	if err != nil {
		return nil, err
	}
	return CanonicalJSON(raw)
}

// indentProse keeps free text from ever starting a line with a marker or fence.
func indentProse(s string) string {
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	for i, l := range lines {
		lines[i] = "> " + neutralizeMarkers(l)
	}
	return strings.Join(lines, "\n")
}

func neutralizeMarkers(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "<!--", "<!\u200b--"), "```", "`\u200b``")
}

// writeOutline prints a readable bullet outline of part. It is for people only: Parse never reads it.
func writeOutline(b *bytes.Buffer, part map[string]json.RawMessage) {
	keys := make([]string, 0, len(part))
	for k := range part {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		var v any
		_ = json.Unmarshal(part[k], &v)
		writeOutlineValue(b, k, v, 0)
	}
}

func writeOutlineValue(b *bytes.Buffer, label string, v any, depth int) {
	pad := strings.Repeat("  ", depth)
	switch x := v.(type) {
	case map[string]any:
		fmt.Fprintf(b, "%s- %s:\n", pad, neutralizeMarkers(label))
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			writeOutlineValue(b, k, x[k], depth+1)
		}
	case []any:
		fmt.Fprintf(b, "%s- %s:\n", pad, neutralizeMarkers(label))
		for i, e := range x {
			writeOutlineValue(b, fmt.Sprintf("#%d", i+1), e, depth+1)
		}
	case string:
		fmt.Fprintf(b, "%s- %s: %s\n", pad, neutralizeMarkers(label), neutralizeMarkers(oneLine(x)))
	default:
		raw, _ := json.Marshal(x)
		fmt.Fprintf(b, "%s- %s: %s\n", pad, neutralizeMarkers(label), raw)
	}
}

// ParseProjection reads a projection back. Data comes only from the frontmatter and the orca-json block of
// each Orca region; every other line is free text. The result is canonical JSON that passed its schema.
func ParseProjection(reg *SchemaRegistry, md []byte, kind ArtifactKind) ([]byte, []Violation) {
	text, vs := normalizeProjectionInput(md)
	if len(vs) > 0 {
		return nil, vs
	}
	fm, rest, vs := ParseFrontmatter(text)
	if len(vs) > 0 {
		return nil, vs
	}
	if fm.Kind != kind {
		return nil, []Violation{{Line: keyLine(strings.Split(text, "\n")[1:], "kind"), Code: CodeProjectionKindMismatch,
			Message: fmt.Sprintf("the projection is a %s, expected a %s", fm.Kind, kind)}}
	}
	lines := strings.Split(text, "\n")
	restFirst := len(lines) - len(strings.Split(rest, "\n")) + 1 // absolute line of rest[0]
	regions, vs := scanRegions(strings.Split(rest, "\n"), restFirst)
	if len(vs) > 0 {
		return nil, vs
	}
	restLines := strings.Split(rest, "\n")
	merged := map[string]json.RawMessage{}
	keyRegion := map[string]int{}
	var out []Violation
	for _, r := range regions {
		spec, known := sectionByName(kind, r.section)
		if !known || spec.Prose {
			continue
		}
		raw, jsonLine, bvs := extractOrcaJSON(restLines[r.begin+1:r.end], restFirst+r.begin+1)
		if len(bvs) > 0 {
			out = append(out, bvs...)
			continue
		}
		canon, err := CanonicalJSON(raw)
		if err != nil {
			out = append(out, Violation{Line: jsonLine, Code: CodeProjectionJSONBlock, Message: err.Error()})
			continue
		}
		if r.digest != "" && r.digest != "sha256:"+DigestOfCanonical(canon) {
			out = append(out, Violation{Line: restFirst + r.begin, Code: CodeProjectionDigestMismatch, Message: "region " + r.section + " does not match its digest"})
		}
		var part map[string]json.RawMessage
		_ = json.Unmarshal(canon, &part)
		for k, v := range part {
			if _, dup := merged[k]; dup {
				out = append(out, Violation{Line: restFirst + r.begin, Code: CodeProjectionJSONBlock, Message: "member " + k + " appears in more than one region"})
				continue
			}
			merged[k], keyRegion[k] = v, jsonLine
		}
	}
	if len(out) > 0 {
		return nil, out
	}
	if len(merged) == 0 {
		return nil, []Violation{{Line: restFirst, Code: CodeProjectionRegionMissing, Message: "the projection has no Orca data region"}}
	}
	docRaw, err := marshalPart(merged)
	if err != nil {
		return nil, []Violation{{Code: CodeProjectionJSONBlock, Message: err.Error()}}
	}
	for _, v := range reg.ValidateDocument(kind, docRaw) {
		v.Line = lineForPath(v.Path, keyRegion, restFirst)
		out = append(out, v)
	}
	if len(out) > 0 {
		return nil, out
	}
	if fm.Digest != "" && fm.Digest != "sha256:"+DigestOfCanonical(docRaw) {
		return nil, []Violation{{Line: keyLine(lines[1:], "digest"), Code: CodeProjectionDigestMismatch, Message: "the frontmatter digest does not match the Orca regions"}}
	}
	return docRaw, nil
}

func sectionByName(kind ArtifactKind, name string) (projectionSection, bool) {
	for _, s := range layoutFor(kind) {
		if s.Name == name {
			return s, true
		}
	}
	return projectionSection{}, false
}

func lineForPath(path string, keyRegion map[string]int, fallback int) int {
	first := strings.TrimPrefix(path, "/")
	if i := strings.Index(first, "/"); i >= 0 {
		first = first[:i]
	}
	if l, ok := keyRegion[first]; ok {
		return l
	}
	return fallback
}

// normalizeProjectionInput bounds size, folds \r\n to \n and refuses control characters (it never strips them).
func normalizeProjectionInput(md []byte) (string, []Violation) {
	if len(md) > MaxProjectionBytes {
		return "", []Violation{{Line: 1, Code: CodeProjectionTooLarge, Message: fmt.Sprintf("a projection is at most %d bytes", MaxProjectionBytes)}}
	}
	text := strings.ReplaceAll(string(md), "\r\n", "\n")
	line := 1
	for _, r := range text {
		switch {
		case r == '\n':
			line++
		case r == '\t':
		case unicode.IsControl(r):
			return "", []Violation{{Line: line, Code: CodeProjectionControlChar, Message: fmt.Sprintf("control character U+%04X is not allowed", r)}}
		}
	}
	return NormalizeNFC(text), nil
}
