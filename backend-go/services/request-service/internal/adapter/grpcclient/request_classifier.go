package grpcclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	aiproviderv1 "github.com/stablyai/orca-go/proto/gen/go/orca/aiprovider/v1"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const defaultClassifierTimeout = 60 * time.Second

type connectionResolver interface {
	ResolveForProject(ctx context.Context, projectID string) (AIConnection, error)
}

// RelayClassifier asks the AI through infra-fleet-service's Relay (ai.complete). It never
// trusts the reply: the proposal parser rejects anything outside the enums.
type RelayClassifier struct {
	infra     infrafleetv1.InfraFleetServiceClient
	resolver  connectionResolver
	providers aiproviderv1.AiProviderServiceClient // optional: picks the account for ai.complete
	timeout   time.Duration
}

var _ usecase.RequestClassifier = (*RelayClassifier)(nil)

func NewRelayClassifier(infra infrafleetv1.InfraFleetServiceClient, resolver connectionResolver, providers aiproviderv1.AiProviderServiceClient) *RelayClassifier {
	return &RelayClassifier{infra: infra, resolver: resolver, providers: providers, timeout: defaultClassifierTimeout}
}

func (c *RelayClassifier) Classify(ctx context.Context, in usecase.ClassificationInput) (domain.ClassificationProposal, error) {
	conn, err := c.resolver.ResolveForProject(ctx, in.ProjectID)
	if err != nil {
		return domain.ClassificationProposal{}, err
	}
	accountID := c.accountID(ctx, in.ProjectID)
	prompt := BuildClassificationPrompt(in)

	text, err := c.complete(ctx, conn, accountID, prompt)
	if err != nil {
		return domain.ClassificationProposal{}, err
	}
	p, perr := domain.ParseClassificationProposal([]byte(text))
	if perr == nil {
		return p, nil
	}
	if !errors.Is(perr, domain.ErrProposalInvalid) {
		return domain.ClassificationProposal{}, perr
	}
	// One retry with a stricter reminder; a second bad answer is final.
	text, err = c.complete(ctx, conn, accountID, prompt+"\n\nYour previous reply was not valid. Reply with only the JSON object.")
	if err != nil {
		return domain.ClassificationProposal{}, err
	}
	return domain.ParseClassificationProposal([]byte(text))
}

// accountID is best effort: without it the dev server falls back to its own default account.
func (c *RelayClassifier) accountID(ctx context.Context, projectID string) string {
	if c.providers == nil {
		return ""
	}
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return ""
	}
	userID, _ := tenant.UserID(ctx)
	pctx, err := withIdentityMetadata(ctx)
	if err != nil {
		return ""
	}
	resp, err := c.providers.ResolveProvider(pctx, &aiproviderv1.ResolveProviderRequest{TenantId: tenantID, UserId: userID, ProjectId: projectID})
	if err != nil {
		return ""
	}
	return resp.GetAccount().GetId()
}

func (c *RelayClassifier) complete(ctx context.Context, conn AIConnection, accountID, prompt string) (string, error) {
	return relayAIComplete(ctx, c.infra, conn, accountID, prompt, c.timeout)
}

// relayAIComplete runs ai.complete through Relay (infra connection) or RelayByDevServer (default-repo dev server).
func relayAIComplete(ctx context.Context, infra infrafleetv1.InfraFleetServiceClient, conn AIConnection, accountID, prompt string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return "", err
	}
	params := map[string]any{"prompt": prompt}
	if accountID != "" {
		params["accountId"] = accountID
	}
	raw, err := json.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("grpcclient: marshal ai.complete params: %w", err)
	}
	var resultJSON string
	if conn.DevServerID != "" {
		var resp *infrafleetv1.RelayResponse
		resp, err = infra.RelayByDevServer(ctx, &infrafleetv1.RelayByDevServerRequest{DevServerId: conn.DevServerID, Method: "ai.complete", ParamsJson: string(raw)})
		resultJSON = resp.GetResultJson()
	} else {
		var resp *infrafleetv1.RelayResponse
		resp, err = infra.Relay(ctx, &infrafleetv1.RelayRequest{ConnectionId: conn.ConnectionID, Method: "ai.complete", ParamsJson: string(raw)})
		resultJSON = resp.GetResultJson()
	}
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || status.Code(err) == codes.DeadlineExceeded {
			return "", usecase.ErrClassifierTimeout
		}
		return "", fmt.Errorf("grpcclient: relay ai.complete: %w", err)
	}
	var result struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
		return "", fmt.Errorf("grpcclient: decode ai.complete result: %w", err)
	}
	return result.Content, nil
}

// UnavailableClassifier is used when downstream addresses are not configured so the service still starts.
type UnavailableClassifier struct{}

func (UnavailableClassifier) Classify(context.Context, usecase.ClassificationInput) (domain.ClassificationProposal, error) {
	return domain.ClassificationProposal{}, usecase.ErrNoDevServer
}
