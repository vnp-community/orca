package domain

import "fmt"

// maxDiagramFindingItems keeps a noisy diagram from burying the items an agent reported.
const maxDiagramFindingItems = 24

// DiagramFindingItems turns what the checked after-code diagrams found into rows, so a deviation that a picture shows is also in
// the table a person decides from: an arrow in the code that the design does not have, or one the design has and the code lacks.
// They are facts Orca derived from the checked model (origin fact), never an agent's words; an arrow that rests on elements
// nobody approved stays advisory and asks for no decision.
func DiagramFindingItems(models []DiagramModel) []ReviewItem {
	var out []ReviewItem
	for _, m := range models {
		if m.Variant != DiagramVariantDelta {
			continue
		}
		label := map[string]string{}
		for _, n := range m.Nodes {
			label[n.ID] = firstNonEmpty(n.Label, n.ID)
		}
		for _, e := range m.Edges {
			if e.State != StateDeviation && e.State != StateMissing {
				continue
			}
			from, to := firstNonEmpty(label[e.From], e.From), firstNonEmpty(label[e.To], e.To)
			title := fmt.Sprintf("%s → %s", from, to)
			if e.Protocol != "" {
				title += " (" + e.Protocol + ")"
			}
			change, detail := ChangeAdded, "In the code but not in the design."
			if e.State == StateMissing {
				change, detail = ChangeMissing, "In the design but not in the code."
			}
			it := ReviewItem{
				Key:     ReviewItemKey(m.Section, "edge", m.Variant+"|"+e.From+">"+e.To+"|"+e.Protocol),
				Section: m.Section, Change: change, Origin: OriginFact, Title: clipRunes(title, MaxReviewTitleRunes), Detail: detail,
				Severity: SeverityDeviation, Containers: []string{clipRunes(from, MaxDiagramLabelRune), clipRunes(to, MaxDiagramLabelRune)},
				Protocol: cleanProtocol(e.Protocol), Expected: e.InDesign, Evidence: capEvidence(e.Evidence), ReviewSt: KGUnknown,
				Advisory: e.Advisory, Unverified: e.Unverified,
			}
			it.NeedsDecision = !it.Advisory && !it.Unverified
			out = append(out, it)
			if len(out) == maxDiagramFindingItems {
				return out
			}
		}
	}
	return out
}
