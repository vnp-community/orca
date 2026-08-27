# TASK-AUTH-04-05: Wire `ListSessions`/`UpdateUser` RPC handlers

**From Solution:** SOL-AUTH-04
**Priority:** P1
**Service:** `auth-service` (grpc adapter)
**File:** `backend-go/services/auth-service/internal/adapter/grpc/server.go`
**Depends on:** TASK-AUTH-04-01, TASK-AUTH-04-03, TASK-AUTH-04-04
**Status:** `[ ]` TODO

---

## Context

The two new proto RPCs (TASK-AUTH-04-01) and their usecases (TASK-AUTH-04-03, TASK-AUTH-04-04) need gRPC server methods, following the existing `apperrors.ToGRPCStatus(err)` pattern every other RPC handler in this file uses.

## Changes to make

In `backend-go/services/auth-service/internal/adapter/grpc/server.go`, add:

```go
func (s *Server) ListSessions(ctx context.Context, req *authv1.ListSessionsRequest) (*authv1.ListSessionsResponse, error) {
	out, err := s.listSessions.Execute(ctx, usecase.ListSessionsInput{
		PageToken: req.GetPageToken(),
		PageSize:  req.GetPageSize(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	sessions := make([]*authv1.SessionWithUser, 0, len(out.Sessions))
	for _, sw := range out.Sessions {
		sessions = append(sessions, &authv1.SessionWithUser{
			Session:   toProtoSession(sw.Session),
			UserEmail: sw.UserEmail,
		})
	}
	return &authv1.ListSessionsResponse{Sessions: sessions, NextPageToken: out.NextPageToken}, nil
}

func (s *Server) UpdateUser(ctx context.Context, req *authv1.UpdateUserRequest) (*authv1.UpdateUserResponse, error) {
	in := usecase.UpdateUserInput{UserID: req.GetUserId()}
	if req.GetEmail() != nil {
		v := req.GetEmail().GetValue()
		in.Email = &v
	}
	if req.GetName() != nil {
		v := req.GetName().GetValue()
		in.Name = &v
	}
	if req.Role != nil {
		r := domain.Role(req.GetRole().String())
		in.Role = &r
	}
	user, err := s.updateUser.Execute(ctx, in)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &authv1.UpdateUserResponse{User: toProtoUser(user)}, nil
}
```

Add `listSessions *usecase.ListSessions` and `updateUser *usecase.UpdateUser` fields to the `Server` struct and thread them through its constructor (matching how every other usecase field is wired), and add the two new usecases' construction to wherever `Server` is instantiated (likely `cmd/server/main.go`).

Adjust the `Role` conversion above (`domain.Role(req.GetRole().String())`) to whatever this codebase's actual proto-`Role`-to-`domain.Role` mapping helper already is — reuse the existing helper (likely named something like `fromProtoRole`) rather than duplicating role-string logic if one exists in this file.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/auth-service/...
go test ./services/auth-service/internal/adapter/grpc/... -run "TestServer_ListSessions|TestServer_UpdateUser" -v
```

Expected: build succeeds; a `ListSessions` call with a fake usecase returning sessions maps each `SessionWithUser` correctly; an `UpdateUser` call with only `email` set in the request produces a `usecase.UpdateUserInput` with `Name`/`Role` nil.
