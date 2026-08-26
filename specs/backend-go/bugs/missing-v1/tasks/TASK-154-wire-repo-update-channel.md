# TASK-154: Wire `repo.update` into `registerRepoChannels` (Bucket 2)

**From Solution:** SOL-023 (Bucket 2)
**Priority:** P1
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`
**Depends on:** TASK-151 (`registerRepoChannels` must exist), TASK-153 (`UpdateRepo` RPC must exist)
**Status:** `[ ]` TODO

---

## Context

Joins `registerRepoChannels` (TASK-151) once `project-service`'s
`UpdateRepo` RPC (TASK-152/TASK-153) exists.

## Changes to make

**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`

In `registerRepoChannels`, replace the comment placeholder:

```go
	// repo.update joins here once TASK-153's UpdateRepo RPC exists (TASK-154).
```

with:

```go
	r.Register("repo.update", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type updateArgs struct {
			RepoID      string `json:"repoId"`
			URL         string `json:"url"`
			DisplayName string `json:"displayName"`
		}
		in, err := decodeArg[updateArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := project.UpdateRepo(rpcCtx, &projectv1.UpdateRepoRequest{
			RepoId: in.RepoID, Url: in.URL, DisplayName: in.DisplayName,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetRepo(), nil
	})
```

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./... && go vet ./...
```
