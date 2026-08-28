# TASK-AUTH-01-03: Thread `req.GetIp()`/`req.GetUserAgent()` into `LoginInput`

**From Solution:** SOL-AUTH-01
**Priority:** P0
**Service:** `auth-service` (grpc adapter)
**File:** `backend-go/services/auth-service/internal/adapter/grpc/server.go`
**Depends on:** TASK-AUTH-01-01, TASK-AUTH-01-02
**Status:** `[x]` DONE — `Server.Login` threads `req.GetIp()`/`req.GetUserAgent()` into `LoginInput`; new `server_test.go` (`TestServer_Login`, `TestServer_Login_WrongPasswordWritesFailureAudit`) passes.

---

## Context

`Server.Login` currently only forwards `Email`/`Password` into `usecase.LoginInput`. Now that the proto has `ip`/`user_agent` (TASK-AUTH-01-01) and `LoginInput` has matching fields (TASK-AUTH-01-02), wire them through.

## Changes to make

In `backend-go/services/auth-service/internal/adapter/grpc/server.go`, change:

```go
func (s *Server) Login(ctx context.Context, req *authv1.LoginRequest) (*authv1.LoginResponse, error) {
	out, err := s.login.Execute(ctx, usecase.LoginInput{Email: req.GetEmail(), Password: req.GetPassword()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &authv1.LoginResponse{SessionToken: out.SessionToken, User: toProtoUser(out.User)}, nil
}
```

to:

```go
func (s *Server) Login(ctx context.Context, req *authv1.LoginRequest) (*authv1.LoginResponse, error) {
	out, err := s.login.Execute(ctx, usecase.LoginInput{
		Email:     req.GetEmail(),
		Password:  req.GetPassword(),
		IP:        req.GetIp(),
		UserAgent: req.GetUserAgent(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &authv1.LoginResponse{SessionToken: out.SessionToken, User: toProtoUser(out.User)}, nil
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/auth-service/...
go test ./services/auth-service/internal/adapter/grpc/... -run TestServer_Login -v
```

Expected: build succeeds; existing `Login` RPC tests still pass, add a case asserting a fake `Login` usecase receives non-empty `IP`/`UserAgent` when the request sets them.
