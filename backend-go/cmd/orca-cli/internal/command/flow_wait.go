package command

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/apiclient"
)

// waiter polls the gateway until an asynchronous step (analysis, plan run, the Request's own transition) finishes.
type waiter struct {
	interval time.Duration
	progress io.Writer
}

func defaultWaiter() waiter { return waiter{interval: 2 * time.Second, progress: os.Stderr} }

func waitTimeout(what string, d time.Duration) error {
	return &apiclient.CLIError{StatusCode: 408, Code: "WAIT_TIMEOUT", Message: fmt.Sprintf("%s did not finish within %s", what, d)}
}

// terminalRequestStatuses end every wait: nothing more will happen to the Request by itself.
var terminalRequestStatuses = map[string]bool{"completed": true, "cancelled": true}

// requestStatus polls request.get until the status is in until (or terminal) and returns the Request document.
func (w waiter) requestStatus(ctx context.Context, cli *apiclient.Client, id string, until map[string]bool, timeout time.Duration) (apiclient.Document, error) {
	deadline := time.Now().Add(timeout)
	last := ""
	for {
		doc, err := cli.GetRequest(ctx, id)
		if err != nil {
			return nil, err
		}
		status := doc.Str("request", "status")
		if until[status] || terminalRequestStatuses[status] {
			return doc, nil
		}
		if status != last {
			fmt.Fprintf(w.progress, "request %s: %s\n", id, status)
			last = status
		}
		if err := w.sleep(ctx, deadline, "the Request", timeout); err != nil {
			return nil, err
		}
	}
}

// analysisRun polls solution.list until the run the generate call started has finished; a failed run is an error.
func (w waiter) analysisRun(ctx context.Context, cli *apiclient.Client, requestID, runID string, timeout time.Duration) (apiclient.Document, error) {
	deadline := time.Now().Add(timeout)
	for {
		doc, err := cli.ListSolutions(ctx, requestID, nil)
		if err != nil {
			return nil, err
		}
		for _, run := range doc.Docs("runs") {
			if run.Str("id") != runID || run.Str("finishedAt") == "" {
				continue
			}
			if code := run.Str("errorCode"); code != "" {
				return nil, &apiclient.CLIError{StatusCode: 500, Code: code, Message: run.Str("errorMessage")}
			}
			return doc, nil
		}
		if err := w.sleep(ctx, deadline, "the analysis", timeout); err != nil {
			return nil, err
		}
	}
}

// planProposal polls request.planProposal until the run produced a proposal; a failed run is an error.
func (w waiter) planProposal(ctx context.Context, cli *apiclient.Client, requestID, runID string, timeout time.Duration) (apiclient.Document, error) {
	deadline := time.Now().Add(timeout)
	for {
		doc, err := cli.GetPlanProposal(ctx, requestID, runID)
		if err != nil {
			return nil, err
		}
		if doc.Str("status") == "failed" {
			code := doc.Str("errorCode")
			if code == "" {
				code = "REQUEST_INTERNAL"
			}
			return nil, &apiclient.CLIError{StatusCode: 500, Code: code, Message: doc.Str("errorMessage")}
		}
		if doc.Str("status") != "running" && doc["proposal"] != nil {
			return doc, nil
		}
		if err := w.sleep(ctx, deadline, "the plan run", timeout); err != nil {
			return nil, err
		}
	}
}

func (w waiter) sleep(ctx context.Context, deadline time.Time, what string, timeout time.Duration) error {
	if !time.Now().Add(w.interval).Before(deadline) {
		return waitTimeout(what, timeout)
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(w.interval):
		return nil
	}
}
