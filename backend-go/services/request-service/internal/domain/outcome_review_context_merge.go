package domain

import (
	"fmt"
	"sort"
	"strings"
)

// ContextValidationInput is what Orca knows to check a context result against.
type ContextValidationInput struct {
	Mode ContextMode
	// Evidence holds the facts evidence is checked with; KGRead and Docs are filled from the design.
	Evidence EvidenceContext
	Redact   func(string) string
	// LockedDesign is the design already extracted for this snapshot and these documents. In delivered mode it replaces whatever
	// design the result carries, so the graph is read once.
	LockedDesign *DesignModel
}

// ValidatedContext is a context result after Orca checked it: only what passed, with every state derived by Orca.
type ValidatedContext struct {
	KG       KGInfo         `json:"kg"`
	Design   DesignModel    `json:"design"`
	Items    []ReviewItem   `json:"items"`
	Diagrams []DiagramModel `json:"diagrams"`
	Arch     DesignVerdict  `json:"arch"`
	Flow     DesignVerdict  `json:"flow"`
	Warnings []string       `json:"warnings"`
	Notes    string         `json:"notes,omitempty"`
}

var allowedAgentWarnings = map[string]bool{"budget_exhausted": true, "kg_partial": true, "partial_result": true}

// ValidateContext applies the contract to a result. A ContextViolations error lists what the agent can fix; anything else Orca can
// deal with is a warning or an item marked unverified.
func ValidateContext(raw ContextResult, in ContextValidationInput) (ValidatedContext, error) {
	var viol ContextViolations
	var warn []string
	if !in.Mode.Valid() {
		return ValidatedContext{}, fmt.Errorf("context mode %q is not valid", in.Mode)
	}
	clip := func(s string, n int) string { return clipRunes(applyRedact(in.Redact, s), n) }

	out := ValidatedContext{KG: KGInfo{Available: raw.KG.Available, Snapshot: clip(raw.KG.Snapshot, 80)}, Notes: clip(raw.Notes, MaxContextNotesRunes)}
	for _, g := range raw.KG.Gaps {
		if len(out.KG.Gaps) < 20 {
			out.KG.Gaps = append(out.KG.Gaps, clip(g, 160))
		}
	}
	for _, c := range raw.KG.Conflicts {
		if len(out.KG.Conflicts) < 20 {
			out.KG.Conflicts = append(out.KG.Conflicts, clip(c, 160))
		}
	}

	var design DesignModel
	if in.Mode == ContextModeDelivered && in.LockedDesign != nil {
		design = *in.LockedDesign
		out.KG.Available, out.KG.Snapshot = design.Available, design.Snapshot
	} else {
		var dv ContextViolations
		var dw []string
		design, dv, dw = NormalizeDesign(raw.KG, raw.Design, in.Redact)
		viol, warn = append(viol, dv...), append(warn, dw...)
	}
	out.Design = design

	ev := in.Evidence
	ev.KGRead, ev.Docs = map[string]bool{}, design.DocSha256
	for _, e := range design.Elements {
		ev.KGRead[e.ID] = true
	}
	for _, id := range raw.KG.Read {
		if ValidKGID(id) && out.KG.Available {
			ev.KGRead[id] = true
		}
	}

	items, iv := validateAgentItems(raw.Items, ev, design, clip)
	viol = append(viol, iv...)

	models, mv, mw := ValidateDiagrams(raw.Diagrams, DiagramValidation{Mode: in.Mode, Design: design, Evidence: ev, Redact: in.Redact})
	viol, warn = append(viol, mv...), append(warn, mw...)
	if len(viol) > 0 {
		return ValidatedContext{}, dedupeViolations(viol)
	}

	if design.Available {
		designModels, err := BuildDesignDiagrams(design)
		if err != nil {
			return ValidatedContext{}, err
		}
		models = append(models, designModels...)
	}
	sort.SliceStable(models, func(i, j int) bool {
		a, b := models[i], models[j]
		if a.Section != b.Section {
			return sectionOrder(a.Section) < sectionOrder(b.Section)
		}
		if a.Variant != b.Variant {
			return a.Variant < b.Variant
		}
		return a.Name < b.Name
	})
	out.Diagrams = models
	out.Items = items
	out.Arch, out.Flow = ComputeDesignVerdicts(models, design)
	addItemsToVerdict(&out.Arch, items, SectionArchitecture)
	addItemsToVerdict(&out.Flow, items, SectionFlow)

	if !out.KG.Available {
		warn = append(warn, "kg_unavailable")
	}
	for _, w := range raw.Warnings {
		if allowedAgentWarnings[w] {
			warn = append(warn, w)
		}
	}
	out.Warnings = dedupeSorted(warn)
	return out, nil
}

func dedupeViolations(v ContextViolations) ContextViolations {
	seen := map[string]bool{}
	var out ContextViolations
	for _, s := range v {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	if len(out) > 30 {
		out = append(out[:30], fmt.Sprintf("and %d more", len(out)-30))
	}
	return out
}

func validateAgentItems(in []AgentReviewItem, ev EvidenceContext, design DesignModel, clip func(string, int) string) ([]ReviewItem, ContextViolations) {
	var viol ContextViolations
	if len(in) > MaxContextItems {
		return nil, ContextViolations{fmt.Sprintf("%d items were returned; the limit is %d", len(in), MaxContextItems)}
	}
	var out []ReviewItem
	for i, a := range in {
		where := fmt.Sprintf("items[%d]", i)
		switch {
		case !a.Section.Valid():
			viol = append(viol, fmt.Sprintf("%s: section %q is not one of the seven sections", where, a.Section))
			continue
		case !validChange(a.Change):
			viol = append(viol, fmt.Sprintf("%s: change %q is not added, modified, removed, unchanged or missing", where, a.Change))
			continue
		case strings.TrimSpace(a.Title) == "":
			viol = append(viol, where+": title is empty")
			continue
		}
		shapeOK := true
		for _, e := range a.Evidence {
			if err := CheckEvidenceShape(e); err != nil {
				viol = append(viol, fmt.Sprintf("%s: %s", where, err))
				shapeOK = false
			}
		}
		if !shapeOK {
			continue
		}
		sev := a.Severity
		if sev.rank() == 1 && sev != SeverityInfo {
			sev = SeverityInfo
		}
		kept, _ := ev.VerifiedEvidence(a.Evidence)
		it := ReviewItem{
			Key:     ReviewItemKey(a.Section, "agent", firstNonEmpty(a.Subject, a.Title)),
			Section: a.Section, Change: a.Change, Origin: OriginAgent, Title: clip(a.Title, MaxReviewTitleRunes), Detail: clip(a.Detail, MaxReviewDetailRune),
			Severity: sev, Protocol: cleanProtocol(a.Protocol), Source: validSource(a.Source), ReviewSt: a.ReviewState,
			Expected: a.Expected, Evidence: capEvidence(kept), Before: clip(a.Before, 120), After: clip(a.After, 120),
		}
		if it.ReviewSt != KGApproved && it.ReviewSt != KGNeedsReview {
			it.ReviewSt = KGUnknown
		}
		for _, c := range a.Containers {
			if c = clip(c, MaxDiagramLabelRune); c != "" && len(it.Containers) < 6 {
				it.Containers = append(it.Containers, c)
			}
		}
		it.Unverified = len(kept) == 0
		it.Advisory = (sev == SeverityDeviation || (a.Expected != nil && !*a.Expected) || a.Change == ChangeMissing) && it.ReviewSt != KGApproved && design.Available
		it.NeedsDecision = a.NeedsDecision && !it.Advisory && !it.Unverified
		out = append(out, it)
	}
	return out, viol
}

func validChange(c ReviewChange) bool {
	switch c {
	case ChangeAdded, ChangeModified, ChangeRemoved, ChangeUnchanged, ChangeMissing:
		return true
	}
	return false
}

func validSource(s ComponentSource) ComponentSource {
	switch s {
	case SourceKG, SourceSADText, SourceCode:
		return s
	}
	return ""
}

func addItemsToVerdict(v *DesignVerdict, items []ReviewItem, section ReviewSection) {
	for _, it := range items {
		if it.Section != section {
			continue
		}
		if it.Severity == SeverityDeviation {
			v.ReportedDeviations++
		}
		if !it.Unverified && (it.Severity == SeverityDeviation || it.Severity == SeverityBreaking || it.NeedsDecision) {
			v.WithEvidence++
		}
		if it.Advisory {
			v.Advisory++
		}
	}
}
