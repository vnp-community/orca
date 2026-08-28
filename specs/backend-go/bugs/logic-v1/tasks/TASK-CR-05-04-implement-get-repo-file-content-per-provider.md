# TASK-CR-05-04: Implement `GetRepoFileContent` on every `ScmProvider` adapter

**From Solution:** SOL-CR-05
**Priority:** P0
**Service:** `scm-integration-service`
**File:** `backend-go/services/scm-integration-service/internal/adapter/github/client.go`, `.../gitlab/client.go`, `.../gitea/client.go`, `.../azuredevops/client.go`, `.../bitbucket/client.go`
**Depends on:** TASK-CR-05-02
**Status:** `[ ]` TODO

---

## Context

`ports.go`'s `ScmProvider` interface (TASK-CR-05-02) is not satisfied by
any adapter until `GetRepoFileContent` is implemented everywhere — this is
required for `go build ./...` to pass again. A 404 maps to `found=false,
err=nil`, not an error, consistent with `BranchExists`/
`GetPullRequestForBranch`'s existing "not found is a valid answer, not a
failure" convention in this package.

## Changes to make

### GitHub — `GET /repos/{owner}/{repo}/contents/{path}?ref=`

```go
// GetRepoFileContent fetches one file's raw content at ref via GitHub's
// Contents API. found=false (not an error) on a 404 — the expected case
// for "no CODEOWNERS file".
func (c *Client) GetRepoFileContent(ctx context.Context, cred usecase.Credential, repo, path, ref string) (string, bool, error) {
	reqURL := fmt.Sprintf("%s/repos/%s/contents/%s?ref=%s", c.baseURL, repo, path, url.QueryEscape(ref))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return "", false, fmt.Errorf("github: build get repo file content request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cred.Token)
	req.Header.Set("Accept", "application/vnd.github.raw+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("github: get repo file content request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return "", false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("github: get repo file content: unexpected status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false, fmt.Errorf("github: read repo file content response: %w", err)
	}
	return string(body), true, nil
}
```

### GitLab — `GET /projects/:id/repository/files/:file_path/raw?ref=`

```go
func (c *Client) GetRepoFileContent(ctx context.Context, cred usecase.Credential, repo, path, ref string) (string, bool, error) {
	reqURL := fmt.Sprintf("%s/projects/%s/repository/files/%s/raw?ref=%s", c.baseURL, projectPath(repo), url.PathEscape(path), url.QueryEscape(ref))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return "", false, fmt.Errorf("gitlab: build get repo file content request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cred.Token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("gitlab: get repo file content request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return "", false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("gitlab: get repo file content: unexpected status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false, fmt.Errorf("gitlab: read repo file content response: %w", err)
	}
	return string(body), true, nil
}
```

### Bitbucket — `GET /2.0/repositories/{workspace}/{repo}/src/{ref}/{path}`

```go
func (c *Client) GetRepoFileContent(ctx context.Context, cred usecase.Credential, repo, path, ref string) (string, bool, error) {
	reqURL := fmt.Sprintf("%s/repositories/%s/src/%s/%s", c.baseURL, repo, url.PathEscape(ref), path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return "", false, fmt.Errorf("bitbucket: build get repo file content request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cred.Token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("bitbucket: get repo file content request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return "", false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("bitbucket: get repo file content: unexpected status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false, fmt.Errorf("bitbucket: read repo file content response: %w", err)
	}
	return string(body), true, nil
}
```

### Azure DevOps — Items API

```go
func (c *Client) GetRepoFileContent(ctx context.Context, cred usecase.Credential, repo, path, ref string) (string, bool, error) {
	org, project, repositoryID, err := splitRepo(repo)
	if err != nil {
		return "", false, err
	}
	reqURL := fmt.Sprintf("%s/%s/%s/_apis/git/repositories/%s/items?path=%s&version=%s&api-version=%s",
		c.baseURL, org, project, repositoryID, url.QueryEscape(path), url.QueryEscape(ref), apiVersion)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return "", false, fmt.Errorf("azuredevops: build get repo file content request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cred.Token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("azuredevops: get repo file content request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return "", false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("azuredevops: get repo file content: unexpected status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false, fmt.Errorf("azuredevops: read repo file content response: %w", err)
	}
	return string(body), true, nil
}
```

### Gitea — `GET /repos/{owner}/{repo}/raw/{path}?ref=`

```go
func (c *Client) GetRepoFileContent(ctx context.Context, cred usecase.Credential, repo, path, ref string) (string, bool, error) {
	reqURL := fmt.Sprintf("%s/repos/%s/raw/%s?ref=%s", c.baseURL, repo, path, url.QueryEscape(ref))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return "", false, fmt.Errorf("gitea: build get repo file content request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cred.Token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("gitea: get repo file content request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return "", false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("gitea: get repo file content: unexpected status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false, fmt.Errorf("gitea: read repo file content response: %w", err)
	}
	return string(body), true, nil
}
```

Add `"io"` and `"net/url"` to each file's imports if not already present
(check first — several of these clients already import `net/url` for
existing methods like GitHub's `GetPullRequestForBranch`).

## Verify

```bash
cd /opt/repos/orca/backend-go/services/scm-integration-service
go build ./...
go test ./internal/adapter/... -run TestGetRepoFileContent -v
```

Expected: `go build ./...` now passes clean (every `ScmProvider` adapter
satisfies the interface again). Add one test per adapter: 200 → returns
`(content, true, nil)`; 404 → returns `("", false, nil)`; non-200/404 →
returns a non-nil error.
