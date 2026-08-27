# TASK-CR-03-04: Wire `registerAnnotationSendChannel` into `RegisterRealChannels`

**From Solution:** SOL-CR-03
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`
**Depends on:** TASK-CR-03-03
**Status:** `[ ]` TODO

---

## Context

`RegisterRealChannels` (`channels.go:71-127`) already calls
`registerRepoSshStatusWorkspaceChannels(r, projectClient, gitClient,
infraFleetClient)` and `registerWorktreeChannels(r, gitClient,
projectClient)` — cross-client registrars living in their own
`channels_*.go` files, wired in from this one central place. This task
adds one more call in that same "final integration pass" block.

## Changes to make

In `RegisterRealChannels`, add a call alongside the other
`channels_*.go`-file registrars (after `registerAnnotationChannels`, since
both operate on `annotation.*` and this keeps related registrations
grouped):

```go
	registerAnnotationChannels(r, annotationClient)
	registerAnnotationSendChannel(r, annotationClient, gitClient) // NEW — SOL-CR-03
	registerTaskChannels(r, taskClient)
```

No new parameter is needed on `RegisterRealChannels` itself —
`annotationClient` and `gitClient` are already parameters.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./...
go vet ./...
go test ./internal/adapter/wscompat/... -run TestRegisterRealChannels -v
```

Confirm `annotation.sendToAgent` is registered by asserting
`r.Has("annotation.sendToAgent")` (or the equivalent existing assertion
pattern `channels_test.go` uses for other real channels) in
`channels_test.go`.
