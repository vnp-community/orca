package grpc

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/common/internalcaller"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

var oauthPublicMethods = map[string]bool{
	"OAuthRegisterClient": true, "OAuthValidateAuthorizeRequest": true, "OAuthExchangeToken": true, "OAuthRevokeToken": true,
}

// Every OAuth RPC must be deliberately classified, so a future RPC can't ship
// internal-only in the spec but unguarded in code.
func TestEveryOAuthRPCIsClassifiedPublicOrGuarded(t *testing.T) {
	internal := map[string]bool{}
	for _, m := range OAuthInternalMethods {
		internal[m] = true
	}
	seen := 0
	for _, m := range authv1.AuthService_ServiceDesc.Methods {
		if !strings.HasPrefix(m.MethodName, "OAuth") {
			continue
		}
		seen++
		full := "/" + authv1.AuthService_ServiceDesc.ServiceName + "/" + m.MethodName
		if internal[full] == oauthPublicMethods[m.MethodName] {
			t.Errorf("%s must be exactly one of public or internal-only", m.MethodName)
		}
	}
	if seen != len(OAuthInternalMethods)+len(oauthPublicMethods) {
		t.Fatalf("saw %d OAuth RPCs, classified %d", seen, len(OAuthInternalMethods)+len(oauthPublicMethods))
	}
}

func TestInternalOAuthMethodsRejectCallersWithoutTheSharedSecret(t *testing.T) {
	guard := internalcaller.Guard("secret", OAuthInternalMethods...)
	run := func(method string) error {
		_, err := guard(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: method},
			func(context.Context, any) (any, error) { return nil, nil })
		return err
	}
	for _, m := range OAuthInternalMethods {
		if status.Code(run(m)) != codes.PermissionDenied {
			t.Errorf("%s reachable without the internal-caller secret", m)
		}
	}
	for name := range oauthPublicMethods {
		if err := run("/" + authv1.AuthService_ServiceDesc.ServiceName + "/" + name); err != nil {
			t.Errorf("public method %s was blocked: %v", name, err)
		}
	}
}

func TestOAuthServerOverridesUnimplementedMethods(t *testing.T) {
	var srv authv1.AuthServiceServer = WithOAuth(&Server{}, OAuthUsecases{})
	if _, ok := srv.(*OAuthServer); !ok {
		t.Fatal("OAuthServer must satisfy AuthServiceServer")
	}
}
