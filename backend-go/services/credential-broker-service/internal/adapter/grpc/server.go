// Package grpc implements the generated credentialbrokerv1.CredentialBrokerServiceServer
// interface by translating wire messages to/from usecase calls — no
// business logic here, per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md's
// inbound-adapter contract.
package grpc

import (
	"context"

	"google.golang.org/grpc/metadata"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/credential-broker-service/internal/domain"
	"github.com/stablyai/orca-go/services/credential-broker-service/internal/usecase"

	credentialbrokerv1 "github.com/stablyai/orca-go/proto/gen/go/orca/credentialbroker/v1"
)

// requestingServiceMetadataKey carries the calling service's identity.
//
// KNOWN GAP (see this service's README): in production this is resolved
// from the caller's mTLS SPIFFE identity by the service mesh / an auth
// interceptor (credential-broker-service.md §4: "RequestingIdentity —
// resolved JWT/mTLS subject, not client-asserted"), the same way
// common/grpcmw's TenantExtractionInterceptor resolves tenant/user identity
// today. That interceptor doesn't yet extract a service identity, and this
// service must not modify common/grpcmw to add it, so this scaffold reads a
// plain gRPC metadata header instead — trusted only as far as this
// scaffold's local-dev/internal-mesh trust boundary goes, NOT a substitute
// for real mTLS identity extraction before production use.
const requestingServiceMetadataKey = "x-orca-service-id"

// Server implements credentialbrokerv1.UnimplementedCredentialBrokerServiceServer.
type Server struct {
	credentialbrokerv1.UnimplementedCredentialBrokerServiceServer

	writeCredential              *usecase.WriteCredential
	resolveCredential            *usecase.ResolveCredential
	rotateCredential             *usecase.RotateCredential
	revokeCredential             *usecase.RevokeCredential
	getCredentialMetadata        *usecase.GetCredentialMetadata
	resolveCredentialByOwner     *usecase.ResolveCredentialByOwner
	revokeCredentialByOwner      *usecase.RevokeCredentialByOwner
	signVapidPayload             *usecase.SignVapidPayload
	getCredentialMetadataByOwner *usecase.GetCredentialMetadataByOwner
	listCredentialsByCategory    *usecase.ListCredentialsByCategory
}

func New(
	write *usecase.WriteCredential,
	resolve *usecase.ResolveCredential,
	rotate *usecase.RotateCredential,
	revoke *usecase.RevokeCredential,
	getMetadata *usecase.GetCredentialMetadata,
	resolveByOwner *usecase.ResolveCredentialByOwner,
	revokeByOwner *usecase.RevokeCredentialByOwner,
	signVapid *usecase.SignVapidPayload,
	getMetadataByOwner *usecase.GetCredentialMetadataByOwner,
	listByCategory *usecase.ListCredentialsByCategory,
) *Server {
	return &Server{
		writeCredential:              write,
		resolveCredential:            resolve,
		rotateCredential:             rotate,
		revokeCredential:             revoke,
		getCredentialMetadata:        getMetadata,
		resolveCredentialByOwner:     resolveByOwner,
		revokeCredentialByOwner:      revokeByOwner,
		signVapidPayload:             signVapid,
		getCredentialMetadataByOwner: getMetadataByOwner,
		listCredentialsByCategory:    listByCategory,
	}
}

func (s *Server) WriteCredential(ctx context.Context, req *credentialbrokerv1.WriteCredentialRequest) (*credentialbrokerv1.WriteCredentialResponse, error) {
	created, err := s.writeCredential.Execute(ctx, usecase.WriteCredentialInput{
		TenantID:          req.GetTenantId(),
		OwnerID:           req.GetOwnerId(),
		Category:          toDomainCategory(req.GetCategory()),
		EncryptedEnvelope: req.GetEncryptedEnvelope(),
		ConfigJSON:        req.GetConfigJson(),
		RequestingService: requestingService(ctx),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &credentialbrokerv1.WriteCredentialResponse{Metadata: toProtoMetadata(created)}, nil
}

func (s *Server) ResolveCredential(ctx context.Context, req *credentialbrokerv1.ResolveCredentialRequest) (*credentialbrokerv1.ResolveCredentialResponse, error) {
	value, err := s.resolveCredential.Execute(ctx, usecase.ResolveCredentialInput{
		CredentialID:      req.GetCredentialId(),
		RequestingService: requestingService(ctx),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &credentialbrokerv1.ResolveCredentialResponse{Value: value}, nil
}

func (s *Server) RotateCredential(ctx context.Context, req *credentialbrokerv1.RotateCredentialRequest) (*credentialbrokerv1.RotateCredentialResponse, error) {
	rotated, err := s.rotateCredential.Execute(ctx, usecase.RotateCredentialInput{
		CredentialID:         req.GetCredentialId(),
		NewEncryptedEnvelope: req.GetNewEncryptedEnvelope(),
		RequestingService:    requestingService(ctx),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &credentialbrokerv1.RotateCredentialResponse{Metadata: toProtoMetadata(rotated)}, nil
}

func (s *Server) RevokeCredential(ctx context.Context, req *credentialbrokerv1.RevokeCredentialRequest) (*credentialbrokerv1.RevokeCredentialResponse, error) {
	_, err := s.revokeCredential.Execute(ctx, usecase.RevokeCredentialInput{
		CredentialID:      req.GetCredentialId(),
		RequestingService: requestingService(ctx),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &credentialbrokerv1.RevokeCredentialResponse{}, nil
}

func (s *Server) GetCredentialMetadata(ctx context.Context, req *credentialbrokerv1.GetCredentialMetadataRequest) (*credentialbrokerv1.GetCredentialMetadataResponse, error) {
	metadata, err := s.getCredentialMetadata.Execute(ctx, usecase.GetCredentialMetadataInput{
		CredentialID: req.GetCredentialId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &credentialbrokerv1.GetCredentialMetadataResponse{Metadata: toProtoMetadata(metadata)}, nil
}

func (s *Server) ResolveCredentialByOwner(ctx context.Context, req *credentialbrokerv1.ResolveCredentialByOwnerRequest) (*credentialbrokerv1.ResolveCredentialByOwnerResponse, error) {
	value, err := s.resolveCredentialByOwner.Execute(ctx, usecase.ResolveCredentialByOwnerInput{
		TenantID:          req.GetTenantId(),
		Category:          toDomainCategory(req.GetCategory()),
		OwnerID:           req.GetOwnerId(),
		RequestingService: requestingService(ctx),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &credentialbrokerv1.ResolveCredentialByOwnerResponse{Value: value}, nil
}

func (s *Server) RevokeCredentialByOwner(ctx context.Context, req *credentialbrokerv1.RevokeCredentialByOwnerRequest) (*credentialbrokerv1.RevokeCredentialByOwnerResponse, error) {
	_, err := s.revokeCredentialByOwner.Execute(ctx, usecase.RevokeCredentialByOwnerInput{
		TenantID:          req.GetTenantId(),
		Category:          toDomainCategory(req.GetCategory()),
		OwnerID:           req.GetOwnerId(),
		RequestingService: requestingService(ctx),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &credentialbrokerv1.RevokeCredentialByOwnerResponse{}, nil
}

// GetCredentialMetadataByOwner is a pure metadata read — see
// usecase.GetCredentialMetadataByOwner's doc comment. Found=false maps to a
// response with metadata left unset, not an error, per that field's
// `optional` proto declaration.
func (s *Server) GetCredentialMetadataByOwner(ctx context.Context, req *credentialbrokerv1.GetCredentialMetadataByOwnerRequest) (*credentialbrokerv1.GetCredentialMetadataByOwnerResponse, error) {
	result, err := s.getCredentialMetadataByOwner.Execute(ctx, usecase.GetCredentialMetadataByOwnerInput{
		TenantID: req.GetTenantId(),
		Category: toDomainCategory(req.GetCategory()),
		OwnerID:  req.GetOwnerId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	if !result.Found {
		return &credentialbrokerv1.GetCredentialMetadataByOwnerResponse{}, nil
	}
	return &credentialbrokerv1.GetCredentialMetadataByOwnerResponse{Metadata: toProtoMetadata(result.Metadata)}, nil
}

func (s *Server) ListCredentialsByCategory(ctx context.Context, req *credentialbrokerv1.ListCredentialsByCategoryRequest) (*credentialbrokerv1.ListCredentialsByCategoryResponse, error) {
	rows, err := s.listCredentialsByCategory.Execute(ctx, usecase.ListCredentialsByCategoryInput{
		TenantID: req.GetTenantId(),
		Category: toDomainCategory(req.GetCategory()),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	credentials := make([]*credentialbrokerv1.CredentialMetadata, 0, len(rows))
	for _, m := range rows {
		credentials = append(credentials, toProtoMetadata(m))
	}
	return &credentialbrokerv1.ListCredentialsByCategoryResponse{Credentials: credentials}, nil
}

func (s *Server) SignVapidPayload(ctx context.Context, req *credentialbrokerv1.SignVapidPayloadRequest) (*credentialbrokerv1.SignVapidPayloadResponse, error) {
	signature, err := s.signVapidPayload.Execute(ctx, usecase.SignVapidPayloadInput{
		TenantID: req.GetTenantId(),
		Payload:  req.GetPayload(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &credentialbrokerv1.SignVapidPayloadResponse{Signature: signature}, nil
}

func requestingService(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	v := md.Get(requestingServiceMetadataKey)
	if len(v) == 0 {
		return ""
	}
	return v[0]
}

func toDomainCategory(c credentialbrokerv1.CredentialCategory) domain.Category {
	switch c {
	case credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_SCM_OAUTH:
		return domain.CategoryScmOAuth
	case credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_ISSUE_TRACKER_OAUTH:
		return domain.CategoryIssueTrackerOAuth
	case credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_AI_PROVIDER_KEY:
		return domain.CategoryAiProviderKey
	case credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_SSH:
		return domain.CategorySsh
	case credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_SERVICE_SECRET:
		return domain.CategoryServiceSecret
	case credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_DEV_SERVER_AGENT_TOKEN:
		return domain.CategoryDevServerAgentToken
	default:
		return ""
	}
}

func toProtoCategory(c domain.Category) credentialbrokerv1.CredentialCategory {
	switch c {
	case domain.CategoryScmOAuth:
		return credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_SCM_OAUTH
	case domain.CategoryIssueTrackerOAuth:
		return credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_ISSUE_TRACKER_OAUTH
	case domain.CategoryAiProviderKey:
		return credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_AI_PROVIDER_KEY
	case domain.CategorySsh:
		return credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_SSH
	case domain.CategoryServiceSecret:
		return credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_SERVICE_SECRET
	case domain.CategoryDevServerAgentToken:
		return credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_DEV_SERVER_AGENT_TOKEN
	default:
		return credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_UNSPECIFIED
	}
}

func toProtoMetadata(m domain.CredentialMetadata) *credentialbrokerv1.CredentialMetadata {
	return &credentialbrokerv1.CredentialMetadata{
		Id:         m.ID,
		TenantId:   m.TenantID,
		OwnerId:    m.OwnerID,
		Category:   toProtoCategory(m.Category),
		Status:     string(m.Status),
		VaultPath:  m.VaultPath,
		ConfigJson: m.ConfigJSON,
	}
}
