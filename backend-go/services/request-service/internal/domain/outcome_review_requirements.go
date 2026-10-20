package domain

import (
	"fmt"
	"sort"
)

// RequirementStatus is what the review says about one acceptance criterion.
type RequirementStatus string

const (
	ReqPlanned    RequirementStatus = "planned"
	ReqDelivered  RequirementStatus = "delivered"
	ReqPartial    RequirementStatus = "partial"
	ReqNotCovered RequirementStatus = "notCovered"
	ReqUnknown    RequirementStatus = "unknown"
)

// RequirementInputs is everything the status of an acceptance criterion is worked out from. Nothing here comes from an agent.
type RequirementInputs struct {
	Phase    ReviewPhase
	Criteria []AcceptanceCriterion
	Coverage []CoverageRow
	// Tasks maps a task id to its status and whether its run ended in failure; a task missing from the map is unknown.
	Tasks map[string]CoveringTask
	// DataOK is false when the coverage or the tasks could not be read; every status is then unknown rather than guessed.
	DataOK bool
}

type CoveringTask struct {
	Status string
	Title  string
	Failed bool
}

type RequirementResult struct {
	Item   ReviewItem
	Status RequirementStatus
}

// ClassifyRequirements gives each active criterion a status.
//
//   - design phase: every criterion is planned; nothing has been built so nothing can be measured.
//   - delivered, no coverage row: notCovered (the plan never claimed it).
//   - delivered, covering tasks: delivered when all are done and none failed; partial when some are done or any failed after
//     finishing others; notCovered when none is done, because nothing was delivered for it.
//   - data could not be read: unknown. A criterion without data is never delivered.
//
// "Delivered" means every task the plan tied to the criterion is done. Orca does not know which command proved it, so the item
// says so instead of claiming a passing check.
func ClassifyRequirements(in RequirementInputs) []RequirementResult {
	byAC := map[string][]CoverageRow{}
	for _, r := range in.Coverage {
		byAC[r.ACID] = append(byAC[r.ACID], r)
	}
	out := make([]RequirementResult, 0, len(in.Criteria))
	for _, c := range in.Criteria {
		if c.Status != ACStatusActive && c.Status != "" {
			continue
		}
		st, detail, ev := classifyOne(in, c, byAC[c.ID])
		item := ReviewItem{
			Key:      ReviewItemKey(SectionRequirements, "ac", c.ID),
			Section:  SectionRequirements,
			Change:   ChangeUnchanged,
			Origin:   OriginFact,
			Title:    c.ID + ": " + c.Text,
			Detail:   detail,
			Severity: requirementSeverity(st),
			Evidence: ev,
			Before:   string(ReqPlanned),
			After:    string(st),
		}
		if st == ReqNotCovered && in.Phase == ReviewPhaseDelivered {
			item.NeedsDecision = true
		}
		out = append(out, RequirementResult{Item: item, Status: st})
	}
	sort.SliceStable(out, func(i, j int) bool { return acNumber(out[i].Item.Title) < acNumber(out[j].Item.Title) })
	return out
}

func requirementSeverity(s RequirementStatus) ReviewSeverity {
	switch s {
	case ReqNotCovered:
		return SeverityDeviation
	case ReqPartial:
		return SeverityWarning
	}
	return SeverityInfo
}

func classifyOne(in RequirementInputs, c AcceptanceCriterion, rows []CoverageRow) (RequirementStatus, string, []ReviewEvidence) {
	if in.Phase == ReviewPhaseDesign {
		return ReqPlanned, "Planned. Nothing is measured before execution finishes.", nil
	}
	if !in.DataOK {
		return ReqUnknown, "The coverage or the tasks could not be read.", nil
	}
	if len(rows) == 0 {
		return ReqNotCovered, "No task in the plan is tied to this criterion.", nil
	}
	seen := map[string]bool{}
	var done, total int
	var failed bool
	var ev []ReviewEvidence
	for _, r := range rows {
		if r.TaskID == "" || seen[r.TaskID] {
			continue
		}
		seen[r.TaskID] = true
		t, ok := in.Tasks[r.TaskID]
		total++
		ev = append(ev, ReviewEvidence{Kind: EvidenceTask, Ref: r.TaskID})
		if !ok {
			continue
		}
		if t.Status == TaskStatusDone {
			done++
		}
		if t.Failed {
			failed = true
		}
	}
	switch {
	case total > 0 && done == total && !failed:
		return ReqDelivered, fmt.Sprintf("All %d task(s) tied to this criterion are done. Orca does not know which check proved it.", total), ev
	case done > 0:
		return ReqPartial, fmt.Sprintf("%d of %d task(s) tied to this criterion are done.", done, total), ev
	default:
		return ReqNotCovered, fmt.Sprintf("None of the %d task(s) tied to this criterion is done.", total), ev
	}
}

func acNumber(title string) int {
	n := 0
	for i := 3; i < len(title) && title[i] >= '0' && title[i] <= '9'; i++ {
		n = n*10 + int(title[i]-'0')
	}
	return n
}

// CountRequirements folds results into the verdict counts.
func CountRequirements(rs []RequirementResult) RequirementsVerdict {
	var v RequirementsVerdict
	for _, r := range rs {
		v.Total++
		switch r.Status {
		case ReqDelivered:
			v.Delivered++
		case ReqPartial:
			v.Partial++
		case ReqNotCovered:
			v.NotCovered++
		case ReqPlanned:
			v.Planned++
		default:
			v.Unknown++
		}
	}
	return v
}
