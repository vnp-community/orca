package command

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/apiclient"
)

const knowledgeToolPrefix = "mcp__tdd-agent__"

// renderRunStats is one line about what an agent run reported: how often it called the knowledge graph's tools, how many
// other tools, turns, time and cost. It prints "cost not reported" rather than a zero when claude gave no figure, and
// returns "" for a run that reported nothing. Only names and numbers are shown, never what a tool read.
func renderRunStats(stats apiclient.Document, elapsedMs int64) string {
	if stats == nil && elapsedMs <= 0 {
		return ""
	}
	var parts []string
	if stats != nil {
		calls := stats.IntMap("toolCalls")
		var kg []string
		var other int64
		for name, n := range calls {
			if short, ok := strings.CutPrefix(name, knowledgeToolPrefix); ok {
				kg = append(kg, fmt.Sprintf("%s x%d", short, n))
			} else {
				other += n
			}
		}
		sort.Strings(kg)
		parts = append(parts, fmt.Sprintf("%d knowledge tool call(s)%s, %d other tool call(s)", stats.Int("knowledgeToolCalls"), listSuffix(kg), other))
		if turns := stats.Int("turns"); turns > 0 {
			parts = append(parts, fmt.Sprintf("%d turn(s)", turns))
		}
	}
	if elapsedMs <= 0 && stats != nil {
		elapsedMs = stats.Int("durationMs")
	}
	if elapsedMs > 0 {
		parts = append(parts, (time.Duration(elapsedMs) * time.Millisecond).Round(time.Second).String())
	}
	if stats != nil {
		if cost, ok := stats.Float("costUsd"); ok {
			parts = append(parts, fmt.Sprintf("$%.2f", cost))
		} else {
			parts = append(parts, "cost not reported")
		}
	}
	return strings.Join(parts, " · ")
}

func listSuffix(items []string) string {
	if len(items) == 0 {
		return ""
	}
	return " (" + strings.Join(items, ", ") + ")"
}
