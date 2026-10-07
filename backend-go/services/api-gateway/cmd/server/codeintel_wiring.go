package main

import (
	"log/slog"
	"os"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/stablyai/orca-go/common/internalcaller"
	codeintelv1 "github.com/stablyai/orca-go/proto/gen/go/orca/codeintel/v1"
	"github.com/stablyai/orca-go/services/api-gateway/internal/config"
)

const (
	codeIntelInternalTokenEnv  = "CODEINTEL_INTERNAL_CALLER_TOKEN"
	codeIntelMaxCallRecvMsgSize = 4 << 20 // 4 MiB (PQ-14)
)

func dialCodeIntelService(addr string) (*grpc.ClientConn, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(codeIntelMaxCallRecvMsgSize)),
	}
	if tok := os.Getenv(codeIntelInternalTokenEnv); tok != "" {
		opts = append(opts,
			grpc.WithChainUnaryInterceptor(internalcaller.ClientInterceptor(tok)),
			grpc.WithChainStreamInterceptor(internalcaller.StreamClientInterceptor(tok)),
		)
	}
	return grpc.NewClient(addr, opts...)
}

func buildCodeIntelClients(cfg config.CodeIntelConfig, logger *slog.Logger) (codeintelv1.CodeIntelServiceClient, codeintelv1.QualityGateServiceClient, *grpc.ClientConn, error) {
	if cfg.ServiceAddr == "" {
		if logger != nil {
			logger.Warn("code-intel-service address empty: client disabled")
		}
		return nil, nil, nil, nil
	}

	if os.Getenv(codeIntelInternalTokenEnv) == "" {
		if logger != nil {
			logger.Warn("internal caller token empty: code-intel-service will reject calls")
		}
	}

	conn, err := dialCodeIntelService(cfg.ServiceAddr)
	if err != nil {
		return nil, nil, nil, err
	}

	return codeintelv1.NewCodeIntelServiceClient(conn), codeintelv1.NewQualityGateServiceClient(conn), conn, nil
}
