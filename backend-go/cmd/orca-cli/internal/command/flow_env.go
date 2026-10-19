package command

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/apiclient"
	"github.com/stablyai/orca-go/cmd/orca-cli/internal/output"
)

// flowEnv is what every Request-flow command needs: the client seam, the --json switch and the exit-code slot.
type flowEnv struct {
	factory func() (*apiclient.Client, error)
	json    *bool
	exit    *int
}

type docCall func(ctx context.Context, cli *apiclient.Client) (apiclient.Document, error)

// run builds the client, makes the call and prints the outcome; render may be nil (then the JSON is printed).
func (e flowEnv) run(cmd *cobra.Command, call docCall, render func(apiclient.Document) string) error {
	*e.exit = RunFlowCall(cmd.Context(), e.factory, *e.json, call, render)
	return nil
}

// RunFlowCall is the testable core of every Request-flow command.
func RunFlowCall(ctx context.Context, factory func() (*apiclient.Client, error), jsonMode bool, call docCall, render func(apiclient.Document) string) int {
	cli, err := factory()
	if err != nil {
		return output.ReportError(err, jsonMode)
	}
	doc, err := call(ctx, cli)
	if err != nil {
		return output.ReportError(err, jsonMode)
	}
	human := ""
	if render != nil {
		human = render(doc)
	}
	return output.ReportDocument(doc, human, jsonMode)
}

// usage reports a bad invocation (exit 2) before any client is built or any request is sent.
func (e flowEnv) usage(format string, a ...any) error {
	*e.exit = output.ReportError(&apiclient.CLIError{StatusCode: 400, Code: "INVALID_ARGUMENT", Message: fmt.Sprintf(format, a...)}, *e.json)
	return nil
}

// versionFor returns the version the caller named, or the Request's current one: the person acts on what is on
// screen now, and the service still refuses a stale write.
func versionFor(ctx context.Context, cli *apiclient.Client, requestID string, given int64) (int64, error) {
	if given != 0 {
		return given, nil
	}
	doc, err := cli.GetRequest(ctx, requestID)
	if err != nil {
		return 0, err
	}
	return doc.Int("request", "version"), nil
}

// withOptional adds key to body only when the flag was given, so the service default (or "no value") stays.
func withOptional(body apiclient.Document, key, value string) {
	if value = strings.TrimSpace(value); value != "" {
		body[key] = value
	}
}

// readBodyArg returns text, or the content of the file named by path ("-" is standard input).
func readBodyArg(text, path string) (string, error) {
	if path == "" {
		return text, nil
	}
	if text != "" {
		return "", fmt.Errorf("give the text or its file, not both")
	}
	var raw []byte
	var err error
	if path == "-" {
		raw, err = readAllStdin()
	} else {
		raw, err = os.ReadFile(path)
	}
	return string(raw), err
}
