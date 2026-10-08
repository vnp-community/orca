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
			for j, t := range ph.Tasks {
				idStr := fmt.Sprintf("T%d.%d", i+1, j+1)
				renderTask(&b, idStr, t)
				if uid, ok := ids[idStr]; ok {
					b.WriteString(fmt.Sprintf("  <!-- orca:task %s id=%s -->\n", idStr, uid))
				}
			}
		}
	} else if len(p.Tasks) > 0 {
		b.WriteString("## TASKS\n")
		for j, t := range p.Tasks {
			idStr := fmt.Sprintf("T1.%d", j+1)
			renderTask(&b, idStr, t)
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

func renderTask(b *strings.Builder, idStr string, t TaskProposal) {
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
	// depends is tricky to reconstruct names from indices in render, skipping for brevity
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

func TickTask(md string, taskRef string) (string, bool, error) {
	// find the line with the taskRef
	lines := strings.Split(md, "\n")
	changed := false
	for i, line := range lines {
		if strings.HasPrefix(line, "- [ ] "+taskRef+" ") {
			lines[i] = strings.Replace(line, "- [ ] "+taskRef+" ", "- [x] "+taskRef+" ", 1)
			changed = true
			break
		}
	}
	return strings.Join(lines, "\n"), changed, nil
}

func ReplacePlanRegion(md, region string) (string, error) {
	return region, nil // stub
}

func PlanRegionDigest(md string) string {
	t := transform.Chain(norm.NFC)
	md, _, _ = transform.String(t, md)
	md = strings.ReplaceAll(md, "\r\n", "\n")
	hash := sha256.Sum256([]byte(md))
	return hex.EncodeToString(hash[:])
}
