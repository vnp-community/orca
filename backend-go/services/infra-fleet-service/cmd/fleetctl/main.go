// fleetctl is a thin HTTPS client of api-gateway's authenticated REST
// surface for BL-FLEET-01's fleet inventory operations — deliberately NOT a
// direct gRPC caller of infra-fleet-service, so it reuses api-gateway's
// existing JWT auth wholesale rather than needing its own NetworkPolicy
// exception into the mesh. Subcommand dispatch uses stdlib flag, mirroring
// the `go` tool's own convention — no new CLI-framework dependency.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/cmd/fleetctl/fleetyaml"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "import":
		err = runImport(os.Args[2:])
	case "list":
		err = runList(os.Args[2:])
	case "status":
		err = runStatus(os.Args[2:])
	case "-h", "--help", "help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "fleetctl: unknown subcommand %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "fleetctl: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `fleetctl — BL-FLEET-01 fleet inventory CLI

Usage:
  fleetctl import --file orca-fleet.yaml [--dry-run] --api-base <url> --token <bearer>
  fleetctl list [--project X] --api-base <url> --token <bearer>
  fleetctl status --api-base <url> --token <bearer>`)
}

// httpClient is shared across subcommands — a modest timeout so a hung
// api-gateway doesn't wedge the CLI forever.
var httpClient = &http.Client{Timeout: 30 * time.Second}

func runImport(args []string) error {
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	file := fs.String("file", "", "path to the fleet inventory YAML file")
	dryRun := fs.Bool("dry-run", false, "validate and preview without writing")
	apiBase := fs.String("api-base", "", "api-gateway base URL, e.g. https://api.orca.internal")
	token := fs.String("token", "", "bearer token for Authorization")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *file == "" || *apiBase == "" || *token == "" {
		return fmt.Errorf("import: --file, --api-base, and --token are required")
	}

	data, err := os.ReadFile(*file)
	if err != nil {
		return fmt.Errorf("reading %s: %w", *file, err)
	}
	parsed, err := fleetyaml.Parse(data)
	if err != nil {
		return fmt.Errorf("parsing %s: %w", *file, err)
	}
	for _, w := range parsed.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}

	body, err := json.Marshal(struct {
		Servers []fleetyaml.ServerInput `json:"servers"`
		DryRun  bool                    `json:"dryRun"`
	}{Servers: parsed.Servers, DryRun: *dryRun})
	if err != nil {
		return fmt.Errorf("marshaling import request: %w", err)
	}

	var result struct {
		Imported int   `json:"imported"`
		Updated  int   `json:"updated"`
		Skipped  int   `json:"skipped"`
		Errors   []any `json:"errors"`
	}
	if err := doJSON(*apiBase, *token, http.MethodPost, "/v1/infra/fleet/import", body, &result); err != nil {
		return err
	}

	fmt.Printf("imported=%d updated=%d skipped=%d\n", result.Imported, result.Updated, result.Skipped)
	if len(result.Errors) > 0 {
		errsJSON, _ := json.MarshalIndent(result.Errors, "", "  ")
		fmt.Printf("errors:\n%s\n", errsJSON)
	}
	return nil
}

func runList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	project := fs.String("project", "", "filter by project (client-side)")
	apiBase := fs.String("api-base", "", "api-gateway base URL")
	token := fs.String("token", "", "bearer token for Authorization")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *apiBase == "" || *token == "" {
		return fmt.Errorf("list: --api-base and --token are required")
	}

	var result struct {
		SSHTargets []struct {
			ID      string   `json:"id"`
			Host    string   `json:"host"`
			User    string   `json:"user"`
			Project string   `json:"project"`
			Tags    []string `json:"tags"`
		} `json:"sshTargets"`
	}
	if err := doJSON(*apiBase, *token, http.MethodGet, "/v1/infra/ssh-targets", nil, &result); err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "HOST\tUSER\tPROJECT\tTAGS")
	for _, t := range result.SSHTargets {
		if *project != "" && t.Project != *project {
			continue
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", t.Host, t.User, t.Project, strings.Join(t.Tags, ","))
	}
	return w.Flush()
}

func runStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	apiBase := fs.String("api-base", "", "api-gateway base URL")
	token := fs.String("token", "", "bearer token for Authorization")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *apiBase == "" || *token == "" {
		return fmt.Errorf("status: --api-base and --token are required")
	}

	var result struct {
		Statuses []struct {
			DevServerID string  `json:"devServerId"`
			Reachable   bool    `json:"reachable"`
			CPUPercent  float64 `json:"cpuPercent"`
			RAMPercent  float64 `json:"ramPercent"`
			DiskPercent float64 `json:"diskPercent"`
			LatencyMs   int64   `json:"latencyMs"`
		} `json:"statuses"`
	}
	if err := doJSON(*apiBase, *token, http.MethodGet, "/v1/infra/health", nil, &result); err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "DEV_SERVER\tREACHABLE\tCPU%\tRAM%\tDISK%\tLATENCY_MS")
	for _, s := range result.Statuses {
		fmt.Fprintf(w, "%s\t%t\t%.1f\t%.1f\t%.1f\t%d\n", s.DevServerID, s.Reachable, s.CPUPercent, s.RAMPercent, s.DiskPercent, s.LatencyMs)
	}
	return w.Flush()
}

// doJSON issues an HTTPS request against apiBase+path, decoding a JSON
// response body into out (skipped when out is nil). A non-2xx response is
// surfaced as an error carrying the response body verbatim — api-gateway's
// error responses are the most useful diagnostic fleetctl can show.
func doJSON(apiBase, token, method, path string, body []byte, out any) error {
	url := strings.TrimRight(apiBase, "/") + path
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s: %s: %s", method, path, resp.Status, strings.TrimSpace(string(respBody)))
	}
	if out == nil || len(respBody) == 0 {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}
	return nil
}
