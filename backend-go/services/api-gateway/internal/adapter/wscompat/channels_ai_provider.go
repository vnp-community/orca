// Package wscompat — aiProvider.* channels. See SOL-005
// (specs/backend-go/bugs/missing-v1/solutions/SOL-005-aiprovider-channels.md)
// for the full design this file wires up. ai-provider-service's usecases
// require tenant via ctx (tenant.RequireTenantID), same AttachIdentity
// requirement as devServer.*/fleet.* channels.
package wscompat

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	aiproviderv1 "github.com/stablyai/orca-go/proto/gen/go/orca/aiprovider/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

// registerAiProviderChannels wires the 7 aiProvider.* channels documented in
// specs/frontend/api/rpc-catalog.md's aiProvider.* section to
// ai-provider-service's AiProviderServiceClient.
func registerAiProviderChannels(r *Registry, client aiproviderv1.AiProviderServiceClient) {
	r.Register("aiProvider.create", handleAiProviderCreate(client))
	r.Register("aiProvider.list", handleAiProviderList(client))
	r.Register("aiProvider.update", handleAiProviderUpdate(client))
	r.Register("aiProvider.delete", handleAiProviderDelete(client))
	r.Register("aiProvider.writeCredential", handleAiProviderWriteCredential(client))
	r.Register("aiProvider.testConnection", handleAiProviderTestConnection(client))
	r.Register("aiProvider.resolve", handleAiProviderResolve(client))
}

// attachAiProviderIdentity is shared by every handler below.
func attachAiProviderIdentity(ctx context.Context, id Identity) (context.Context, context.CancelFunc) {
	ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
	return context.WithTimeout(ctx, rpcTimeout)
}

func handleAiProviderCreate(client aiproviderv1.AiProviderServiceClient) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type createArgs struct {
			Type          string   `json:"type"`
			DevServerID   string   `json:"devServerId"`
			Label         string   `json:"label"`
			ModelHint     string   `json:"modelHint"`
			BaseURL       string   `json:"baseUrl"`
			QuotaLimitDay int32    `json:"quotaLimitDay"`
			Models        []string `json:"models"`
			IsDefault     bool     `json:"isDefault"`
		}
		in, err := decodeArg[createArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := attachAiProviderIdentity(ctx, id)
		defer cancel()
		resp, err := client.CreateAccount(rpcCtx, &aiproviderv1.CreateAccountRequest{
			TenantId:      id.TenantID,
			Type:          aiproviderv1.ProviderType(aiproviderv1.ProviderType_value[in.Type]),
			DevServerId:   in.DevServerID,
			Label:         in.Label,
			ModelHint:     in.ModelHint,
			BaseUrl:       in.BaseURL,
			QuotaLimitDay: in.QuotaLimitDay,
			Models:        in.Models,
			IsDefault:     in.IsDefault,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetAccount(), nil
	}
}

func handleAiProviderList(client aiproviderv1.AiProviderServiceClient) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type listArgs struct {
			DevServerID string `json:"devServerId"`
		}
		in, _ := decodeArg[listArgs](args, 0) // devServerId is an optional filter
		rpcCtx, cancel := attachAiProviderIdentity(ctx, id)
		defer cancel()
		resp, err := client.ListAccounts(rpcCtx, &aiproviderv1.ListAccountsRequest{DevServerId: in.DevServerID})
		if err != nil {
			return nil, err
		}
		return resp.GetAccounts(), nil
	}
}

func handleAiProviderUpdate(client aiproviderv1.AiProviderServiceClient) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type updateArgs struct {
			AccountID string `json:"accountId"`
			Label     string `json:"label"`
			ModelHint string `json:"modelHint"`
			BaseURL   string `json:"baseUrl"`
		}
		in, err := decodeArg[updateArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := attachAiProviderIdentity(ctx, id)
		defer cancel()
		resp, err := client.UpdateAccount(rpcCtx, &aiproviderv1.UpdateAccountRequest{
			AccountId: in.AccountID, Label: in.Label, ModelHint: in.ModelHint, BaseUrl: in.BaseURL,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetAccount(), nil
	}
}

func handleAiProviderDelete(client aiproviderv1.AiProviderServiceClient) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type deleteArgs struct {
			AccountID string `json:"accountId"`
		}
		in, err := decodeArg[deleteArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := attachAiProviderIdentity(ctx, id)
		defer cancel()
		if _, err := client.DeleteAccount(rpcCtx, &aiproviderv1.DeleteAccountRequest{AccountId: in.AccountID}); err != nil {
			return nil, err
		}
		// matches annotation.delete's response shape (channels.go)
		return map[string]bool{"ok": true}, nil
	}
}

func handleAiProviderWriteCredential(client aiproviderv1.AiProviderServiceClient) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type writeCredentialArgs struct {
			AccountID     string `json:"accountId"`
			EncryptedBlob string `json:"encryptedBlob"` // base64 in the JSON envelope
			IV            string `json:"iv"`            // base64 in the JSON envelope
		}
		in, err := decodeArg[writeCredentialArgs](args, 0)
		if err != nil {
			return nil, err
		}
		blob, err := base64.StdEncoding.DecodeString(in.EncryptedBlob)
		if err != nil {
			return nil, fmt.Errorf("decoding encryptedBlob: %w", err)
		}
		iv, err := base64.StdEncoding.DecodeString(in.IV)
		if err != nil {
			return nil, fmt.Errorf("decoding iv: %w", err)
		}
		rpcCtx, cancel := attachAiProviderIdentity(ctx, id)
		defer cancel()
		resp, err := client.WriteCredential(rpcCtx, &aiproviderv1.WriteCredentialRequest{
			AccountId: in.AccountID, EncryptedBlob: blob, Iv: iv,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetAccount(), nil
	}
}

func handleAiProviderTestConnection(client aiproviderv1.AiProviderServiceClient) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type testConnectionArgs struct {
			AccountID string `json:"accountId"`
			TraceID   string `json:"traceId"`
		}
		in, err := decodeArg[testConnectionArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := attachAiProviderIdentity(ctx, id)
		defer cancel()
		resp, err := client.TestConnection(rpcCtx, &aiproviderv1.TestConnectionRequest{
			AccountId: in.AccountID, TraceId: in.TraceID,
		})
		if err != nil {
			return nil, err
		}
		return map[string]any{"success": resp.GetSuccess(), "message": resp.GetMessage()}, nil
	}
}

func handleAiProviderResolve(client aiproviderv1.AiProviderServiceClient) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type resolveArgs struct {
			UserID      string `json:"userId"`
			ProjectID   string `json:"projectId"`
			DevServerID string `json:"devServerId"`
			ModelHint   string `json:"modelHint"`
			AccountID   string `json:"accountId"`
			ScopedRef   string `json:"scopedRef"`
		}
		in, err := decodeArg[resolveArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := attachAiProviderIdentity(ctx, id)
		defer cancel()
		req := &aiproviderv1.ResolveProviderRequest{
			TenantId:    id.TenantID,
			UserId:      in.UserID,
			ProjectId:   in.ProjectID,
			DevServerId: in.DevServerID,
			AccountId:   in.AccountID,
			ScopedRef:   in.ScopedRef,
		}
		// ModelHint is a proto3-optional *string — only set it when non-empty,
		// same nil-vs-empty convention as channels_scm.go's *string fields.
		if in.ModelHint != "" {
			req.ModelHint = &in.ModelHint
		}
		resp, err := client.ResolveProvider(rpcCtx, req)
		if err != nil {
			return nil, err
		}
		return resp.GetAccount(), nil
	}
}
