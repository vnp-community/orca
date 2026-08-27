// Package grpc implements the generated aiproviderv1.AiProviderServiceServer
// interface by translating wire messages to/from usecase calls — no
// business logic here, per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md's
// inbound-adapter contract. No message this layer builds or reads ever
// carries a secret field — see internal/domain's package doc.
package grpc

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
	"github.com/stablyai/orca-go/services/ai-provider-service/internal/usecase"

	aiproviderv1 "github.com/stablyai/orca-go/proto/gen/go/orca/aiprovider/v1"
)

// Server implements aiproviderv1.UnimplementedAiProviderServiceServer.
type Server struct {
	aiproviderv1.UnimplementedAiProviderServiceServer

	createAccount   *usecase.CreateAccount
	resolveProvider *usecase.ResolveProvider
	rotateKey       *usecase.RotateKey
	getUsageToday   *usecase.GetUsageToday
	listAccounts    *usecase.ListAccounts
	updateAccount   *usecase.UpdateAccount
	deleteAccount   *usecase.DeleteAccount
	writeCredential *usecase.WriteCredential
	testConnection  *usecase.TestConnection
}

func New(
	create *usecase.CreateAccount,
	resolve *usecase.ResolveProvider,
	rotate *usecase.RotateKey,
	usage *usecase.GetUsageToday,
	list *usecase.ListAccounts,
	update *usecase.UpdateAccount,
	del *usecase.DeleteAccount,
	writeCredential *usecase.WriteCredential,
	testConnection *usecase.TestConnection,
) *Server {
	return &Server{
		createAccount:   create,
		resolveProvider: resolve,
		rotateKey:       rotate,
		getUsageToday:   usage,
		listAccounts:    list,
		updateAccount:   update,
		deleteAccount:   del,
		writeCredential: writeCredential,
		testConnection:  testConnection,
	}
}

func (s *Server) CreateAccount(ctx context.Context, req *aiproviderv1.CreateAccountRequest) (*aiproviderv1.CreateAccountResponse, error) {
	account, err := s.createAccount.Execute(ctx, usecase.CreateAccountInput{
		TenantID:     req.GetTenantId(),
		ProviderType: toDomainProviderType(req.GetType()),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &aiproviderv1.CreateAccountResponse{Account: toProtoAccount(account)}, nil
}

func (s *Server) ResolveProvider(ctx context.Context, req *aiproviderv1.ResolveProviderRequest) (*aiproviderv1.ResolveProviderResponse, error) {
	account, err := s.resolveProvider.Resolve(ctx, usecase.ResolveProviderInput{
		UserID:    req.GetUserId(),
		ProjectID: req.GetProjectId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(mapDomainError(err))
	}
	return &aiproviderv1.ResolveProviderResponse{Account: toProtoAccount(account)}, nil
}

func (s *Server) RotateKey(ctx context.Context, req *aiproviderv1.RotateKeyRequest) (*aiproviderv1.RotateKeyResponse, error) {
	account, err := s.rotateKey.Execute(ctx, usecase.RotateKeyInput{AccountID: req.GetAccountId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &aiproviderv1.RotateKeyResponse{Account: toProtoAccount(account)}, nil
}

func (s *Server) GetUsageToday(ctx context.Context, req *aiproviderv1.GetUsageTodayRequest) (*aiproviderv1.GetUsageTodayResponse, error) {
	state, err := s.getUsageToday.Execute(ctx, usecase.GetUsageTodayInput{AccountID: req.GetAccountId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &aiproviderv1.GetUsageTodayResponse{CostUsd: state.CostUSD, RequestCount: state.RequestCount}, nil
}

func (s *Server) ListAccounts(ctx context.Context, req *aiproviderv1.ListAccountsRequest) (*aiproviderv1.ListAccountsResponse, error) {
	accounts, err := s.listAccounts.Execute(ctx, usecase.ListAccountsInput{DevServerID: req.GetDevServerId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*aiproviderv1.ProviderAccount, 0, len(accounts))
	for _, a := range accounts {
		out = append(out, toProtoAccount(a))
	}
	return &aiproviderv1.ListAccountsResponse{Accounts: out}, nil
}

func (s *Server) UpdateAccount(ctx context.Context, req *aiproviderv1.UpdateAccountRequest) (*aiproviderv1.UpdateAccountResponse, error) {
	account, err := s.updateAccount.Execute(ctx, usecase.UpdateFields{
		AccountID: req.GetAccountId(),
		Label:     req.GetLabel(),
		ModelHint: req.GetModelHint(),
		BaseURL:   req.GetBaseUrl(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &aiproviderv1.UpdateAccountResponse{Account: toProtoAccount(account)}, nil
}

func (s *Server) DeleteAccount(ctx context.Context, req *aiproviderv1.DeleteAccountRequest) (*emptypb.Empty, error) {
	if err := s.deleteAccount.Execute(ctx, req.GetAccountId()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) WriteCredential(ctx context.Context, req *aiproviderv1.WriteCredentialRequest) (*aiproviderv1.WriteCredentialResponse, error) {
	account, err := s.writeCredential.Execute(ctx, usecase.WriteCredentialForAccountInput{
		AccountID:     req.GetAccountId(),
		EncryptedBlob: req.GetEncryptedBlob(),
		IV:            req.GetIv(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &aiproviderv1.WriteCredentialResponse{Account: toProtoAccount(account)}, nil
}

func (s *Server) TestConnection(ctx context.Context, req *aiproviderv1.TestConnectionRequest) (*aiproviderv1.TestConnectionResponse, error) {
	result, err := s.testConnection.Execute(ctx, usecase.TestConnectionInput{AccountID: req.GetAccountId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &aiproviderv1.TestConnectionResponse{Success: result.Success, Message: result.Message}, nil
}

// mapDomainError wraps a bare *domain.ErrNoProviderAvailable (which isn't an
// *apperrors.AppError) into one apperrors.ToGRPCStatus can map, so a failed
// cascade resolves to codes.NotFound rather than falling through to the
// generic codes.Internal default.
func mapDomainError(err error) error {
	var notAvailable *domain.ErrNoProviderAvailable
	if ok := asNoProviderAvailable(err, &notAvailable); ok {
		return apperrors.New(apperrors.KindNotFound, "AIPROVIDER_NO_ACCOUNT_AVAILABLE", notAvailable.Error(), notAvailable)
	}
	return err
}

func asNoProviderAvailable(err error, target **domain.ErrNoProviderAvailable) bool {
	if e, ok := err.(*domain.ErrNoProviderAvailable); ok {
		*target = e
		return true
	}
	return false
}

func toDomainProviderType(t aiproviderv1.ProviderType) domain.ProviderType {
	switch t {
	case aiproviderv1.ProviderType_PROVIDER_TYPE_ANTHROPIC:
		return domain.ProviderTypeAnthropic
	case aiproviderv1.ProviderType_PROVIDER_TYPE_OPENAI:
		return domain.ProviderTypeOpenAI
	case aiproviderv1.ProviderType_PROVIDER_TYPE_GOOGLE:
		return domain.ProviderTypeGoogle
	case aiproviderv1.ProviderType_PROVIDER_TYPE_AZURE:
		return domain.ProviderTypeAzure
	case aiproviderv1.ProviderType_PROVIDER_TYPE_AWS_BEDROCK:
		return domain.ProviderTypeAWSBedrock
	case aiproviderv1.ProviderType_PROVIDER_TYPE_OLLAMA:
		return domain.ProviderTypeOllama
	case aiproviderv1.ProviderType_PROVIDER_TYPE_VLLM:
		return domain.ProviderTypeVLLM
	default:
		return ""
	}
}

func toProtoProviderType(t domain.ProviderType) aiproviderv1.ProviderType {
	switch t {
	case domain.ProviderTypeAnthropic:
		return aiproviderv1.ProviderType_PROVIDER_TYPE_ANTHROPIC
	case domain.ProviderTypeOpenAI:
		return aiproviderv1.ProviderType_PROVIDER_TYPE_OPENAI
	case domain.ProviderTypeGoogle:
		return aiproviderv1.ProviderType_PROVIDER_TYPE_GOOGLE
	case domain.ProviderTypeAzure:
		return aiproviderv1.ProviderType_PROVIDER_TYPE_AZURE
	case domain.ProviderTypeAWSBedrock:
		return aiproviderv1.ProviderType_PROVIDER_TYPE_AWS_BEDROCK
	case domain.ProviderTypeOllama:
		return aiproviderv1.ProviderType_PROVIDER_TYPE_OLLAMA
	case domain.ProviderTypeVLLM:
		return aiproviderv1.ProviderType_PROVIDER_TYPE_VLLM
	default:
		return aiproviderv1.ProviderType_PROVIDER_TYPE_UNSPECIFIED
	}
}

// toProtoAccount maps domain.ProviderAccount onto the wire ProviderAccount
// message — id, tenant_id, type, status, credential_ref ONLY. There is no
// field on the wire message this could populate with a secret even by
// mistake; the proto simply has none.
func toProtoAccount(a domain.ProviderAccount) *aiproviderv1.ProviderAccount {
	return &aiproviderv1.ProviderAccount{
		Id:            a.ID,
		TenantId:      a.TenantID,
		Type:          toProtoProviderType(a.ProviderType),
		Status:        string(a.Status),
		CredentialRef: a.CredentialRef,
		DevServerId:   a.DevServerID,
	}
}
