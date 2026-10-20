package domain

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// DesignModel is the design extracted once from the knowledge graph and the SAD, after Orca checked it and merged the edges
// its own parser read from mermaid blocks. It is what the design lock stores and what both phases of a review reuse.
type DesignModel struct {
	Available  bool              `json:"available"`
	Snapshot   string            `json:"snapshot,omitempty"`
	DocSha256  map[string]string `json:"docSha256,omitempty"`
	Elements   []ContextElement  `json:"elements,omitempty"`
	Containers []DesignContainer `json:"containers,omitempty"`
	// Components maps the id of a folded component to the container it is part of.
	Components map[string]string `json:"components,omitempty"`
	Edges      []DesignEdge      `json:"edges,omitempty"`
	Flows      []DesignFlow      `json:"flows,omitempty"`
	Warnings   []string          `json:"warnings,omitempty"`
}

var containerKinds = map[string]bool{"service": true, "app": true, "datastore": true, "queue": true, "agent": true, "external": true}

// ValidContainerKind says whether kind is one of the six kinds of a container.
func ValidContainerKind(kind string) bool { return containerKinds[kind] }

var (
	nonAlnum        = regexp.MustCompile(`[^a-z0-9]+`)
	componentSuffix = regexp.MustCompile(`^[A-Z][a-z0-9]+([A-Z][a-z0-9]+)*(Handler|Controller|Repository|Usecase|UseCase|Interactor|Mapper|Adapter|Factory|Validator)$`)
	fileExtension   = regexp.MustCompile(`\.(go|ts|tsx|js|py|java|kt|rb|rs|sql|proto)$`)
)

// Kebab turns a label into the lowercase, dash-separated form used in generated ids.
func Kebab(label string) string {
	return strings.Trim(nonAlnum.ReplaceAllString(strings.ToLower(NormalizeNFC(label)), "-"), "-")
}

// labelKey is the form two labels are compared in: lowercase letters and digits, without the ms- prefix services carry.
func labelKey(s string) string {
	s = strings.ToLower(NormalizeNFC(s))
	s = strings.TrimPrefix(strings.TrimSpace(s), "ms-")
	return nonAlnum.ReplaceAllString(s, "")
}

// LooksLikeComponent catches what a container diagram must not draw: a function, a file, a path or a class. A name that could
// still be a container passes; the prompt does the rest.
func LooksLikeComponent(label string) bool {
	l := strings.TrimSpace(label)
	switch {
	case strings.Contains(l, "()"), strings.Contains(l, "::"), strings.Contains(l, "internal/"), strings.Contains(l, "pkg/"), strings.Contains(l, "src/"):
		return true
	case fileExtension.MatchString(l):
		return true
	case !strings.ContainsAny(l, " -_") && componentSuffix.MatchString(l):
		return true
	}
	return false
}

// Match finds the container a label, a graph id or a mermaid id stands for.
func (d DesignModel) Match(label, kgID string) (DesignContainer, bool) {
	if kgID != "" {
		for _, c := range d.Containers {
			if c.ID == kgID || (c.KGID != nil && *c.KGID == kgID) {
				return c, true
			}
		}
	}
	key := labelKey(label)
	if key == "" {
		return DesignContainer{}, false
	}
	for _, c := range d.Containers {
		if labelKey(c.Label) == key || labelKey(c.ID) == key {
			return c, true
		}
		for _, a := range c.Aliases {
			if labelKey(a) == key || strings.EqualFold(a, label) {
				return c, true
			}
		}
	}
	best, bestGap := DesignContainer{}, -1
	for _, c := range d.Containers {
		ck := labelKey(c.Label)
		short, long := ck, key
		if len(short) > len(long) {
			short, long = long, short
		}
		if len(short) < 4 || !strings.Contains(long, short) {
			continue
		}
		if gap := len(long) - len(short); bestGap < 0 || gap < bestGap || (gap == bestGap && c.ID < best.ID) {
			best, bestGap = c, gap
		}
	}
	return best, bestGap >= 0
}

// ContainerByID returns the container with the id.
func (d DesignModel) ContainerByID(id string) (DesignContainer, bool) {
	for _, c := range d.Containers {
		if c.ID == id {
			return c, true
		}
	}
	return DesignContainer{}, false
}

// HasEdge reports whether the design has a dependency from one container to another.
func (d DesignModel) HasEdge(from, to string) bool {
	for _, e := range d.Edges {
		if e.From == from && e.To == to {
			return true
		}
	}
	return false
}

// HasCall reports whether any designed flow has a message between the two containers.
func (d DesignModel) HasCall(from, to string) bool {
	for _, f := range d.Flows {
		for _, m := range f.Messages {
			if m.From == from && m.To == to {
				return true
			}
		}
	}
	return false
}

// ReviewStateOf is the state of the element behind a container or an id, unknown when none was declared.
func (d DesignModel) ReviewStateOf(id string) KGReviewState {
	for _, e := range d.Elements {
		if e.ID == id {
			return e.ReviewState
		}
	}
	if c, ok := d.ContainerByID(id); ok && c.ReviewState != "" {
		return c.ReviewState
	}
	return KGUnknown
}

// ReviewedRatio is the share of the graph elements read that a person approved; 0 when none was read.
func (d DesignModel) ReviewedRatio() float64 {
	if len(d.Elements) == 0 {
		return 0
	}
	n := 0
	for _, e := range d.Elements {
		if e.ReviewState == KGApproved {
			n++
		}
	}
	return float64(n) / float64(len(d.Elements))
}

// Checkable says what the design can be compared with: edges for the architecture, messages for the flow.
func (d DesignModel) Checkable() (architecture, flow bool) {
	if !d.Available {
		return false, false
	}
	calls := 0
	for _, f := range d.Flows {
		calls += len(f.Messages)
	}
	return len(d.Edges) > 0, calls > 0
}

// NormalizeDesign checks the design an agent reported and merges the edges Orca read from the mermaid blocks. A violation is
// something the agent can fix; a warning is something Orca fixed or skipped.
func NormalizeDesign(kg ContextKG, in ContextDesign, redact func(string) string) (DesignModel, ContextViolations, []string) {
	var viol ContextViolations
	var warn []string
	out := DesignModel{Available: in.Available && kg.Available, Snapshot: strings.TrimSpace(kg.Snapshot)}
	if !kg.Available {
		if in.Available || len(in.Containers) > 0 {
			warn = append(warn, "design_ignored:kg_unavailable")
		}
		return out, nil, warn
	}
	if !in.Available {
		return out, nil, warn
	}

	docs, v := normalizeDocs(in.DocSha256)
	viol = append(viol, v...)
	out.DocSha256 = docs
	if len(in.ElementsRead) > MaxContextElements {
		viol = append(viol, fmt.Sprintf("design.elementsRead has %d entries; the limit is %d", len(in.ElementsRead), MaxContextElements))
		return out, viol, warn
	}
	read := map[string]bool{}
	for _, e := range in.ElementsRead {
		if !ValidKGID(e.ID) {
			viol = append(viol, fmt.Sprintf("design.elementsRead id %q is not a knowledge graph id", e.ID))
			continue
		}
		if e.ReviewState != KGApproved && e.ReviewState != KGNeedsReview {
			e.ReviewState = KGUnknown
		}
		if !read[e.ID] {
			read[e.ID] = true
			out.Elements = append(out.Elements, ContextElement{ID: e.ID, Class: clipRunes(e.Class, 40), ReviewState: e.ReviewState})
		}
	}
	for _, id := range kg.Read {
		if ValidKGID(id) {
			read[id] = true
		}
	}
	sort.Slice(out.Elements, func(i, j int) bool { return out.Elements[i].ID < out.Elements[j].ID })

	if len(in.Containers) > MaxContextContainers || len(in.Edges) > MaxContextDesignEdges || len(in.Flows) > MaxContextFlows || len(in.SourceBlocks) > MaxContextBlocks {
		viol = append(viol, "design has more containers, edges, flows or source blocks than the limits allow")
		return out, viol, warn
	}
	ev := EvidenceContext{KGRead: read, Docs: docs}
	clip := func(s string, n int) string { return clipRunes(applyRedact(redact, s), n) }

	seen := map[string]bool{}
	byAgentID := map[string]string{} // the id the agent used -> the id Orca assigns
	for _, c := range in.Containers {
		label := clip(c.Label, 40)
		switch {
		case label == "":
			viol = append(viol, "a design container has no label")
			continue
		case !ValidContainerKind(c.Kind):
			viol = append(viol, fmt.Sprintf("container %q: kind %q must be one of service, app, datastore, queue, agent, external", label, c.Kind))
			continue
		case LooksLikeComponent(label):
			viol = append(viol, fmt.Sprintf("container %q looks like a component (a class, function or file); a container diagram shows applications, data stores and systems", label))
			continue
		}
		dc := DesignContainer{Label: label, Kind: c.Kind, Tech: clip(c.Tech, 40), Aliases: cleanAliases(c.Aliases), Parent: strings.TrimSpace(c.Parent)}
		switch c.Source {
		case SourceKG:
			if c.KGID == nil || !read[*c.KGID] {
				viol = append(viol, fmt.Sprintf("container %q: kgId must be an element listed in design.elementsRead", label))
				continue
			}
			id := *c.KGID
			dc.ID, dc.KGID, dc.Source, dc.ReviewState = id, &id, SourceKG, out.reviewStateFrom(id, c.ReviewState)
		case SourceSADText:
			id := strings.TrimSpace(c.ID)
			if !strings.HasPrefix(id, "sad:") {
				id = "sad:" + strings.TrimPrefix(Kebab(firstNonEmpty(id, label)), "sad-")
			}
			if len(id) <= 4 || len(id) > 60 {
				viol = append(viol, fmt.Sprintf("container %q: id %q is not usable", label, id))
				continue
			}
			kept, probs := ev.VerifiedEvidence(onlyKind(c.Evidence, EvidenceSource))
			if len(kept) == 0 {
				viol = append(viol, fmt.Sprintf("container %q has source sad-text but no source evidence pointing at a document listed in design.docSha256 (%s)", label, strings.Join(probs, "; ")))
				continue
			}
			dc.ID, dc.Source, dc.Evidence, dc.ReviewState = id, SourceSADText, capEvidence(kept), KGUnknown
		default:
			viol = append(viol, fmt.Sprintf("container %q: source must be kg or sad-text in a design", label))
			continue
		}
		if seen[dc.ID] {
			continue
		}
		seen[dc.ID] = true
		byAgentID[c.ID] = dc.ID
		byAgentID[dc.ID] = dc.ID
		out.Containers = append(out.Containers, dc)
	}
	if len(viol) > 0 {
		return out, viol, warn
	}

	out.foldComponents()
	resolve := func(ref string) (string, bool) {
		if id, ok := byAgentID[ref]; ok {
			return id, true
		}
		if parent, ok := out.Components[ref]; ok {
			return parent, true
		}
		if c, ok := out.Match(ref, ref); ok {
			return c.ID, true
		}
		return "", false
	}

	parsed := out.readBlocks(in.SourceBlocks, &warn)
	agentEdges, agentFlows, v := normalizeAgentDesign(in, resolve, ev, clip)
	viol = append(viol, v...)
	out.mergeEdges(agentEdges, parsed, &warn)
	out.mergeFlows(agentFlows, parsed, &warn)

	sort.Slice(out.Containers, func(i, j int) bool { return out.Containers[i].ID < out.Containers[j].ID })
	out.Warnings = dedupeSorted(warn)
	return out, viol, out.Warnings
}

func (d DesignModel) reviewStateFrom(id string, declared KGReviewState) KGReviewState {
	for _, e := range d.Elements {
		if e.ID == id {
			return e.ReviewState // the state read from the graph wins over what the container says
		}
	}
	if declared == KGApproved || declared == KGNeedsReview {
		return declared
	}
	return KGUnknown
}

func (d *DesignModel) foldComponents() {
	ids := map[string]bool{}
	for _, c := range d.Containers {
		ids[c.ID] = true
	}
	kept := d.Containers[:0:0]
	for _, c := range d.Containers {
		if c.Parent != "" && c.Parent != c.ID && ids[c.Parent] {
			if d.Components == nil {
				d.Components = map[string]string{}
			}
			d.Components[c.ID] = c.Parent
			continue
		}
		c.Parent = ""
		kept = append(kept, c)
	}
	d.Containers = kept
}

func normalizeDocs(in map[string]string) (map[string]string, ContextViolations) {
	var viol ContextViolations
	if len(in) > MaxContextDocs {
		return nil, ContextViolations{fmt.Sprintf("design.docSha256 names %d documents; the limit is %d", len(in), MaxContextDocs)}
	}
	out := map[string]string{}
	for doc, sum := range in {
		sum = strings.ToLower(strings.TrimSpace(sum))
		if normalizeRel(doc) == "" || !sha256Hex.MatchString(sum) {
			viol = append(viol, fmt.Sprintf("design.docSha256[%q] must map a repository path to a 64 character sha256", doc))
			continue
		}
		out[normalizeRel(doc)] = sum
	}
	return out, viol
}

func cleanAliases(in []string) []string {
	var out []string
	for _, a := range in {
		if a = clipRunes(a, 40); a != "" {
			out = append(out, a)
		}
		if len(out) == 8 {
			break
		}
	}
	return out
}

func onlyKind(in []ReviewEvidence, kind EvidenceKind) []ReviewEvidence {
	var out []ReviewEvidence
	for _, e := range in {
		if e.Kind == kind {
			out = append(out, e)
		}
	}
	return out
}

func capEvidence(in []ReviewEvidence) []ReviewEvidence {
	if len(in) > maxContextEvidencePerX {
		return in[:maxContextEvidencePerX]
	}
	return in
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// parsedBlock is one mermaid block with its nodes mapped to containers.
type parsedBlock struct {
	doc, section string
	diagram      MermaidDiagram
	// container maps a mermaid node id to the container it stands for ("" when it stands for none).
	container map[string]string
}

// readBlocks parses the mermaid blocks the agent copied from the SAD and maps their nodes to containers, adding a container
// for a node that stands for none. A block that is not mermaid, not a flowchart or sequence, or not from a document the agent
// listed is skipped with a warning.
func (d *DesignModel) readBlocks(blocks []SourceBlock, warn *[]string) []parsedBlock {
	var out []parsedBlock
	for _, b := range blocks {
		doc := normalizeRel(b.Doc)
		if b.Kind != "mermaid" {
			continue
		}
		if d.DocSha256[doc] == "" {
			*warn = append(*warn, "design_block_ignored:unlisted_doc:"+b.Section)
			continue
		}
		if len(b.Text) > MaxContextBlockBytes {
			*warn = append(*warn, "design_block_ignored:too_large:"+b.Section)
			continue
		}
		diagram, err := ParseMermaid(b.Text)
		if err != nil {
			*warn = append(*warn, "design_block_ignored:"+b.Section+":"+strings.ReplaceAll(err.Error(), " ", "_"))
			continue
		}
		for _, w := range diagram.Warnings {
			*warn = append(*warn, fmt.Sprintf("%s:%s", w, b.Section))
		}
		pb := parsedBlock{doc: doc, section: b.Section, diagram: diagram, container: map[string]string{}}
		groups := map[string]string{}
		for _, n := range diagram.Nodes {
			if n.Group != "" {
				if c, ok := d.Match(n.Group, ""); ok {
					groups[n.ID] = c.ID
				}
			}
		}
		for _, n := range diagram.Nodes {
			if c, ok := d.Match(n.Label, ""); ok {
				pb.container[n.ID] = c.ID
				continue
			}
			if c, ok := d.Match(n.ID, ""); ok {
				pb.container[n.ID] = c.ID
				continue
			}
			if parent := groups[n.ID]; parent != "" {
				pb.container[n.ID] = parent // a part inside a container: its calls are the container's
				continue
			}
		}
		if diagram.Kind == MermaidFlowchart {
			d.addUnmatchedNodes(&pb, warn)
		}
		out = append(out, pb)
	}
	return out
}

// addUnmatchedNodes adds a container for a node that is a data store, a queue or an outside system and has no match.
func (d *DesignModel) addUnmatchedNodes(pb *parsedBlock, warn *[]string) {
	for _, n := range pb.diagram.Nodes {
		if pb.container[n.ID] != "" || n.Group != "" {
			continue
		}
		kind := map[string]string{"datastore": "datastore", "queue": "queue"}[n.Shape]
		if kind == "" {
			kind = "service"
		}
		id := "sad:" + Kebab(n.ID)
		label := clipRunes(n.Label, 40)
		if LooksLikeComponent(label) || label == "" {
			continue
		}
		c := DesignContainer{
			ID: id, Source: SourceSADText, Label: label, Kind: kind, ReviewState: KGUnknown,
			Evidence: []ReviewEvidence{{Kind: EvidenceSource, Ref: pb.doc, Doc: pb.doc, Section: pb.section}},
		}
		d.Containers = append(d.Containers, c)
		pb.container[n.ID] = id
		*warn = append(*warn, "design_container_from_parser:"+n.ID)
	}
}

// normalizeAgentDesign maps the edges and flows an agent reported onto the container ids Orca assigned.
func normalizeAgentDesign(in ContextDesign, resolve func(string) (string, bool), ev EvidenceContext, clip func(string, int) string) ([]DesignEdge, []DesignFlow, ContextViolations) {
	var viol ContextViolations
	var edges []DesignEdge
	for _, e := range in.Edges {
		from, ok1 := resolve(e.From)
		to, ok2 := resolve(e.To)
		switch {
		case !ok1 || !ok2:
			viol = append(viol, fmt.Sprintf("design edge %q -> %q names a container that is not in design.containers", e.From, e.To))
			continue
		case from == to:
			viol = append(viol, fmt.Sprintf("design edge %q -> %q is a container calling itself", e.From, e.To))
			continue
		}
		kept, _ := ev.VerifiedEvidence(e.Evidence)
		edges = append(edges, DesignEdge{From: from, To: to, Protocol: strings.ToLower(clip(e.Protocol, 20)), Label: clip(e.Label, 40), Source: SourceSADText, Evidence: capEvidence(kept)})
	}
	var flows []DesignFlow
	for _, f := range in.Flows {
		flow := DesignFlow{Name: clip(f.Name, 60)}
		if flow.Name == "" {
			viol = append(viol, "a design flow has no name")
			continue
		}
		for _, m := range f.Messages {
			from, ok1 := resolve(m.From)
			to, ok2 := resolve(m.To)
			switch {
			case !ok1 || !ok2:
				viol = append(viol, fmt.Sprintf("flow %q: message %q -> %q names a container that is not in design.containers", flow.Name, m.From, m.To))
				continue
			case from == to:
				viol = append(viol, fmt.Sprintf("flow %q: message %q -> %q is a container calling itself", flow.Name, m.From, m.To))
				continue
			}
			flow.Messages = append(flow.Messages, DesignMessage{From: from, To: to, Protocol: strings.ToLower(clip(m.Protocol, 20)), Label: clip(m.Label, 40), Order: m.Order})
		}
		kept, _ := ev.VerifiedEvidence(f.Evidence)
		flow.Evidence = capEvidence(kept)
		sort.SliceStable(flow.Messages, func(i, j int) bool { return flow.Messages[i].Order < flow.Messages[j].Order })
		for i := range flow.Messages {
			flow.Messages[i].Order = i + 1
		}
		flows = append(flows, flow)
	}
	return edges, flows, viol
}

// mergeEdges lets the edges Orca parsed win over the ones the agent reported for the same block, and keeps the agent's edges
// that no parsed block speaks for.
func (d *DesignModel) mergeEdges(agent []DesignEdge, blocks []parsedBlock, warn *[]string) {
	type key struct{ from, to string }
	parsed := map[key]DesignEdge{}
	covered := map[string]map[string]bool{} // block section -> containers the block mentions
	citedBy := map[string]bool{}            // block sections an agent edge cites
	for _, b := range blocks {
		if b.diagram.Kind != MermaidFlowchart {
			continue
		}
		set := map[string]bool{}
		for _, c := range b.container {
			set[c] = true
		}
		covered[b.section] = set
		for _, e := range b.diagram.Edges {
			from, to := b.container[e.From], b.container[e.To]
			if from == "" || to == "" || from == to {
				continue
			}
			de := DesignEdge{From: from, To: to, Label: clipRunes(e.Label, 40), Source: SourceSADText,
				Evidence: []ReviewEvidence{{Kind: EvidenceSource, Ref: b.doc, Doc: b.doc, Section: b.section}}}
			parsed[key{from, to}] = de
			if e.Both {
				parsed[key{to, from}] = DesignEdge{From: to, To: from, Label: de.Label, Source: SourceSADText, Evidence: de.Evidence}
			}
		}
	}
	var out []DesignEdge
	have := map[key]bool{}
	for _, e := range agent {
		cites := ""
		for _, ev := range e.Evidence {
			if ev.Kind == EvidenceSource && covered[ev.Section] != nil {
				cites = ev.Section
				citedBy[cites] = true
			}
		}
		_, inParsed := parsed[key{e.From, e.To}]
		if cites != "" && !inParsed && covered[cites][e.From] && covered[cites][e.To] {
			*warn = append(*warn, fmt.Sprintf("design_edge_mismatch:%s->%s", e.From, e.To))
			continue // the block says there is no such edge; the parser wins
		}
		if inParsed {
			p := parsed[key{e.From, e.To}]
			if p.Protocol == "" {
				p.Protocol = e.Protocol
			}
			parsed[key{e.From, e.To}] = p
			continue
		}
		if !have[key{e.From, e.To}] {
			have[key{e.From, e.To}] = true
			out = append(out, e)
		}
	}
	keys := make([]key, 0, len(parsed))
	for k := range parsed {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].from != keys[j].from {
			return keys[i].from < keys[j].from
		}
		return keys[i].to < keys[j].to
	})
	agentHas := map[key]bool{}
	for _, e := range agent {
		agentHas[key{e.From, e.To}] = true
	}
	for _, k := range keys {
		if !have[k] {
			have[k] = true
			out = append(out, parsed[k])
		}
		if !agentHas[k] && len(citedBy) > 0 {
			for _, ev := range parsed[k].Evidence {
				if citedBy[ev.Section] {
					*warn = append(*warn, fmt.Sprintf("design_edge_mismatch:+%s->%s", k.from, k.to))
				}
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].From != out[j].From {
			return out[i].From < out[j].From
		}
		return out[i].To < out[j].To
	})
	d.Edges = out
}

// mergeFlows adds the flows Orca read from sequence blocks; a flow the agent reported for the same section is replaced.
func (d *DesignModel) mergeFlows(agent []DesignFlow, blocks []parsedBlock, warn *[]string) {
	replaced := map[string]bool{}
	var parsedFlows []DesignFlow
	for _, b := range blocks {
		if b.diagram.Kind != MermaidSequence {
			continue
		}
		flow := DesignFlow{Name: "Flow " + b.section, Evidence: []ReviewEvidence{{Kind: EvidenceSource, Ref: b.doc, Doc: b.doc, Section: b.section}}}
		for _, m := range b.diagram.Messages {
			from, to := b.container[m.From], b.container[m.To]
			if from == "" || to == "" || from == to {
				if from == "" || to == "" {
					*warn = append(*warn, "design_flow_participant_unmapped:"+b.section)
				}
				continue
			}
			flow.Messages = append(flow.Messages, DesignMessage{From: from, To: to, Label: clipRunes(m.Label, 40), Order: len(flow.Messages) + 1})
		}
		if len(flow.Messages) == 0 {
			continue
		}
		parsedFlows = append(parsedFlows, flow)
		replaced[b.section] = true
	}
	var out []DesignFlow
	for _, f := range agent {
		section := ""
		for _, ev := range f.Evidence {
			if ev.Kind == EvidenceSource && replaced[ev.Section] {
				section = ev.Section
			}
		}
		if section != "" {
			*warn = append(*warn, "design_flow_replaced_by_parser:"+section)
			continue
		}
		out = append(out, f)
	}
	d.Flows = append(out, parsedFlows...)
}

// DesignInputDigest is the digest of a design as locked: independent of time, of the run that extracted it and of warnings.
func DesignInputDigest(d DesignModel) (string, error) {
	d.Warnings = nil
	return DigestValue(d)
}
