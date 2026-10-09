package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/stablyai/orca-go/common/health"
)

// healthcheckArg makes the binary probe its own /readyz and exit: the dev image is distroless (no wget, no curl),
// so the compose healthcheck can only run the service binary itself.
const healthcheckArg = "healthcheck"

// runHealthcheck returns the process exit code: 0 when /readyz answers 200 within the budget, 1 otherwise.
func runHealthcheck(httpPort int, stderr io.Writer) int {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	url := fmt.Sprintf("http://127.0.0.1:%d/readyz", httpPort)
	if err := health.Probe(ctx, &http.Client{}, url); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
