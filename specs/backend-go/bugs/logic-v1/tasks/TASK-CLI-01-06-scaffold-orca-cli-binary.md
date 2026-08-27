# TASK-CLI-01-06: Scaffold `backend-go/cmd/orca-cli/` — new module, credentials, REST client core

**From Solution:** SOL-CLI-01
**Priority:** P1
**Service:** new `orca-cli` binary (`backend-go/cmd/orca-cli/`)
**File:** `backend-go/cmd/orca-cli/go.mod`
**Depends on:** none (this scaffolding compiles standalone; TASK-CLI-01-07 wires it to the real routes)
**Status:** [x] DONE — created `backend-go/cmd/orca-cli` module (go.mod, internal/config/credentials.go, internal/apiclient/{client,errors}.go), added to `go.work`; builds standalone and from workspace root.

---

## Context

BUG-CLI-01 establishes there is no CLI package anywhere under `backend-go/`. Not a service — no `.proto`, no gRPC/HTTP server, no database — so it lives outside `services/`, at `backend-go/cmd/orca-cli/`, its own Go module per `04-tech-stack.md`'s "one Go module per deployable" convention. This task creates the module and the two lowest-level pieces every later command needs: credential storage and the bearer-token-injecting HTTP client.

## Changes to make

**1. `backend-go/cmd/orca-cli/go.mod`:**

```
module github.com/stablyai/orca-go/cmd/orca-cli

go 1.25.0

require github.com/spf13/cobra v1.8.1
```

Go version matches every other module's `go.mod` (`go 1.25.0`, confirmed in `backend-go/go.work` and `services/api-gateway/go.mod`). `cobra` is a new dependency — no existing service uses it — but is the de facto standard for a Go CLI's command tree (`--json`/subcommand flags) and is explicitly named in SOL-CLI-01's `command/root.go` design.

Add the new module to the workspace, `backend-go/go.work`'s `use` block:

```
use (
	./common
	./proto
	./cmd/orca-cli
	./services/ai-provider-service
	...
)
```

**2. `backend-go/cmd/orca-cli/internal/config/credentials.go`:**

```go
// Package config manages orca-cli's on-disk credentials — the JWT
// /auth/cli-token issues, cached so every command doesn't re-login.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Credentials is the on-disk shape at ~/.config/orca/credentials.json —
// 0600, since it holds a live bearer JWT.
type Credentials struct {
	APIURL    string    `json:"api_url"`
	JWT       string    `json:"jwt"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Path returns ~/.config/orca/credentials.json, honoring $XDG_CONFIG_HOME
// when set (cross-platform per AGENTS.md — os.UserConfigDir already
// resolves the right base dir on macOS/Linux/Windows).
func Path() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "orca", "credentials.json"), nil
}

// Load reads Credentials from disk. A missing file returns the zero value
// and a nil error — "not logged in yet" is not a hard failure.
func Load() (Credentials, error) {
	path, err := Path()
	if err != nil {
		return Credentials{}, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Credentials{}, nil
	}
	if err != nil {
		return Credentials{}, err
	}
	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return Credentials{}, err
	}
	return creds, nil
}

// Save writes creds to disk with 0600 permissions — never world/group
// readable, since this file holds a live bearer JWT.
func Save(creds Credentials) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// ResolveAPIURL applies the ORCA_API_URL env override, falling back to
// creds.APIURL, falling back to def.
func ResolveAPIURL(creds Credentials, def string) string {
	if v := os.Getenv("ORCA_API_URL"); v != "" {
		return v
	}
	if creds.APIURL != "" {
		return creds.APIURL
	}
	return def
}

// ResolveToken applies the ORCA_API_TOKEN env override, falling back to
// creds.JWT.
func ResolveToken(creds Credentials) string {
	if v := os.Getenv("ORCA_API_TOKEN"); v != "" {
		return v
	}
	return creds.JWT
}
```

**3. `backend-go/cmd/orca-cli/internal/apiclient/client.go`:**

```go
// Package apiclient is orca-cli's thin REST client — one method per
// api-gateway endpoint this CLI calls. Never imports any service's
// internal/ package (SOL-CLI-01's dependency-inversion boundary) — only
// stdlib net/http and encoding/json.
package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Client wraps http.Client with bearer-token injection and JSON
// marshal/unmarshal for every apiclient method.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func New(baseURL, token string) *Client {
	return &Client{baseURL: baseURL, token: token, http: &http.Client{}}
}

// do issues one JSON request/response round trip. A non-2xx status maps
// the response body through errors.go's decodeErrorBody rather than
// returning a raw *http.Response for every caller to re-check.
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("apiclient: marshal request: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("apiclient: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("apiclient: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("apiclient: read response body: %w", err)
	}
	if resp.StatusCode >= 300 {
		return decodeErrorBody(resp.StatusCode, respBody)
	}
	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("apiclient: decode response: %w", err)
		}
	}
	return nil
}
```

**4. `backend-go/cmd/orca-cli/internal/apiclient/errors.go`:**

```go
package apiclient

import "encoding/json"

// CLIError maps api-gateway's writeJSONError {code, message} shape
// (git_routes.go and every other handler already use it) into a typed,
// catchable error the CLI's exit-code mapping (see internal/output) keys
// off directly.
type CLIError struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *CLIError) Error() string {
	return e.Code + ": " + e.Message
}

// errorBody mirrors api-gateway's writeJSONError JSON shape:
// {"error": {"code": "...", "message": "..."}}.
type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func decodeErrorBody(statusCode int, body []byte) error {
	var eb errorBody
	if err := json.Unmarshal(body, &eb); err != nil || eb.Error.Code == "" {
		return &CLIError{StatusCode: statusCode, Code: "UNKNOWN", Message: string(body)}
	}
	return &CLIError{StatusCode: statusCode, Code: eb.Error.Code, Message: eb.Error.Message}
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go/cmd/orca-cli
go build ./...
cd /opt/repos/orca/backend-go
go build ./cmd/orca-cli/...
```

Expected: the new module builds standalone AND from the workspace root (confirms the `go.work` edit is correct).
