// Package grpc implements the generated authv1.AuthServiceServer interface
// by translating wire messages to/from usecase calls — no business logic
// here, per specs/backend-go/architecture/03-clean-architecture-guidelines.md's
// inbound-adapter contract.
package grpc

import (
	"context"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
	"github.com/stablyai/orca-go/services/auth-service/internal/usecase"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

// Server implements authv1.UnimplementedAuthServiceServer.
type Server struct {
	authv1.UnimplementedAuthServiceServer

	login             *usecase.Login
	logout            *usecase.Logout
	validateSession   *usecase.ValidateSession
	createUser        *usecase.CreateUser
	listUsers         *usecase.ListUsers
	updateUserRole    *usecase.UpdateUserRole
	revokeSession     *usecase.RevokeSession
	queryAuditLog     *usecase.QueryAuditLog
	issueServiceToken *usecase.IssueServiceToken
	getJWKS           *usecase.GetJWKS
}

func New(
	login *usecase.Login,
	logout *usecase.Logout,
	validateSession *usecase.ValidateSession,
	createUser *usecase.CreateUser,
	listUsers *usecase.ListUsers,
	updateUserRole *usecase.UpdateUserRole,
	revokeSession *usecase.RevokeSession,
	queryAuditLog *usecase.QueryAuditLog,
	issueServiceToken *usecase.IssueServiceToken,
	getJWKS *usecase.GetJWKS,
) *Server {
	return &Server{
		login:             login,
		logout:            logout,
		validateSession:   validateSession,
		createUser:        createUser,
		listUsers:         listUsers,
		updateUserRole:    updateUserRole,
		revokeSession:     revokeSession,
		queryAuditLog:     queryAuditLog,
		issueServiceToken: issueServiceToken,
		getJWKS:           getJWKS,
	}
}

func (s *Server) Login(ctx context.Context, req *authv1.LoginRequest) (*authv1.LoginResponse, error) {
	out, err := s.login.Execute(ctx, usecase.LoginInput{Email: req.GetEmail(), Password: req.GetPassword()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &authv1.LoginResponse{SessionToken: out.SessionToken, User: toProtoUser(out.User)}, nil
}

func (s *Server) Logout(ctx context.Context, req *authv1.LogoutRequest) (*authv1.LogoutResponse, error) {
	if err := s.logout.Execute(ctx, req.GetSessionToken()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &authv1.LogoutResponse{}, nil
}

func (s *Server) ValidateSession(ctx context.Context, req *authv1.ValidateSessionRequest) (*authv1.ValidateSessionResponse, error) {
	out, err := s.validateSession.Execute(ctx, req.GetSessionToken())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	resp := &authv1.ValidateSessionResponse{Valid: out.Valid}
	if out.Valid {
		resp.User = toProtoUser(out.User)
	}
	return resp, nil
}

// IssueServiceToken mints a real RS256 JWT, signed through Vault Transit
// (internal/adapter/vault.TokenSigner) — the private key never materializes
// in this service's process memory. See this service's README "Known gaps"
// for what's still not covered (caller-authorization, the fuller
// IssueToken/RefreshToken/RevokeToken surface).
func (s *Server) IssueServiceToken(ctx context.Context, req *authv1.IssueServiceTokenRequest) (*authv1.IssueServiceTokenResponse, error) {
	out, err := s.issueServiceToken.Execute(ctx, usecase.IssueServiceTokenInput{
		UserID:   req.GetUserId(),
		Audience: req.GetAudience(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &authv1.IssueServiceTokenResponse{
		Jwt:       out.JWT,
		ExpiresAt: timestamppb.New(out.ExpiresAt),
	}, nil
}

// GetJWKS is public/unauthenticated by convention (see
// proto/orca/auth/v1/auth.proto's doc comment on the RPC) — no actor
// resolution here, unlike every admin-console handler above.
func (s *Server) GetJWKS(ctx context.Context, req *authv1.GetJWKSRequest) (*authv1.GetJWKSResponse, error) {
	out, err := s.getJWKS.Execute(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &authv1.GetJWKSResponse{JwksJson: out.JWKSJSON}, nil
}

func (s *Server) CreateUser(ctx context.Context, req *authv1.CreateUserRequest) (*authv1.CreateUserResponse, error) {
	user, err := s.createUser.Execute(ctx, usecase.CreateUserInput{
		Email:    req.GetEmail(),
		Name:     req.GetName(),
		TenantID: req.GetTenantId(),
		Role:     toDomainRole(req.GetRole()),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &authv1.CreateUserResponse{User: toProtoUser(user)}, nil
}

func (s *Server) ListUsers(ctx context.Context, req *authv1.ListUsersRequest) (*authv1.ListUsersResponse, error) {
	out, err := s.listUsers.Execute(ctx, usecase.ListUsersInput{
		TenantID:  req.GetTenantId(),
		PageToken: req.GetPageToken(),
		PageSize:  req.GetPageSize(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	users := make([]*authv1.User, 0, len(out.Users))
	for _, u := range out.Users {
		users = append(users, toProtoUser(u))
	}
	return &authv1.ListUsersResponse{Users: users, NextPageToken: out.NextPageToken}, nil
}

func (s *Server) UpdateUserRole(ctx context.Context, req *authv1.UpdateUserRoleRequest) (*authv1.UpdateUserRoleResponse, error) {
	user, err := s.updateUserRole.Execute(ctx, req.GetUserId(), toDomainRole(req.GetRole()))
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &authv1.UpdateUserRoleResponse{User: toProtoUser(user)}, nil
}

func (s *Server) RevokeSession(ctx context.Context, req *authv1.RevokeSessionRequest) (*authv1.RevokeSessionResponse, error) {
	if err := s.revokeSession.Execute(ctx, req.GetSessionToken()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &authv1.RevokeSessionResponse{}, nil
}

func (s *Server) QueryAuditLog(ctx context.Context, req *authv1.QueryAuditLogRequest) (*authv1.QueryAuditLogResponse, error) {
	out, err := s.queryAuditLog.Execute(ctx, usecase.QueryAuditLogInput{
		TenantID:  req.GetTenantId(),
		Since:     toTime(req.GetSince()),
		PageToken: req.GetPageToken(),
		PageSize:  req.GetPageSize(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	entries := make([]*authv1.AuditEntry, 0, len(out.Entries))
	for _, e := range out.Entries {
		entries = append(entries, toProtoAuditEntry(e))
	}
	return &authv1.QueryAuditLogResponse{Entries: entries, NextPageToken: out.NextPageToken}, nil
}

func toDomainRole(r authv1.Role) domain.Role {
	switch r {
	case authv1.Role_ROLE_USER:
		return domain.RoleUser
	case authv1.Role_ROLE_ADMIN:
		return domain.RoleAdmin
	default:
		return ""
	}
}

func toProtoRole(r domain.Role) authv1.Role {
	switch r {
	case domain.RoleUser:
		return authv1.Role_ROLE_USER
	case domain.RoleAdmin:
		return authv1.Role_ROLE_ADMIN
	default:
		return authv1.Role_ROLE_UNSPECIFIED
	}
}

func toTime(ts *timestamppb.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return ts.AsTime()
}

func toProtoUser(u domain.User) *authv1.User {
	out := &authv1.User{
		Id:       u.ID,
		TenantId: u.TenantID,
		Email:    u.Email,
		Name:     u.Name,
		Role:     toProtoRole(u.Role),
		IsActive: u.IsActive,
	}
	if !u.CreatedAt.IsZero() {
		out.CreatedAt = timestamppb.New(u.CreatedAt)
	}
	return out
}

func toProtoAuditEntry(e domain.AuditEntry) *authv1.AuditEntry {
	out := &authv1.AuditEntry{
		Id:       e.ID,
		TenantId: e.TenantID,
		ActorId:  e.ActorID,
		Action:   e.Action,
		Target:   e.Target,
	}
	if !e.OccurredAt.IsZero() {
		out.OccurredAt = timestamppb.New(e.OccurredAt)
	}
	return out
}
