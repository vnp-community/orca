package domain

import (
	"fmt"
	"sort"
	"strings"
)

type DiagramType string

const (
	DiagramArchitecture DiagramType = "architecture-delta"
	DiagramSequence     DiagramType = "sequence"
	DiagramStateMachine DiagramType = "state-machine"
	DiagramDatabase     DiagramType = "database-schema"
)

// DiagramVariantDesign and DiagramVariantDelta name the two views of a section: what was designed, and what the code changed.
const (
	DiagramVariantDesign = "design"
	DiagramVariantDelta  = "delta"
)

const (
	LevelContainer = "container"

	MaxDiagramNodes     = 12
	MaxDiagramLanes     = 6
	MaxDiagramRawNodes  = 60
	MaxDiagramRawEdges  = 200
	MaxDiagramLabelRune = 40
	moreNodeID          = "more"
)

// DiagramType returns the type a section is drawn as.
func DiagramTypeFor(s ReviewSection) (DiagramType, bool) {
	switch s {
	case SectionArchitecture:
		return DiagramArchitecture, true
	case SectionFlow:
		return DiagramSequence, true
	case SectionBehavior:
		return DiagramStateMachine, true
	case SectionData:
		return DiagramDatabase, true
	}
	return "", false
}

type DiagramDrilldown struct {
	Lens        string `json:"lens"`
	ContainerID string `json:"containerId"`
}

type DiagramColumn struct {
	Name   string    `json:"name"`
	Type   string    `json:"type,omitempty"`
	InBase bool      `json:"inBase"`
	InHead bool      `json:"inHead"`
	State  EdgeState `json:"state,omitempty"`
}

// DiagramNode is a container, a lane of a sequence, a state or a table. Members an agent must not decide (State, Unverified,
// Drilldown) are recomputed on validation.
type DiagramNode struct {
	ID                string            `json:"id"`
	KGID              string            `json:"kgId,omitempty"`
	Source            ComponentSource   `json:"source,omitempty"`
	ReviewState       KGReviewState     `json:"reviewState,omitempty"`
	Label             string            `json:"label"`
	Kind              string            `json:"kind,omitempty"`
	Tech              string            `json:"tech,omitempty"`
	Parent            string            `json:"parent,omitempty"`
	InBase            bool              `json:"inBase"`
	InHead            bool              `json:"inHead"`
	ComponentsChanged int               `json:"componentsChanged,omitempty"`
	Drilldown         *DiagramDrilldown `json:"drilldown,omitempty"`
	Columns           []DiagramColumn   `json:"columns,omitempty"`
	Evidence          []ReviewEvidence  `json:"evidence,omitempty"`
	State             EdgeState         `json:"state,omitempty"`
	Unverified        bool              `json:"unverified,omitempty"`
}

// DiagramEdge is a dependency, a message, a transition or a relation.
type DiagramEdge struct {
	From       string           `json:"from"`
	To         string           `json:"to"`
	Protocol   string           `json:"protocol,omitempty"`
	Label      string           `json:"label,omitempty"`
	Order      int              `json:"order,omitempty"`
	BaseOrder  int              `json:"baseOrder,omitempty"`
	Reordered  bool             `json:"reordered,omitempty"`
	InBase     bool             `json:"inBase"`
	InHead     bool             `json:"inHead"`
	InDesign   *bool            `json:"inDesign"`
	Change     EdgeState        `json:"change,omitempty"`
	State      EdgeState        `json:"state,omitempty"`
	Advisory   bool             `json:"advisory,omitempty"`
	Unverified bool             `json:"unverified,omitempty"`
	Evidence   []ReviewEvidence `json:"evidence,omitempty"`
}

// DiagramModel is the data a drawing is made from. It is small and structured; the picture is a separate step.
type DiagramModel struct {
	Section     ReviewSection `json:"section"`
	Type        DiagramType   `json:"type"`
	Variant     string        `json:"variant"`
	Level       string        `json:"level,omitempty"`
	Name        string        `json:"name,omitempty"`
	Nodes       []DiagramNode `json:"nodes"`
	Edges       []DiagramEdge `json:"edges"`
	Truncated   bool          `json:"truncated,omitempty"`
	InputDigest string        `json:"inputDigest,omitempty"`
}

// ComputeInputDigest sets InputDigest to the digest of the model without it: no time, no snapshot, no run id.
func (m DiagramModel) ComputeInputDigest() (string, error) {
	m.InputDigest = ""
	return DigestValue(m)
}

// Ref is the entry of the review that points at this model.
func (m DiagramModel) Ref() DiagramRef {
	return DiagramRef{Section: m.Section, Variant: m.Variant, Name: m.Name, Type: string(m.Type), InputDigest: m.InputDigest, Status: "missing"}
}

// DiagramValidation is what the checks need besides the diagram.
type DiagramValidation struct {
	Mode     ContextMode
	Design   DesignModel
	Evidence EvidenceContext
	Redact   func(string) string
}

// ValidateDiagrams checks the diagrams an agent returned and derives their states. Violations are what the agent can fix on its
// one retry; warnings are what Orca dropped or corrected.
func ValidateDiagrams(raw []DiagramModel, v DiagramValidation) ([]DiagramModel, ContextViolations, []string) {
	var viol ContextViolations
	var warn []string
	if len(raw) > MaxContextDiagrams {
		return nil, ContextViolations{fmt.Sprintf("%d diagrams were returned; the limit is %d", len(raw), MaxContextDiagrams)}, nil
	}
	var out []DiagramModel
	seen := map[string]bool{}
	for i, m := range raw {
		label := fmt.Sprintf("diagram %d (%s %s)", i+1, m.Section, m.Variant)
		want, ok := DiagramTypeFor(m.Section)
		switch {
		case !ok:
			viol = append(viol, fmt.Sprintf("%s: section %q is not drawn", label, m.Section))
			continue
		case m.Type != want:
			viol = append(viol, fmt.Sprintf("%s: a %s diagram has type %q, not %q", label, m.Section, want, m.Type))
			continue
		case m.Variant != DiagramVariantDesign && m.Variant != DiagramVariantDelta:
			viol = append(viol, fmt.Sprintf("%s: variant must be design or delta", label))
			continue
		}
		if m.Variant == DiagramVariantDelta && v.Mode != ContextModeDelivered {
			warn = append(warn, fmt.Sprintf("diagram_ignored:%s:delta_in_design_mode", m.Section))
			continue
		}
		if m.Variant == DiagramVariantDesign && (m.Section == SectionArchitecture || m.Section == SectionFlow) {
			warn = append(warn, fmt.Sprintf("diagram_ignored:%s:design_is_built_by_orca", m.Section))
			continue
		}
		if len(m.Nodes) > MaxDiagramRawNodes || len(m.Edges) > MaxDiagramRawEdges {
			viol = append(viol, fmt.Sprintf("%s: %d nodes and %d edges are over the limits %d and %d", label, len(m.Nodes), len(m.Edges), MaxDiagramRawNodes, MaxDiagramRawEdges))
			continue
		}
		var res DiagramModel
		var vs ContextViolations
		var ws []string
		if m.Section == SectionArchitecture || m.Section == SectionFlow {
			res, vs, ws = validateContainerDiagram(m, v, label)
		} else {
			res, vs, ws = validateSimpleDiagram(m, v, label)
		}
		warn = append(warn, ws...)
		if len(vs) > 0 {
			viol = append(viol, vs...)
			continue
		}
		key := string(res.Section) + "|" + res.Variant + "|" + res.Name
		if seen[key] {
			viol = append(viol, fmt.Sprintf("%s: a second diagram for the same section, variant and name", label))
			continue
		}
		seen[key] = true
		d, err := res.ComputeInputDigest()
		if err != nil {
			viol = append(viol, label+": "+err.Error())
			continue
		}
		res.InputDigest = d
		out = append(out, res)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Section != b.Section {
			return sectionOrder(a.Section) < sectionOrder(b.Section)
		}
		if a.Variant != b.Variant {
			return a.Variant < b.Variant
		}
		return a.Name < b.Name
	})
	return out, viol, warn
}

func cleanProtocol(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	return clipRunes(s, 20)
}

// validateContainerDiagram checks an architecture or flow diagram, which is drawn at the container level.
func validateContainerDiagram(m DiagramModel, v DiagramValidation, label string) (DiagramModel, ContextViolations, []string) {
	var viol ContextViolations
	var warn []string
	if m.Level != LevelContainer {
		return DiagramModel{}, ContextViolations{fmt.Sprintf("%s: level must be %q; a component is not drawn here", label, LevelContainer)}, nil
	}
	clip := func(s string, n int) string { return clipRunes(applyRedact(v.Redact, s), n) }
	designed := v.Design.Available
	idMap := map[string]string{} // id the agent used -> id assigned
	nodes := map[string]DiagramNode{}
	var folded []foldedNode
	for _, n := range m.Nodes {
		lbl := clip(n.Label, MaxDiagramLabelRune)
		switch {
		case lbl == "":
			viol = append(viol, fmt.Sprintf("%s: a node has no label", label))
			continue
		case !ValidContainerKind(n.Kind):
			viol = append(viol, fmt.Sprintf("%s: node %q has kind %q; use service, app, datastore, queue, agent or external", label, lbl, n.Kind))
			continue
		case LooksLikeComponent(lbl):
			viol = append(viol, fmt.Sprintf("%s: node %q looks like a component; fold it into its container with parent", label, lbl))
			continue
		}
		node := DiagramNode{Label: lbl, Kind: n.Kind, Tech: clip(n.Tech, 40), InBase: n.InBase, InHead: n.InHead, Parent: strings.TrimSpace(n.Parent), ComponentsChanged: clampInt(n.ComponentsChanged, 0, 999)}
		if n.Parent != "" {
			folded = append(folded, withOriginalID(node, n.ID))
			continue
		}
		if c, ok := v.Design.Match(lbl, n.KGID); ok && designed {
			node.ID, node.Source, node.ReviewState = c.ID, c.Source, v.Design.ReviewStateOf(c.ID)
			if c.KGID != nil {
				node.KGID = *c.KGID
			}
		} else {
			switch n.Source {
			case SourceKG:
				if n.KGID == "" || !v.Evidence.KGRead[n.KGID] {
					viol = append(viol, fmt.Sprintf("%s: node %q: kgId %q is not an element listed in design.elementsRead", label, lbl, n.KGID))
					continue
				}
				node.ID, node.KGID, node.Source, node.ReviewState = n.KGID, n.KGID, SourceKG, v.Design.ReviewStateOf(n.KGID)
			case SourceSADText:
				kept, probs := v.Evidence.VerifiedEvidence(onlyKind(n.Evidence, EvidenceSource))
				if len(kept) == 0 {
					viol = append(viol, fmt.Sprintf("%s: node %q has source sad-text but no source evidence in a listed document (%s)", label, lbl, strings.Join(probs, "; ")))
					continue
				}
				id := n.ID
				if !strings.HasPrefix(id, "sad:") {
					id = "sad:" + Kebab(firstNonEmpty(id, lbl))
				}
				node.ID, node.Source, node.Evidence, node.ReviewState = id, SourceSADText, capEvidence(kept), KGUnknown
			case SourceCode:
				node.ID, node.Source, node.ReviewState = "code:"+Kebab(lbl), SourceCode, KGUnknown
				if Kebab(lbl) == "" {
					viol = append(viol, fmt.Sprintf("%s: node %q cannot get an id", label, lbl))
					continue
				}
			default:
				viol = append(viol, fmt.Sprintf("%s: node %q: source must be kg, sad-text or code", label, lbl))
				continue
			}
		}
		if _, dup := nodes[node.ID]; dup {
			idMap[n.ID] = node.ID
			continue
		}
		idMap[n.ID], idMap[node.ID] = node.ID, node.ID
		nodes[node.ID] = node
	}
	if len(viol) > 0 {
		return DiagramModel{}, viol, warn
	}

	// A component folds into its container: it becomes a count on the container and its calls become the container's.
	for _, f := range folded {
		parent := resolveParent(f.Parent, nodes, idMap)
		if parent == "" {
			viol = append(viol, fmt.Sprintf("%s: node %q names parent %q which is not a node of this diagram", label, f.Label, f.Parent))
			continue
		}
		idMap[f.originalID] = parent
		p := nodes[parent]
		if f.InBase != f.InHead {
			p.ComponentsChanged++
		}
		nodes[parent] = p
	}
	if len(viol) > 0 {
		return DiagramModel{}, viol, warn
	}

	isFlow := m.Section == SectionFlow
	edges := map[string]DiagramEdge{}
	for _, e := range m.Edges {
		from, ok1 := idMap[e.From]
		to, ok2 := idMap[e.To]
		switch {
		case !ok1 || !ok2:
			viol = append(viol, fmt.Sprintf("%s: %q -> %q names a node that is not in the diagram", label, e.From, e.To))
			continue
		case from == to:
			if idMap[e.From] != e.From || idMap[e.To] != e.To {
				continue // a call between two parts of one container is not drawn
			}
			viol = append(viol, fmt.Sprintf("%s: %q calls itself; a container diagram does not show self calls", label, e.From))
			continue
		}
		for _, ev := range e.Evidence {
			if err := CheckEvidenceShape(ev); err != nil {
				viol = append(viol, fmt.Sprintf("%s: %s -> %s: %s", label, from, to, err))
			}
		}
		presence := Presence{InBase: e.InBase, InHead: e.InHead}
		if designed {
			arch, flow := v.Design.Checkable()
			switch {
			case isFlow && flow:
				d := v.Design.HasCall(from, to)
				presence.InDesign = &d
			case !isFlow && arch:
				d := v.Design.HasEdge(from, to)
				presence.InDesign = &d
			}
		}
		state, change, ok := DeriveState(presence)
		if !ok {
			warn = append(warn, fmt.Sprintf("edge_dropped:%s->%s:seen_nowhere", from, to))
			continue
		}
		kept, _ := v.Evidence.VerifiedEvidence(e.Evidence)
		de := DiagramEdge{From: from, To: to, Protocol: cleanProtocol(e.Protocol), Label: clip(e.Label, MaxDiagramLabelRune), Order: e.Order, BaseOrder: e.BaseOrder,
			InBase: e.InBase, InHead: e.InHead, InDesign: presence.InDesign, Change: change, State: state, Evidence: capEvidence(kept)}
		de.Unverified = state.IsFinding() && len(kept) == 0
		de.Advisory = AdvisoryFor(state, nodes[from].ReviewState, nodes[to].ReviewState)
		if isFlow && de.InBase && de.InHead && de.BaseOrder > 0 && de.BaseOrder != de.Order {
			de.Reordered = true
		}
		key := fmt.Sprintf("%s|%s|%s|%d", from, to, de.Protocol, de.Order)
		if _, dup := edges[key]; !dup {
			edges[key] = de
		}
	}
	if len(viol) > 0 {
		return DiagramModel{}, viol, warn
	}

	res := DiagramModel{Section: m.Section, Type: m.Type, Variant: m.Variant, Level: LevelContainer, Name: clip(m.Name, 60)}
	for _, n := range nodes {
		n = finishNode(n, designed)
		res.Nodes = append(res.Nodes, n)
	}
	for _, e := range edges {
		res.Edges = append(res.Edges, e)
	}
	limit := MaxDiagramNodes
	if isFlow {
		limit = MaxDiagramLanes
	}
	res.Nodes, res.Edges, res.Truncated = capDiagram(res.Nodes, res.Edges, limit)
	sortDiagram(&res)
	if isFlow {
		renumberMessages(&res)
	}
	return res, nil, warn
}

type foldedNode struct {
	DiagramNode
	originalID string
}

func withOriginalID(n DiagramNode, id string) foldedNode {
	return foldedNode{DiagramNode: n, originalID: id}
}

func resolveParent(parent string, nodes map[string]DiagramNode, idMap map[string]string) string {
	if id, ok := idMap[parent]; ok {
		if _, in := nodes[id]; in {
			return id
		}
	}
	key := labelKey(parent)
	var ids []string
	for id, n := range nodes {
		if labelKey(n.Label) == key || labelKey(id) == key {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	if len(ids) > 0 {
		return ids[0]
	}
	return ""
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// finishNode derives the state of a container from where it was seen. A node only the code has, with a design to compare with,
// is unmapped.
func finishNode(n DiagramNode, designed bool) DiagramNode {
	state, _, ok := DeriveState(Presence{InBase: n.InBase, InHead: n.InHead})
	if !ok {
		state = StateSame
	}
	if n.Source == SourceCode && designed {
		state = StateUnmapped
	}
	n.State = state
	n.Unverified = false
	if n.ComponentsChanged > 0 {
		n.Drilldown = &DiagramDrilldown{Lens: "architecture", ContainerID: n.ID}
	} else {
		n.Drilldown = nil
	}
	n.Parent = ""
	return n
}

// capDiagram keeps the nodes a person has to look at and gathers the rest into one "+N more" node, so a picture stays readable.
// The choice is by state, then by id, so the same input is cut the same way.
func capDiagram(nodes []DiagramNode, edges []DiagramEdge, limit int) ([]DiagramNode, []DiagramEdge, bool) {
	if len(nodes) <= limit {
		return nodes, edges, false
	}
	sort.SliceStable(nodes, func(i, j int) bool {
		ri, rj := nodes[i].State.IsReviewable() || nodes[i].ComponentsChanged > 0, nodes[j].State.IsReviewable() || nodes[j].ComponentsChanged > 0
		if ri != rj {
			return ri
		}
		return nodes[i].ID < nodes[j].ID
	})
	kept, rest := append([]DiagramNode(nil), nodes[:limit-1]...), nodes[limit-1:]
	merged := map[string]bool{}
	for _, n := range rest {
		merged[n.ID] = true
	}
	more := DiagramNode{ID: moreNodeID, Label: fmt.Sprintf("+%d more", len(rest)), Kind: "external", Source: SourceCode, State: StateSame, InBase: true, InHead: true}
	kept = append(kept, more)
	seen := map[string]bool{}
	var out []DiagramEdge
	for _, e := range edges {
		if merged[e.From] && merged[e.To] {
			continue
		}
		if merged[e.From] {
			e.From = moreNodeID
		}
		if merged[e.To] {
			e.To = moreNodeID
		}
		key := fmt.Sprintf("%s|%s|%s|%s|%d", e.From, e.To, e.Protocol, e.State, e.Order)
		if !seen[key] {
			seen[key] = true
			out = append(out, e)
		}
	}
	return kept, out, true
}

func sortDiagram(m *DiagramModel) {
	sort.SliceStable(m.Nodes, func(i, j int) bool { return m.Nodes[i].ID < m.Nodes[j].ID })
	sort.SliceStable(m.Edges, func(i, j int) bool {
		a, b := m.Edges[i], m.Edges[j]
		switch {
		case a.Order != b.Order:
			return a.Order < b.Order
		case a.From != b.From:
			return a.From < b.From
		case a.To != b.To:
			return a.To < b.To
		}
		return a.Protocol < b.Protocol
	})
}

// renumberMessages makes the order of a sequence 1..n without gaps, in the order the agent gave.
func renumberMessages(m *DiagramModel) {
	for i := range m.Edges {
		m.Edges[i].Order = i + 1
	}
}

// validateSimpleDiagram checks a state machine or a database schema. They are not compared with the design: a state or a column
// is new, removed or the same by where it was seen.
func validateSimpleDiagram(m DiagramModel, v DiagramValidation, label string) (DiagramModel, ContextViolations, []string) {
	var viol ContextViolations
	var warn []string
	clip := func(s string, n int) string { return clipRunes(applyRedact(v.Redact, s), n) }
	delta := m.Variant == DiagramVariantDelta
	res := DiagramModel{Section: m.Section, Type: m.Type, Variant: m.Variant, Name: clip(m.Name, 60)}
	ids := map[string]bool{}
	for _, n := range m.Nodes {
		lbl := clip(n.Label, MaxDiagramLabelRune)
		id := Kebab(firstNonEmpty(n.ID, lbl))
		if lbl == "" || id == "" {
			viol = append(viol, fmt.Sprintf("%s: a node has no label", label))
			continue
		}
		if ids[id] {
			continue
		}
		ids[id] = true
		node := DiagramNode{ID: id, Label: lbl, InBase: n.InBase, InHead: n.InHead}
		if !delta {
			node.InHead = true
		}
		node.State = simpleState(node.InBase, node.InHead, delta)
		for _, c := range n.Columns {
			name := clip(c.Name, MaxDiagramLabelRune)
			if name == "" {
				continue
			}
			col := DiagramColumn{Name: name, Type: clip(c.Type, 30), InBase: c.InBase, InHead: c.InHead}
			if !delta {
				col.InHead = true
			}
			col.State = simpleState(col.InBase, col.InHead, delta)
			node.Columns = append(node.Columns, col)
			if len(node.Columns) == 40 {
				res.Truncated = true
				break
			}
		}
		res.Nodes = append(res.Nodes, node)
	}
	for _, e := range m.Edges {
		from, to := Kebab(e.From), Kebab(e.To)
		if !ids[from] || !ids[to] {
			viol = append(viol, fmt.Sprintf("%s: %q -> %q names a node that is not in the diagram", label, e.From, e.To))
			continue
		}
		de := DiagramEdge{From: from, To: to, Label: clip(e.Label, MaxDiagramLabelRune), InBase: e.InBase, InHead: e.InHead || !delta}
		de.State = simpleState(de.InBase, de.InHead, delta)
		de.Change = de.State
		res.Edges = append(res.Edges, de)
	}
	if len(res.Nodes) > MaxDiagramNodes {
		res.Nodes, res.Edges, res.Truncated = capDiagram(res.Nodes, res.Edges, MaxDiagramNodes)
	}
	sortDiagram(&res)
	return res, viol, warn
}

// simpleState is the state of a thing with no design to compare to. Seen nowhere reads as the same, since it has to be drawn.
func simpleState(inBase, inHead, delta bool) EdgeState {
	if !delta {
		return StateSame
	}
	s, _, ok := DeriveState(Presence{InBase: inBase, InHead: inHead})
	if !ok {
		return StateSame
	}
	return s
}

// BuildDesignDiagrams draws the design itself, from the locked design only: the containers and their edges, and each designed
// flow. The same design gives the same models, whatever the commit.
func BuildDesignDiagrams(d DesignModel) ([]DiagramModel, error) {
	if !d.Available || len(d.Containers) == 0 {
		return nil, nil
	}
	var out []DiagramModel
	arch := DiagramModel{Section: SectionArchitecture, Type: DiagramArchitecture, Variant: DiagramVariantDesign, Level: LevelContainer}
	for _, c := range d.Containers {
		n := DiagramNode{ID: c.ID, Source: c.Source, ReviewState: d.ReviewStateOf(c.ID), Label: c.Label, Kind: c.Kind, Tech: c.Tech, InBase: true, InHead: true, State: StateSame}
		if c.KGID != nil {
			n.KGID = *c.KGID
		}
		arch.Nodes = append(arch.Nodes, n)
	}
	for _, e := range d.Edges {
		t := true
		arch.Edges = append(arch.Edges, DiagramEdge{From: e.From, To: e.To, Protocol: e.Protocol, Label: e.Label, InBase: true, InHead: true, InDesign: &t, Change: StateSame, State: StateSame, Evidence: e.Evidence})
	}
	arch.Nodes, arch.Edges, arch.Truncated = capDiagram(arch.Nodes, arch.Edges, MaxDiagramNodes)
	sortDiagram(&arch)
	if len(arch.Edges) > 0 {
		out = append(out, arch)
	}
	for _, f := range d.Flows {
		seq := DiagramModel{Section: SectionFlow, Type: DiagramSequence, Variant: DiagramVariantDesign, Level: LevelContainer, Name: f.Name}
		lanes := map[string]bool{}
		for _, m := range f.Messages {
			for _, id := range []string{m.From, m.To} {
				if lanes[id] {
					continue
				}
				lanes[id] = true
				if c, ok := d.ContainerByID(id); ok {
					n := DiagramNode{ID: c.ID, Source: c.Source, ReviewState: d.ReviewStateOf(c.ID), Label: c.Label, Kind: c.Kind, InBase: true, InHead: true, State: StateSame}
					if c.KGID != nil {
						n.KGID = *c.KGID
					}
					seq.Nodes = append(seq.Nodes, n)
				}
			}
			t := true
			seq.Edges = append(seq.Edges, DiagramEdge{From: m.From, To: m.To, Protocol: m.Protocol, Label: m.Label, Order: m.Order, InBase: true, InHead: true, InDesign: &t, Change: StateSame, State: StateSame})
		}
		seq.Nodes, seq.Edges, seq.Truncated = capDiagram(seq.Nodes, seq.Edges, MaxDiagramLanes)
		sortDiagram(&seq)
		renumberMessages(&seq)
		out = append(out, seq)
	}
	for i := range out {
		d, err := out[i].ComputeInputDigest()
		if err != nil {
			return nil, err
		}
		out[i].InputDigest = d
	}
	return out, nil
}

// ComputeDesignVerdicts counts what the validated diagrams say. The counts are of what an agent reported and Orca checked, so the
// screen labels them that way; designChecked says whether there was a design to compare with.
func ComputeDesignVerdicts(models []DiagramModel, d DesignModel) (arch, flow DesignVerdict) {
	archChecked, flowChecked := d.Checkable()
	arch = DesignVerdict{DesignChecked: archChecked, ReviewedRatio: d.ReviewedRatio()}
	flow = DesignVerdict{DesignChecked: flowChecked, ReviewedRatio: d.ReviewedRatio()}
	for _, m := range models {
		if m.Variant != DiagramVariantDelta {
			continue
		}
		switch m.Section {
		case SectionArchitecture:
			for _, n := range m.Nodes {
				if n.ID != moreNodeID && (n.State != StateSame || n.ComponentsChanged > 0) {
					arch.ContainersChanged++
				}
			}
			for _, e := range m.Edges {
				countEdge(&arch, e, false)
			}
		case SectionFlow:
			for _, e := range m.Edges {
				countEdge(&flow, e, true)
			}
		}
	}
	return arch, flow
}

func countEdge(v *DesignVerdict, e DiagramEdge, flow bool) {
	switch e.Change {
	case StateAdded:
		if flow {
			v.NewCalls++
		} else {
			v.NewEdges++
		}
	case StateRemoved:
		if flow {
			v.RemovedCalls++
		} else {
			v.RemovedEdges++
		}
	}
	if flow && e.Reordered {
		v.Reordered++
	}
	if e.State == StateDeviation {
		if flow {
			v.UnplannedCalls++
		} else {
			v.UnplannedEdges++
		}
	}
	if e.State == StateDeviation || e.State == StateMissing {
		v.ReportedDeviations++
	}
	if e.State.IsFinding() && !e.Unverified {
		v.WithEvidence++
	}
	if e.Advisory {
		v.Advisory++
	}
}
