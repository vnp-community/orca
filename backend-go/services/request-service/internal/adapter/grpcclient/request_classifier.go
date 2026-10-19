package grpcclient

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/stablyai/orca-go/common/agentrelay"
	"github.com/stablyai/orca-go/common/tenant"
	aiproviderv1 "github.com/stablyai/orca-go/proto/gen/go/orca/aiprovider/v1"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

const defaultClassifierTimeout = 60 * time.Second

type connectionResolver interface {
	ResolveForProject(ctx context.Context, projectID string) (AIConnection, error)
}

// RelayClassifier asks the AI through infra-fleet-service's Relay (ai.complete). It never
// trusts the reply: the proposal parser rejects anything outside the enums.
type RelayClassifier struct {
	relay     *agentrelay.Relayer
	resolver  connectionResolver
	providers aiproviderv1.AiProviderServiceClient // optional: picks the account for ai.complete
	caps      usecase.DevServerCapabilityReader    // optional: lets an unusable dev server fail fast with a remedy
	timeout   time.Duration
}

var _ usecase.RequestClassifier = (*RelayClassifier)(nil)

// WithCapabilities enables the readiness check before each AI call.
func (c *RelayClassifier) WithCapabilities(caps usecase.DevServerCapabilityReader) *RelayClassifier {
	c.caps = caps
	return c
}

func NewRelayClassifier(infra infrafleetv1.InfraFleetServiceClient, resolver connectionResolver, providers aiproviderv1.AiProviderServiceClient) *RelayClassifier {
	return &RelayClassifier{relay: newTenantRelayer(infra), resolver: resolver, providers: providers, timeout: defaultClassifierTimeout}
}

func (c *RelayClassifier) Classify(ctx context.Context, in usecase.ClassificationInput) (domain.ClassificationProposal, error) {
	conn, err := c.resolver.ResolveForProject(ctx, in.ProjectID)
	if err != nil {
		return domain.ClassificationProposal{}, err
	}
	if err := checkAIReady(ctx, c.caps, conn); err != nil {
		return domain.ClassificationProposal{}, err
	}
	accountID := c.accountID(ctx, in.ProjectID)
	prompt, err := RenderClassificationPrompt(in)
	if err != nil {
		return domain.ClassificationProposal{}, err
	}

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
	return relayAIComplete(ctx, c.relay, conn, accountID, prompt, c.timeout)
}

// relayAIComplete runs ai.complete through Relay (infra connection) or RelayByDevServer (default-repo dev server).
func relayAIComplete(ctx context.Context, relay *agentrelay.Relayer, conn AIConnection, accountID, prompt string, timeout time.Duration) (string, error) {
	res, err := relay.Complete(ctx, agentrelay.Target{DevServerID: conn.DevServerID, ConnectionID: conn.ConnectionID},
		agentrelay.CompleteParams{Prompt: prompt, AccountID: accountID, Timeout: timeout})
	if err != nil {
		return "", mapAICompleteError(err)
	}
	return res.Content, nil
}

// mapAICompleteError gives a typed ai.complete refusal the user-facing code this service owns (mapAIRelayError reads
// the original gRPC message from the cause); a timeout becomes the classifier's own.
func mapAICompleteError(err error) error {
	if errors.Is(err, agentrelay.ErrTimeout) {
		return usecase.ErrClassifierTimeout
	}
	var refused *agentrelay.CompleteError
	if errors.As(err, &refused) {
		if mapped := mapAIRelayError(refused.Cause); mapped != refused.Cause {
			return mapped
		}
		return fmt.Errorf("grpcclient: relay ai.complete: %w", refused.Cause)
	}
	return err
}

// UnavailableClassifier is used when downstream addresses are not configured so the service still starts.
type UnavailableClassifier struct{}

func (UnavailableClassifier) Classify(context.Context, usecase.ClassificationInput) (domain.ClassificationProposal, error) {
	return domain.ClassificationProposal{}, usecase.ErrNoDevServer
}
