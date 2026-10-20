package command

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/apiclient"
)

func newReviewCmd(e flowEnv) *cobra.Command {
	cmd := &cobra.Command{Use: "review", Short: "the Outcome Review of a Request"}
	profile := &cobra.Command{Use: "profile", Short: "how the review's diagrams are drawn: the tenant layer, a project layer, and the profile in force"}
	profile.AddCommand(newReviewProfileShowCmd(e), newReviewProfileSetCmd(e), newReviewProfileResetCmd(e))
	cmd.AddCommand(profile)
	return cmd
}

func newReviewProfileShowCmd(e flowEnv) *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use: "show", Short: "read the drawing profile in force and where each value came from (default, tenant or project)", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return e.run(cmd, func(ctx context.Context, cli *apiclient.Client) (apiclient.Document, error) {
				return cli.GetReviewProfile(ctx, project)
			}, renderReviewProfile)
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "project id (leave out for the tenant layer)")
	return cmd
}

// newReviewProfileSetCmd reads the layer's current version first, like `engine set`, so a person changing one thing does not need to
// look the version up; the service still refuses a write based on a version that has moved.
func newReviewProfileSetCmd(e flowEnv) *cobra.Command {
	var project, spec, file string
	var expected int64
	cmd := &cobra.Command{
		Use: "set", Short: "save the tenant's or a project's drawing profile layer (administrators only)", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			text, err := readBodyArg(spec, file)
			if err != nil {
				return e.usage("%v", err)
			}
			if strings.TrimSpace(text) == "" {
				return e.usage("give the profile with --spec '<json>' or --file <path> (\"-\" reads standard input)")
			}
			var parsed any
			if err := json.Unmarshal([]byte(text), &parsed); err != nil {
				return e.usage("the profile is not valid JSON: %v", err)
			}
			return e.run(cmd, func(ctx context.Context, cli *apiclient.Client) (apiclient.Document, error) {
				version := expected
				if !cmd.Flags().Changed("expected-version") {
					cur, err := cli.GetReviewProfile(ctx, project)
					if err != nil {
						return nil, err
					}
					version = cur.Int(layerKey(project), "version")
				}
				return cli.SaveReviewProfile(ctx, project, apiclient.Document{"spec": parsed, "expectedVersion": version})
			}, renderReviewProfileLayer)
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "project id (leave out for the tenant layer)")
	cmd.Flags().StringVar(&spec, "spec", "", "the profile as JSON, for example '{\"schema\":\"review-drawing-profile/1\",\"style\":{\"detail\":\"detailed\"}}'")
	cmd.Flags().StringVar(&file, "file", "", "read the profile from this file (\"-\" is standard input)")
	cmd.Flags().Int64Var(&expected, "expected-version", 0, "version the change is based on (default: the layer's version just read; 0 for a layer not saved yet)")
	return cmd
}

func newReviewProfileResetCmd(e flowEnv) *cobra.Command {
	var project string
	var expected int64
	cmd := &cobra.Command{
		Use: "reset", Short: "remove the tenant's or a project's layer, so the layer below applies again (administrators only)", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return e.run(cmd, func(ctx context.Context, cli *apiclient.Client) (apiclient.Document, error) {
				version := expected
				if !cmd.Flags().Changed("expected-version") {
					cur, err := cli.GetReviewProfile(ctx, project)
					if err != nil {
						return nil, err
					}
					version = cur.Int(layerKey(project), "version")
				}
				return cli.ResetReviewProfile(ctx, project, apiclient.Document{"expectedVersion": version})
			}, func(apiclient.Document) string { return "layer removed; the profile below it applies again\n" })
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "project id (leave out for the tenant layer)")
	cmd.Flags().Int64Var(&expected, "expected-version", 0, "version the change is based on (default: the layer's version just read)")
	return cmd
}

func layerKey(project string) string {
	if project == "" {
		return "tenantLayer"
	}
	return "projectLayer"
}

func renderReviewProfile(doc apiclient.Document) string {
	var b strings.Builder
	for _, l := range []struct{ key, label string }{{"tenantLayer", "tenant layer"}, {"projectLayer", "project layer"}} {
		if layer := doc.Doc(l.key); layer != nil {
			fmt.Fprintf(&b, "%s: version %d, saved by %s at %s\n", l.label, layer.Int("version"), firstOr(layer.Str("updatedBy"), "-"), firstOr(layer.Str("updatedAt"), "-"))
		} else {
			fmt.Fprintf(&b, "%s: none\n", l.label)
		}
	}
	if eff, err := json.MarshalIndent(doc["effective"], "  ", "  "); err == nil {
		fmt.Fprintf(&b, "in force:\n  %s\n", eff)
	}
	if src, ok := doc["sources"].(map[string]any); ok && len(src) > 0 {
		keys := make([]string, 0, len(src))
		for k := range src {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteString("where each value came from:\n")
		for _, k := range keys {
			fmt.Fprintf(&b, "  %-32s %v\n", k, src[k])
		}
	}
	return b.String()
}

func renderReviewProfileLayer(doc apiclient.Document) string {
	layer := doc.Doc("layer")
	return fmt.Sprintf("saved: version %d, digest %s\n", layer.Int("version"), firstOr(layer.Str("specDigest"), "-"))
}
