package domain

import "sort"

// BuildCoverageRows derives request_coverage from a plan: one row per (AC, task, check) of every
// non-exempt task, or one row without a check when the task has none. Order is stable.
func BuildCoverageRows(planTaskID string, p PlanSpecs) []CoverageRow {
	var rows []CoverageRow
	for _, t := range p.Tasks {
		if t.ExemptFromCoverage {
			continue
		}
		acs := append([]string(nil), t.Satisfies...)
		sort.Strings(acs)
		prev := ""
		for _, ac := range acs {
			if ac == prev {
				continue
			}
			prev = ac
			if len(t.CheckIDs) == 0 {
				rows = append(rows, CoverageRow{ACID: ac, TaskID: t.ID, PlanTaskID: planTaskID})
				continue
			}
			checks := append([]string(nil), t.CheckIDs...)
			sort.Strings(checks)
			for _, c := range checks {
				rows = append(rows, CoverageRow{ACID: ac, TaskID: t.ID, CheckID: c, PlanTaskID: planTaskID})
			}
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.ACID != b.ACID {
			return a.ACID < b.ACID
		}
		if a.TaskID != b.TaskID {
			return a.TaskID < b.TaskID
		}
		return a.CheckID < b.CheckID
	})
	return rows
}

// UncoveredACs lists the active acceptance criteria no coverage row mentions.
func UncoveredACs(ac AcceptanceCriteria, rows []CoverageRow) []string {
	covered := map[string]bool{}
	for _, r := range rows {
		covered[r.ACID] = true
	}
	var out []string
	for _, id := range ac.ActiveIDs() {
		if !covered[id] {
			out = append(out, id)
		}
	}
	return out
}
