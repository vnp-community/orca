package usecase

import (
	"path"
	"strings"
)

func CheckProposalScope(changed []string, changeID string) []string {
	prefix := "openspec/changes/" + changeID + "/"
	var outOfScope []string
	for _, p := range changed {
		p = path.Clean(strings.ReplaceAll(p, "\\", "/"))
		if strings.Contains(p, "\x00") || len(p) > 512 {
			outOfScope = append(outOfScope, p)
			continue
		}
		if path.IsAbs(p) || strings.HasPrefix(p, "../") {
			outOfScope = append(outOfScope, p)
			continue
		}
		if !strings.HasPrefix(p, prefix) {
			outOfScope = append(outOfScope, p)
		}
	}
	return outOfScope
}
