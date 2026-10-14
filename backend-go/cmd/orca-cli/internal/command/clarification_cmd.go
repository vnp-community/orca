package command

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/apiclient"
)

func newClarificationCmd(e flowEnv) *cobra.Command {
	cmd := &cobra.Command{Use: "clarification", Short: "questions the AI asked about a Request, and your answers"}
	cmd.AddCommand(newClarificationListCmd(e), newClarificationAskCmd(e), newClarificationAnswerCmd(e))
	return cmd
}

func newClarificationListCmd(e flowEnv) *cobra.Command {
	var status string
	cmd := &cobra.Command{
		Use: "list <request-id>", Short: "list the clarifications of a Request (the open one holds the questions)", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			q := url.Values{}
			if status != "" {
				q.Set("status", status)
			}
			return e.run(cmd, func(ctx context.Context, cli *apiclient.Client) (apiclient.Document, error) {
				return cli.ListClarifications(ctx, args[0], q)
			}, nil)
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "open, answered, expired or cancelled")
	return cmd
}

// answerItem turns "question=value" into an AnswerItem. Text answers (rawJSON false) go as a JSON string, which is what
// the string and list questions expect (a list is one item per line); rawJSON sends the value as given.
func answerItem(spec string, rawJSON bool) (apiclient.Document, error) {
	id, value, ok := strings.Cut(spec, "=")
	if !ok || id == "" {
		return nil, &apiclient.CLIError{StatusCode: 400, Code: "INVALID_ARGUMENT", Message: "an answer needs question-id=value, got " + spec}
	}
	if rawJSON {
		if !json.Valid([]byte(value)) {
			return nil, &apiclient.CLIError{StatusCode: 400, Code: "INVALID_ARGUMENT", Message: "--answer-json value is not valid JSON: " + value}
		}
	} else {
		quoted, _ := json.Marshal(value)
		value = string(quoted)
	}
	return apiclient.Document{"questionId": id, "valueJson": value}, nil
}

func newClarificationAnswerCmd(e flowEnv) *cobra.Command {
	var answers, rawAnswers, defaults []string
	var incomplete bool
	var version int64
	cmd := &cobra.Command{
		Use: "answer <clarification-id>", Short: "answer the questions; the Request then resumes", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			items := []apiclient.Document{}
			for _, group := range []struct {
				specs []string
				raw   bool
			}{{answers, false}, {rawAnswers, true}} {
				for _, spec := range group.specs {
					item, err := answerItem(spec, group.raw)
					if err != nil {
						return e.usage("%v", err)
					}
					items = append(items, item)
				}
			}
			for _, id := range defaults {
				items = append(items, apiclient.Document{"questionId": id, "acceptDefault": true})
			}
			if len(items) == 0 {
				return e.usage("give at least one --answer question-id=text or --accept-default question-id")
			}
			return e.run(cmd, func(ctx context.Context, cli *apiclient.Client) (apiclient.Document, error) {
				v := version
				if v == 0 {
					got, err := cli.GetClarification(ctx, args[0])
					if err != nil {
						return nil, err
					}
					v = got.Int("clarification", "version")
				}
				return cli.AnswerClarification(ctx, args[0], apiclient.Document{"answers": items, "complete": !incomplete, "expectedVersion": v})
			}, nil)
		},
	}
	cmd.Flags().StringArrayVar(&answers, "answer", nil, "question-id=text (repeatable; a list question takes one item per line)")
	cmd.Flags().StringArrayVar(&rawAnswers, "answer-json", nil, "question-id=<JSON> for a question that takes a non-string value (repeatable)")
	cmd.Flags().StringArrayVar(&defaults, "accept-default", nil, "question-id whose suggested default you accept (repeatable)")
	cmd.Flags().BoolVar(&incomplete, "partial", false, "save the answers without submitting them yet")
	cmd.Flags().Int64Var(&version, "expected-version", 0, "the clarification version you saw (default: the current one)")
	return cmd
}

// solutionOpenQuestionSource is the Clarification source whose answers the Solution gate looks for.
const solutionOpenQuestionSource = "solution_open_question"

// openQuestionsOf turns the open questions of one Solution into the questions of a Clarification. Only the blocking ones
// stop the approval, so they are the default; all=true asks every question the Solution listed. The key is what the gate
// matches an answer against, so it must stay "open_question:<id>".
func openQuestionsOf(solutions []apiclient.Document, solutionID string, all bool) (qs []apiclient.Document, found bool) {
	for _, s := range solutions {
		if s.Str("id") != solutionID {
			continue
		}
		found = true
		for _, q := range s.Docs("options", "open_questions") {
			if !all && !q.Bool("blocking") {
				continue
			}
			qs = append(qs, apiclient.Document{
				"questionKey": "open_question:" + q.Str("id"), "kind": "text", "required": true,
				"prompt": q.Str("text"), "reason": "the Solution lists this as an open question",
			})
		}
	}
	return qs, found
}

func newClarificationAskCmd(e flowEnv) *cobra.Command {
	var solution string
	var all bool
	cmd := &cobra.Command{
		Use: "ask <request-id> --solution <solution-id>", Args: cobra.ExactArgs(1),
		Short: "turn the blocking open questions of a Solution into a Clarification (the Solution gate needs them answered)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if solution == "" {
				return e.usage("--solution is required: the Clarification is about one Solution's open questions")
			}
			return e.run(cmd, func(ctx context.Context, cli *apiclient.Client) (apiclient.Document, error) {
				listed, err := cli.ListSolutions(ctx, args[0], nil)
				if err != nil {
					return nil, err
				}
				qs, found := openQuestionsOf(listed.Docs("solutions"), solution, all)
				switch {
				case !found:
					return nil, &apiclient.CLIError{StatusCode: 404, Code: "NOT_FOUND", Message: "this Request has no Solution " + solution}
				case len(qs) == 0:
					return nil, &apiclient.CLIError{StatusCode: 400, Code: "INVALID_ARGUMENT", Message: "the Solution has no open question that needs an answer (use --all for the non-blocking ones)"}
				}
				body := apiclient.Document{"source": solutionOpenQuestionSource, "sourceRef": solution, "questions": qs}
				return cli.AskClarification(ctx, args[0], body)
			}, nil)
		},
	}
	cmd.Flags().StringVar(&solution, "solution", "", "the Solution whose open questions to ask")
	cmd.Flags().BoolVar(&all, "all", false, "ask the non-blocking open questions too")
	return cmd
}
