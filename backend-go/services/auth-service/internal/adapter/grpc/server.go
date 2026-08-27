// Package grpc implements the generated authv1.AuthServiceServer interface
// by translating wire messages to/from usecase calls — no business logic
// here, per specs/backend-go/architecture/03-clean-architecture-guidelines.md's
// inbound-adapter contract.
package grpc

import (
	"context"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"
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

	deactivateUser                *usecase.DeactivateUser
	reactivateUser                *usecase.ReactivateUser
	listSessionsForUser           *usecase.ListSessionsForUser
	forceRevokeAllSessionsForUser *usecase.ForceRevokeAllSessionsForUser
	createAccessPolicy            *usecase.CreateAccessPolicy
	getAccessPolicy               *usecase.GetAccessPolicy
	listAccessPolicies            *usecase.ListAccessPolicies
	updateAccessPolicy            *usecase.UpdateAccessPolicy
	deleteAccessPolicy            *usecase.DeleteAccessPolicy
	getAdminStats                 *usecase.GetAdminStats

	initiateDevicePairing     *usecase.InitiateDevicePairing
	completeDevicePairing     *usecase.CompleteDevicePairing
	listPairedDevices         *usecase.ListPairedDevices
	unpairDevice              *usecase.UnpairDevice
	resolveDeviceSharedSecret *usecase.ResolveDeviceSharedSecret
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
	deactivateUser *usecase.DeactivateUser,
	reactivateUser *usecase.ReactivateUser,
	listSessionsForUser *usecase.ListSessionsForUser,
	forceRevokeAllSessionsForUser *usecase.ForceRevokeAllSessionsForUser,
	createAccessPolicy *usecase.CreateAccessPolicy,
	getAccessPolicy *usecase.GetAccessPolicy,
	listAccessPolicies *usecase.ListAccessPolicies,
	updateAccessPolicy *usecase.UpdateAccessPolicy,
	deleteAccessPolicy *usecase.DeleteAccessPolicy,
	getAdminStats *usecase.GetAdminStats,
	initiateDevicePairing *usecase.InitiateDevicePairing,
	completeDevicePairing *usecase.CompleteDevicePairing,
	listPairedDevices *usecase.ListPairedDevices,
	unpairDevice *usecase.UnpairDevice,
	resolveDeviceSharedSecret *usecase.ResolveDeviceSharedSecret,
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

		deactivateUser:                deactivateUser,
		reactivateUser:                reactivateUser,
		listSessionsForUser:           listSessionsForUser,
		forceRevokeAllSessionsForUser: forceRevokeAllSessionsForUser,
		createAccessPolicy:            createAccessPolicy,
		getAccessPolicy:               getAccessPolicy,
		listAccessPolicies:            listAccessPolicies,
		updateAccessPolicy:            updateAccessPolicy,
		deleteAccessPolicy:            deleteAccessPolicy,
		getAdminStats:                 getAdminStats,

		initiateDevicePairing:     initiateDevicePairing,
		completeDevicePairing:     completeDevicePairing,
		listPairedDevices:         listPairedDevices,
		unpairDevice:              unpairDevice,
		resolveDeviceSharedSecret: resolveDeviceSharedSecret,
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

func (s *Server) DeactivateUser(ctx context.Context, req *authv1.DeactivateUserRequest) (*authv1.DeactivateUserResponse, error) {
	user, err := s.deactivateUser.Execute(ctx, req.GetUserId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &authv1.DeactivateUserResponse{User: toProtoUser(user)}, nil
}

func (s *Server) ReactivateUser(ctx context.Context, req *authv1.ReactivateUserRequest) (*authv1.ReactivateUserResponse, error) {
	user, err := s.reactivateUser.Execute(ctx, req.GetUserId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &authv1.ReactivateUserResponse{User: toProtoUser(user)}, nil
}

func (s *Server) ListSessionsForUser(ctx context.Context, req *authv1.ListSessionsForUserRequest) (*authv1.ListSessionsForUserResponse, error) {
	sessions, err := s.listSessionsForUser.Execute(ctx, req.GetUserId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*authv1.Session, 0, len(sessions))
	for _, sess := range sessions {
		out = append(out, toProtoSession(sess))
	}
	return &authv1.ListSessionsForUserResponse{Sessions: out}, nil
}

func (s *Server) ForceRevokeAllSessionsForUser(ctx context.Context, req *authv1.ForceRevokeAllSessionsForUserRequest) (*authv1.ForceRevokeAllSessionsForUserResponse, error) {
	revoked, err := s.forceRevokeAllSessionsForUser.Execute(ctx, req.GetUserId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &authv1.ForceRevokeAllSessionsForUserResponse{RevokedCount: revoked}, nil
}

func (s *Server) CreateAccessPolicy(ctx context.Context, req *authv1.CreateAccessPolicyRequest) (*authv1.AccessPolicy, error) {
	policy, err := s.createAccessPolicy.Execute(ctx, usecase.CreateAccessPolicyInput{
		Name:         req.GetName(),
		Kind:         req.GetKind(),
		DocumentJSON: req.GetDocumentJson(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return toProtoAccessPolicy(policy), nil
}

func (s *Server) GetAccessPolicy(ctx context.Context, req *authv1.GetAccessPolicyRequest) (*authv1.AccessPolicy, error) {
	policy, err := s.getAccessPolicy.Execute(ctx, req.GetId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return toProtoAccessPolicy(policy), nil
}

func (s *Server) ListAccessPolicies(ctx context.Context, req *authv1.ListAccessPoliciesRequest) (*authv1.ListAccessPoliciesResponse, error) {
	out, err := s.listAccessPolicies.Execute(ctx, usecase.ListAccessPoliciesInput{
		PageToken: req.GetPageToken(),
		PageSize:  req.GetPageSize(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	policies := make([]*authv1.AccessPolicy, 0, len(out.Policies))
	for _, p := range out.Policies {
		policies = append(policies, toProtoAccessPolicy(p))
	}
	return &authv1.ListAccessPoliciesResponse{Policies: policies, NextPageToken: out.NextPageToken}, nil
}

func (s *Server) UpdateAccessPolicy(ctx context.Context, req *authv1.UpdateAccessPolicyRequest) (*authv1.AccessPolicy, error) {
	policy, err := s.updateAccessPolicy.Execute(ctx, req.GetId(), req.GetDocumentJson(), req.GetExpectedVersion())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return toProtoAccessPolicy(policy), nil
}

func (s *Server) DeleteAccessPolicy(ctx context.Context, req *authv1.DeleteAccessPolicyRequest) (*emptypb.Empty, error) {
	if err := s.deleteAccessPolicy.Execute(ctx, req.GetId()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) GetAdminStats(ctx context.Context, req *authv1.GetAdminStatsRequest) (*authv1.GetAdminStatsResponse, error) {
	stats, err := s.getAdminStats.Execute(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &authv1.GetAdminStatsResponse{
		TotalUsers:     stats.TotalUsers,
		ActiveSessions: stats.ActiveSessions,
		TotalPolicies:  stats.TotalPolicies,
	}, nil
}

func (s *Server) InitiateDevicePairing(ctx context.Context, req *authv1.InitiateDevicePairingRequest) (*authv1.InitiateDevicePairingResponse, error) {
	result, err := s.initiateDevicePairing.Execute(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &authv1.InitiateDevicePairingResponse{
		PairingToken:     result.PairingToken,
		DesktopPublicKey: result.DesktopPublicKey,
		ServerAddress:    result.ServerAddress,
		ExpiresAtUnixMs:  result.ExpiresAt.UnixMilli(),
	}, nil
}

func (s *Server) CompleteDevicePairing(ctx context.Context, req *authv1.CompleteDevicePairingRequest) (*authv1.CompleteDevicePairingResponse, error) {
	result, err := s.completeDevicePairing.Execute(ctx, req.GetPairingToken(), req.GetMobilePublicKey(), req.GetDeviceLabel())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &authv1.CompleteDevicePairingResponse{
		DeviceId:                     result.DeviceID,
		DesktopPublicKeyConfirmation: result.DesktopPublicKeyConfirmation,
		AccessToken:                  result.AccessToken,
		RefreshToken:                 result.RefreshToken,
	}, nil
}

func (s *Server) ListPairedDevices(ctx context.Context, req *authv1.ListPairedDevicesRequest) (*authv1.ListPairedDevicesResponse, error) {
	devices, err := s.listPairedDevices.Execute(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*authv1.PairedDevice, 0, len(devices))
	for _, d := range devices {
		out = append(out, &authv1.PairedDevice{
			Id:               d.ID,
			DeviceLabel:      d.DeviceLabel,
			PairedAtUnixMs:   d.PairedAt.UnixMilli(),
			LastUsedAtUnixMs: d.LastUsedAt.UnixMilli(),
			Status:           string(d.Status),
		})
	}
	return &authv1.ListPairedDevicesResponse{Devices: out}, nil
}

func (s *Server) UnpairDevice(ctx context.Context, req *authv1.UnpairDeviceRequest) (*emptypb.Empty, error) {
	if err := s.unpairDevice.Execute(ctx, req.GetDeviceId()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) ResolveDeviceSharedSecret(ctx context.Context, req *authv1.ResolveDeviceSharedSecretRequest) (*authv1.ResolveDeviceSharedSecretResponse, error) {
	secret, err := s.resolveDeviceSharedSecret.Execute(ctx, req.GetDeviceId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &authv1.ResolveDeviceSharedSecretResponse{SharedSecret: secret}, nil
}

func toProtoSession(s domain.Session) *authv1.Session {
	out := &authv1.Session{
		Id:     s.TokenHash,
		UserId: s.UserID,
	}
	if !s.CreatedAt.IsZero() {
		out.CreatedAt = timestamppb.New(s.CreatedAt)
	}
	if !s.ExpiresAt.IsZero() {
		out.ExpiresAt = timestamppb.New(s.ExpiresAt)
	}
	return out
}

func toProtoAccessPolicy(p domain.AccessPolicy) *authv1.AccessPolicy {
	out := &authv1.AccessPolicy{
		Id:           p.ID,
		Name:         p.Name,
		Kind:         p.Kind,
		DocumentJson: p.DocumentJSON,
		Version:      p.Version,
		UpdatedBy:    p.UpdatedBy,
	}
	if !p.UpdatedAt.IsZero() {
		out.UpdatedAt = timestamppb.New(p.UpdatedAt)
	}
	return out
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
