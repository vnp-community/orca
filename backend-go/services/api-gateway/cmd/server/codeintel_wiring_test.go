package main

import (
	"context"
	"net"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/common/internalcaller"
	codeintelv1 "github.com/stablyai/orca-go/proto/gen/go/orca/codeintel/v1"
	"github.com/stablyai/orca-go/services/api-gateway/internal/config"
)

type guardedQualityStub struct {
	codeintelv1.UnimplementedQualityGateServiceServer
	largePayload bool
}

func (s guardedQualityStub) GetQualityGate(_ context.Context, _ *codeintelv1.GetQualityGateRequest) (*codeintelv1.GetQualityGateResponse, error) {
	resp := &codeintelv1.GetQualityGateResponse{
		Gate: &codeintelv1.QualityGate{Status: "pass"},
	}
	if s.largePayload {
		// 5 MiB digest to trigger MaxCallRecvMsgSize (4 MiB ceiling)
		resp.ProfileDefinitionDigest = strings.Repeat("A", 5<<20)
	}
	return resp, nil
}

func TestBuildCodeIntelClientsEmptyAddr(t *testing.T) {
	cfg := config.CodeIntelConfig{ServiceAddr: ""}
	core, quality, conn, err := buildCodeIntelClients(cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if core != nil || quality != nil || conn != nil {
		t.Fatalf("expected nil clients and conn for empty addr, got core=%v, quality=%v, conn=%v", core, quality, conn)
	}
}

func TestDialCodeIntelServiceSendsToken(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(internalcaller.Guard("s3cret", codeintelv1.QualityGateService_GetQualityGate_FullMethodName)),
	)
	codeintelv1.RegisterQualityGateServiceServer(srv, guardedQualityStub{})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	// Call without token -> should fail
	t.Setenv("CODEINTEL_INTERNAL_CALLER_TOKEN", "")
	ccNoTok, err := dialCodeIntelService(lis.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ccNoTok.Close() })
	cNoTok := codeintelv1.NewQualityGateServiceClient(ccNoTok)
	_, err = cNoTok.GetQualityGate(context.Background(), &codeintelv1.GetQualityGateRequest{})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied without token, got: %v", err)
	}

	// Call with correct token -> should succeed
	t.Setenv("CODEINTEL_INTERNAL_CALLER_TOKEN", "s3cret")
	ccTok, err := dialCodeIntelService(lis.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ccTok.Close() })
	cTok := codeintelv1.NewQualityGateServiceClient(ccTok)
	if _, err := cTok.GetQualityGate(context.Background(), &codeintelv1.GetQualityGateRequest{}); err != nil {
		t.Fatalf("unexpected error with token: %v", err)
	}
}

func TestDialCodeIntelServiceMaxCallRecvMsgSize(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	codeintelv1.RegisterQualityGateServiceServer(srv, guardedQualityStub{largePayload: true})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	cc, err := dialCodeIntelService(lis.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cc.Close() })

	client := codeintelv1.NewQualityGateServiceClient(cc)
	_, err = client.GetQualityGate(context.Background(), &codeintelv1.GetQualityGateRequest{})
	if status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("expected ResourceExhausted for payload > 4 MiB, got: %v", err)
	}
}
