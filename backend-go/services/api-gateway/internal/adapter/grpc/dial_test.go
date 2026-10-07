package grpc

import (
	"context"
	"testing"

	"google.golang.org/grpc/metadata"

	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

func TestAttachIdentity_WithActorTypeAgent(t *testing.T) {
	ctx := tenant.WithActorType(context.Background(), tenant.ActorAgent)
	id := usecase.Identity{TenantID: "t1"}

	ctxOut := AttachIdentity(ctx, id)
	md, ok := metadata.FromOutgoingContext(ctxOut)
	if !ok {
		t.Fatal("expected outgoing metadata")
	}

	actorType := md.Get(grpcmw.MetadataActorType)
	if len(actorType) == 0 || actorType[0] != tenant.ActorAgent {
		t.Errorf("expected actor type agent, got %v", actorType)
	}
}

func TestAttachIdentity_WithoutActorTypeGetsUser(t *testing.T) {
	ctx := context.Background()
	id := usecase.Identity{TenantID: "t1"}

	ctxOut := AttachIdentity(ctx, id)
	md, ok := metadata.FromOutgoingContext(ctxOut)
	if !ok {
		t.Fatal("expected outgoing metadata")
	}

	actorType := md.Get(grpcmw.MetadataActorType)
	if len(actorType) == 0 || actorType[0] != tenant.ActorUser {
		t.Errorf("expected actor type user, got %v", actorType)
	}
}
