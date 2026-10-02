package grpc

import (
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"

	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

func TestToStatusMapsDomainErrorsToCodedMessages(t *testing.T) {
	cases := []struct {
		err  error
		code codes.Code
		msg  string
	}{
		{domain.ErrConsentNotFound(), codes.NotFound, "MCP_CONSENT_NOT_FOUND: consent request not found"},
		{domain.ErrConsentExpired(), codes.FailedPrecondition, "MCP_CONSENT_EXPIRED: consent request has expired"},
		{domain.ErrScopeInvalid("x"), codes.InvalidArgument, "MCP_SCOPE_INVALID: x"},
		{domain.ErrScopeNotAllowed("y"), codes.PermissionDenied, "MCP_SCOPE_NOT_ALLOWED: y"},
		{domain.ErrNotAdmin(), codes.PermissionDenied, "MCP_NOT_ADMIN: administrator role required"},
		{domain.ErrNotFound(), codes.NotFound, "MCP_NOT_FOUND: not found"},
		{domain.ErrClientNotAllowed(), codes.PermissionDenied, "MCP_CLIENT_NOT_ALLOWED: client is not allowed in this tenant"},
		{domain.ErrDisabled(), codes.FailedPrecondition, "MCP_DISABLED: MCP is disabled for this tenant"},
	}
	for _, tc := range cases {
		st := status.Convert(toStatus(tc.err))
		if st.Code() != tc.code || st.Message() != tc.msg {
			t.Errorf("got %v %q, want %v %q", st.Code(), st.Message(), tc.code, tc.msg)
		}
	}
}

func TestToStatusKeepsTransportStatuses(t *testing.T) {
	in := status.Error(codes.Unavailable, "MCP_UNAVAILABLE: authorization server is unavailable")
	st := status.Convert(toStatus(in))
	if st.Code() != codes.Unavailable || st.Message() != "MCP_UNAVAILABLE: authorization server is unavailable" {
		t.Fatalf("got %v %q", st.Code(), st.Message())
	}
	if status.Convert(toStatus(errors.New("raw"))).Code() != codes.Internal {
		t.Fatal("unknown errors must not leak")
	}
}

func TestAuthorizationServerOverridesUnimplemented(t *testing.T) {
	var srv mcpv1.McpServiceServer = WithAuthorization(&Server{}, AuthorizationUsecases{})
	if _, ok := srv.(*AuthorizationServer); !ok {
		t.Fatal("must satisfy McpServiceServer")
	}
}
