package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

func RenderPlanRegion(ref string, p PlanProposal, ids map[string]string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("<!-- orca:begin plan request=%s schema=1 -->\n", ref))

	if len(p.Phases) > 0 {
		for i, ph := range p.Phases {
			b.WriteString(fmt.Sprintf("## PH-%d %s\n", i+1, ph.Title))
			if ph.Description != "" {
				lines := strings.Split(strings.TrimSpace(ph.Description), "\n")
				for _, line := range lines {
					b.WriteString("> " + line + "\n")
				}
			}
			siblings := siblingIDs(i+1, len(ph.Tasks))
			for j, t := range ph.Tasks {
				idStr := fmt.Sprintf("T%d.%d", i+1, j+1)
				renderTask(&b, idStr, t, siblings)
				if uid, ok := ids[idStr]; ok {
					b.WriteString(fmt.Sprintf("  <!-- orca:task %s id=%s -->\n", idStr, uid))
				}
			}
		}
	} else if len(p.Tasks) > 0 {
		b.WriteString("## TASKS\n")
		siblings := siblingIDs(1, len(p.Tasks))
		for j, t := range p.Tasks {
			idStr := fmt.Sprintf("T1.%d", j+1)
			renderTask(&b, idStr, t, siblings)
			if uid, ok := ids[idStr]; ok {
				b.WriteString(fmt.Sprintf("  <!-- orca:task %s id=%s -->\n", idStr, uid))
			}
		}
	}

	b.WriteString("<!-- orca:end plan -->")

	t := transform.Chain(norm.NFC)
	res, _, _ := transform.String(t, b.String())
	return res
}

// siblingIDs are the ids of one container's tasks in order; depends= names a sibling by its id. A dependency on an
// earlier phase is not written: phases are ordered, which says the same thing.
func siblingIDs(container, n int) []string {
	out := make([]string, n)
	for j := range out {
		out[j] = fmt.Sprintf("T%d.%d", container, j+1)
	}
	return out
}

func renderTask(b *strings.Builder, idStr string, t TaskProposal, dep []string) {
	cb := " "
	if t.Done {
		cb = "x"
	}
	b.WriteString(fmt.Sprintf("- [%s] %s %s", cb, idStr, t.Title))
	var attrs []string
	if t.TaskType != "" && t.TaskType != "task" {
		attrs = append(attrs, fmt.Sprintf("[type=%s]", t.TaskType))
	}
	if t.EstimatedHours > 0 {
		attrs = append(attrs, fmt.Sprintf("[h=%v]", t.EstimatedHours))
	}
	if len(t.Satisfies) > 0 {
		attrs = append(attrs, fmt.Sprintf("[ac=%s]", strings.Join(t.Satisfies, ",")))
	}
	if len(t.Labels) > 0 {
		attrs = append(attrs, fmt.Sprintf("[labels=%s]", strings.Join(t.Labels, ",")))
	}
	var deps []string
	for _, idx := range t.DependsOnIndices {
		if idx >= 0 && idx < len(dep) {
			deps = append(deps, dep[idx])
		}
	}
	if len(deps) > 0 {
		attrs = append(attrs, fmt.Sprintf("[depends=%s]", strings.Join(deps, ",")))
	}
	if t.Irreversible {
		attrs = append(attrs, "[irreversible]")
	}
	if len(attrs) > 0 {
		b.WriteString(" " + strings.Join(attrs, " "))
	}
	b.WriteString("\n")
	if t.Description != "" {
		lines := strings.Split(strings.TrimSpace(t.Description), "\n")
		for _, line := range lines {
			b.WriteString("  " + line + "\n")
		}
	}
}

// planRegionBounds returns the line indexes of the begin and end markers, or ok=false when the document has no region.
func planRegionBounds(lines []string) (start, end int, ok bool) {
	start, end = -1, -1
	for i, line := range lines {
		switch {
		case start < 0 && beginRegex.MatchString(strings.TrimRight(line, "\r")):
			start = i
		case start >= 0 && endRegex.MatchString(strings.TrimRight(line, "\r")):
			return start, i, true
		}
	}
	return -1, -1, false
}

// TickTask turns "- [ ] <ref> " into "- [x] <ref> " inside the plan region only: a look-alike checkbox in the prose around
// it must never be touched. It never un-ticks: ticking is one way, so a task that is done stays done.
func TickTask(md string, taskRef string) (string, bool, error) {
	lines := strings.Split(md, "\n")
	start, end, ok := planRegionBounds(lines)
	if !ok {
		return md, false, fmt.Errorf("tasks.md has no plan region")
	}
	prefix := "- [ ] " + taskRef + " "
	for i := start + 1; i < end; i++ {
		if strings.HasPrefix(lines[i], prefix) {
			lines[i] = "- [x] " + taskRef + " " + strings.TrimPrefix(lines[i], prefix)
			return strings.Join(lines, "\n"), true, nil
		}
	}
	return md, false, nil
}

// ReplacePlanRegion swaps the region (markers included) of md for region and keeps everything around it byte for byte.
func ReplacePlanRegion(md, region string) (string, error) {
	lines := strings.Split(md, "\n")
	start, end, ok := planRegionBounds(lines)
	if !ok {
		return "", fmt.Errorf("tasks.md has no plan region")
	}
	regionLines := strings.Split(strings.TrimRight(region, "\r\n"), "\n")
	if strings.HasSuffix(lines[start], "\r") { // a CRLF document stays CRLF: no mixed endings
		for i := range regionLines {
			regionLines[i] = strings.TrimRight(regionLines[i], "\r") + "\r"
		}
		if !strings.HasSuffix(lines[end], "\r") {
			regionLines[len(regionLines)-1] = strings.TrimSuffix(regionLines[len(regionLines)-1], "\r")
		}
	}
	out := append(append(append([]string{}, lines[:start]...), regionLines...), lines[end+1:]...)
	return strings.Join(out, "\n"), nil
}

func PlanRegionDigest(md string) string {
	t := transform.Chain(norm.NFC)
	md, _, _ = transform.String(t, md)
	md = strings.ReplaceAll(md, "\r\n", "\n")
	hash := sha256.Sum256([]byte(md))
	return hex.EncodeToString(hash[:])
}
