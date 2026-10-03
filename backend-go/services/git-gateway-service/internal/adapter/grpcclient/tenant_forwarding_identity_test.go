package grpcclient

import (
	"context"
	"testing"

	"google.golang.org/grpc/metadata"

	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/tenant"
)

func TestWithIdentityMetadata_ForwardsTenantAndUser(t *testing.T) {
	ctx := tenant.WithUserID(tenant.WithTenantID(context.Background(), "t1"), "u1")
	out, err := withIdentityMetadata(ctx)
	if err != nil {
		t.Fatal(err)
	}
	md, _ := metadata.FromOutgoingContext(out)
	if got := md.Get(grpcmw.MetadataTenantID); len(got) != 1 || got[0] != "t1" {
		t.Errorf("tenant = %v", got)
	}
	if got := md.Get(grpcmw.MetadataUserID); len(got) != 1 || got[0] != "u1" {
		t.Errorf("user = %v", got)
	}
}

func TestWithIdentityMetadata_NoUserSendsTenantOnly(t *testing.T) {
	out, err := withIdentityMetadata(tenant.WithTenantID(context.Background(), "t1"))
	if err != nil {
		t.Fatal(err)
	}
	md, _ := metadata.FromOutgoingContext(out)
	if got := md.Get(grpcmw.MetadataUserID); len(got) != 0 {
		t.Errorf("no user must not send an empty user header, got %v", got)
	}
}

func TestWithIdentityMetadata_RequiresTenant(t *testing.T) {
	if _, err := withIdentityMetadata(context.Background()); err == nil {
		t.Fatal("must fail closed without a tenant, like withTenantMetadata")
	}
}

// The shared helper must stay tenant-only: other outbound calls never had a
// user and forwarding one could change their authorization outcome.
func TestWithTenantMetadata_StaysTenantOnly(t *testing.T) {
	ctx := tenant.WithUserID(tenant.WithTenantID(context.Background(), "t1"), "u1")
	out, err := withTenantMetadata(ctx)
	if err != nil {
		t.Fatal(err)
	}
	md, _ := metadata.FromOutgoingContext(out)
	if got := md.Get(grpcmw.MetadataUserID); len(got) != 0 {
		t.Errorf("withTenantMetadata must not forward the user, got %v", got)
	}
}
