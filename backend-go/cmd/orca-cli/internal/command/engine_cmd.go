package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/apiclient"
)

func newEngineCmd(e flowEnv) *cobra.Command {
	cmd := &cobra.Command{Use: "engine", Short: "a project's switches for the OpenSpec plan engine and the agent-written solution"}
	cmd.AddCommand(newEngineShowCmd(e), newEngineSetCmd(e))
	return cmd
}

func newEngineShowCmd(e flowEnv) *cobra.Command {
	var project string
	var check bool
	cmd := &cobra.Command{
		Use: "show", Short: "read a project's engine switches (--check also asks its dev server whether they can run)", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if project == "" {
				return e.usage("--project is required")
			}
			return e.run(cmd, func(ctx context.Context, cli *apiclient.Client) (apiclient.Document, error) {
				return cli.GetEngineSettings(ctx, project, check)
			}, renderEngineSettings)
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "project id")
	cmd.Flags().BoolVar(&check, "check", false, "run the readiness check on the project's dev server")
	return cmd
}

// newEngineSetCmd keeps what the flags leave out: an administrator turning one switch must not flip the other, so the
// current settings are read first and the version they carry is sent back with the change.
func newEngineSetCmd(e flowEnv) *cobra.Command {
	var project, engine, minVersion string
	var solutionAgent, planAutoGenerate bool
	var expected int64
	cmd := &cobra.Command{
		Use: "set", Short: "change a project's engine switches (administrators only)", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if project == "" {
				return e.usage("--project is required")
			}
			flags := cmd.Flags()
			if !flags.Changed("engine") && !flags.Changed("solution-agent") && !flags.Changed("min-version") && !flags.Changed("plan-auto-generate") {
				return e.usage("name what to change: --engine, --solution-agent, --min-version or --plan-auto-generate")
			}
			return e.run(cmd, func(ctx context.Context, cli *apiclient.Client) (apiclient.Document, error) {
				cur, err := cli.GetEngineSettings(ctx, project, false)
				if err != nil {
					return nil, err
				}
				body := apiclient.Document{
					"engine": firstOr(cur.Str("engine"), "native"), "openspecMinVersion": cur.Str("openspecMinVersion"),
					"solutionAgent": cur.Bool("solutionAgent"), "expectedVersion": cur.Int("version"),
				}
				if flags.Changed("engine") {
					body["engine"] = engine
				}
				if flags.Changed("min-version") {
					body["openspecMinVersion"] = minVersion
				}
				if flags.Changed("solution-agent") {
					body["solutionAgent"] = solutionAgent
				}
				// Only sent when named: an absent key keeps the project's stored switch (CR-REQ-044).
				if flags.Changed("plan-auto-generate") {
					body["planAutoGenerate"] = planAutoGenerate
				}
				if flags.Changed("expected-version") {
					body["expectedVersion"] = expected
				}
				return cli.SetEngineSettings(ctx, project, body)
			}, renderEngineSettings)
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "project id")
	cmd.Flags().StringVar(&engine, "engine", "", "plan engine: native or openspec")
	cmd.Flags().BoolVar(&solutionAgent, "solution-agent", false, "write the Solution with the read-only agent and the knowledge graph")
	cmd.Flags().BoolVar(&planAutoGenerate, "plan-auto-generate", true, "start the Plan by itself once the Analysis is approved (use =false to turn off for the project)")
	cmd.Flags().StringVar(&minVersion, "min-version", "", "lowest openspec version the dev server must have (empty clears it)")
	cmd.Flags().Int64Var(&expected, "expected-version", 0, "version the change is based on (default: the version just read)")
	return cmd
}

func renderEngineSettings(doc apiclient.Document) string {
	var b strings.Builder
	fmt.Fprintf(&b, "project %s  version %d\n", doc.Str("projectId"), doc.Int("version"))
	fmt.Fprintf(&b, "  plan engine: %s", firstOr(doc.Str("engine"), "native"))
	if v := doc.Str("openspecMinVersion"); v != "" {
		fmt.Fprintf(&b, " (openspec >= %s)", v)
	}
	fmt.Fprintf(&b, "\n  solution by agent: %s\n", onOff(doc.Bool("solutionAgent")))
	if _, present := doc["planAutoGenerate"]; present { // an older gateway does not send it; "off" would be a guess
		fmt.Fprintf(&b, "  plan auto-generate: %s\n", onOff(doc.Bool("planAutoGenerate")))
	}
	pre := doc.Doc("preflight")
	if pre == nil {
		return strings.TrimRight(b.String(), "\n")
	}
	if !pre.Bool("ran") {
		b.WriteString("  readiness: not checked\n")
	} else if pre.Bool("ok") {
		fmt.Fprintf(&b, "  readiness: ready (openspec %s)\n", firstOr(pre.Str("openspecVersion"), "not probed"))
	} else {
		b.WriteString("  readiness: NOT ready; the old mode keeps working\n")
	}
	writeNotes(&b, "blocked", pre.Docs("blockers"))
	writeNotes(&b, "knowledge hub unused", pre.Docs("gaps"))
	for _, w := range pre.Strs("warnings") {
		fmt.Fprintf(&b, "    warning: %s\n", w)
	}
	return strings.TrimRight(b.String(), "\n")
}

func writeNotes(b *strings.Builder, label string, notes []apiclient.Document) {
	for _, n := range notes {
		fmt.Fprintf(b, "    %s: %s (%s)\n", label, n.Str("message"), n.Str("code"))
		if h := n.Str("fixHint"); h != "" {
			fmt.Fprintf(b, "      fix: %s\n", h)
		}
	}
}

func onOff(v bool) string {
	if v {
		return "on"
	}
	return "off"
}
