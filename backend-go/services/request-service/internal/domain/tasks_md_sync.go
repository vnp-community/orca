package domain

import (
	"regexp"
	"strings"
	"time"
)

// MaxSyncAttempts is how many times a failed sync or archive is retried before the change is left as failed. A failed sync
// never changes the Request, which stays completed.
const MaxSyncAttempts = 5

var syncBackoff = []time.Duration{30 * time.Second, 2 * time.Minute, 10 * time.Minute, 30 * time.Minute, 2 * time.Hour}

// SyncBackoff is the wait after the attempt-th failure (1-based).
func SyncBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > len(syncBackoff) {
		attempt = len(syncBackoff)
	}
	return syncBackoff[attempt-1]
}

var specLabelRegex = regexp.MustCompile(`^spec:t(\d+)\.(\d+)$`)

// SpecLabel marks a Plan task with the position of its line in tasks.md. Labels are lower-cased by task-service, so the
// reference travels in that form.
func SpecLabel(ref string) string { return "spec:" + strings.ToLower(ref) }

// SpecRefFromLabels returns the tasks.md reference ("T1.2") a task was created for, when it carries a spec label.
func SpecRefFromLabels(labels []string) (string, bool) {
	for _, l := range labels {
		if m := specLabelRegex.FindStringSubmatch(strings.ToLower(strings.TrimSpace(l))); m != nil {
			return "T" + m[1] + "." + m[2], true
		}
	}
	return "", false
}

// TickTasks ticks every ref that is open in the plan region and reports which ones changed. A ref that is missing or
// already ticked is skipped without an error: ticking is one way and a hand-edited line is not Orca's to rewrite.
func TickTasks(md string, refs []string) (string, []string, error) {
	var ticked []string
	for _, ref := range refs {
		next, changed, err := TickTask(md, ref)
		if err != nil {
			return md, nil, err
		}
		if changed {
			md = next
			ticked = append(ticked, ref)
		}
	}
	return md, ticked, nil
}

// PlanRegionText returns the region of md between and including its markers, the part Orca owns.
func PlanRegionText(md string) (string, bool) {
	lines := strings.Split(md, "\n")
	start, end, ok := planRegionBounds(lines)
	if !ok {
		return "", false
	}
	return strings.Join(lines[start:end+1], "\n"), true
}

// archiveScopePrefixes are the only places `openspec archive` is expected to write.
var archiveScopePrefixes = []string{"openspec/changes/", "openspec/specs/"}

// ArchiveScopeViolations returns the changed paths outside openspec/changes/ and openspec/specs/. An archive that touches
// anything else is discarded, because the archive step must never be a way to change code.
func ArchiveScopeViolations(paths []string) []string {
	var bad []string
	for _, p := range paths {
		p = strings.ReplaceAll(p, "\\", "/")
		ok := false
		for _, pre := range archiveScopePrefixes {
			if strings.HasPrefix(p, pre) && !strings.Contains(p, "/../") {
				ok = true
				break
			}
		}
		if !ok {
			bad = append(bad, p)
		}
	}
	return bad
}
