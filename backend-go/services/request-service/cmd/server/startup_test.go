package main

import (
	"context"
	"net"
	"strings"
	"testing"

	commonconfig "github.com/stablyai/orca-go/common/config"
	"github.com/stablyai/orca-go/services/request-service/internal/config"
)

const (
	testGatewayToken = "gateway-test-token"
	testServiceToken = "service-test-token"
)

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func testConfig(t *testing.T, dsn string) config.Config {
	t.Helper()
	return config.Config{Base: commonconfig.Base{
		ServiceName: "request-service", GRPCPort: freePort(t), HTTPPort: freePort(t), DatabaseDSN: dsn,
	}, GatewayInternalToken: testGatewayToken, ServiceInternalToken: testServiceToken, OPABundlePath: "../../../../policy/orca-authz",
		WebhookRequireTimestamp: true}
}

func TestRun_UnknownDSNExitsWithDialectError(t *testing.T) {
	err := run(context.Background(), testConfig(t, "sqlite://x"))
	if err == nil || !strings.Contains(err.Error(), "unrecognized DSN scheme") {
		t.Fatalf("want unrecognized DSN scheme error, got %v", err)
	}
}
