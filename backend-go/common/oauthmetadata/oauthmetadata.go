// Package oauthmetadata builds the public OAuth discovery documents Orca
// serves for MCP clients: RFC 9728 protected-resource metadata and RFC 8414
// authorization-server metadata. It only builds data; the HTTP routes live
// in api-gateway. The document advertises exactly what auth-service
// implements (public clients, PKCE S256, authorization_code + refresh_token)
// and nothing more, so clients never try an unsupported feature.
package oauthmetadata

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

const (
	AuthorizePath = "/oauth/authorize"
	TokenPath     = "/oauth/token"
	RegisterPath  = "/oauth/register"
	RevokePath    = "/oauth/revoke"
	ResourcePath  = "/mcp"

	WellKnownProtectedResource   = "/.well-known/oauth-protected-resource"
	WellKnownAuthorizationServer = "/.well-known/oauth-authorization-server"
)

// ProtectedResource is the RFC 9728 document.
type ProtectedResource struct {
	Resource               string   `json:"resource"`
	AuthorizationServers   []string `json:"authorization_servers"`
	ScopesSupported        []string `json:"scopes_supported"`
	BearerMethodsSupported []string `json:"bearer_methods_supported"`
}

// AuthorizationServer is the RFC 8414 document. RegistrationEndpoint is
// omitted when dynamic registration is off. There is deliberately no
// jwks_uri: MCP clients treat the access token as opaque.
type AuthorizationServer struct {
	Issuer                                     string   `json:"issuer"`
	AuthorizationEndpoint                      string   `json:"authorization_endpoint"`
	TokenEndpoint                              string   `json:"token_endpoint"`
	RevocationEndpoint                         string   `json:"revocation_endpoint"`
	RegistrationEndpoint                       string   `json:"registration_endpoint,omitempty"`
	ResponseTypesSupported                     []string `json:"response_types_supported"`
	GrantTypesSupported                        []string `json:"grant_types_supported"`
	CodeChallengeMethodsSupported              []string `json:"code_challenge_methods_supported"`
	TokenEndpointAuthMethodsSupported          []string `json:"token_endpoint_auth_methods_supported"`
	ScopesSupported                            []string `json:"scopes_supported"`
	AuthorizationResponseIssParameterSupported bool     `json:"authorization_response_iss_parameter_supported"`
}

// ResourceURL returns the MCP resource identifier for a public base URL.
func ResourceURL(baseURL string) string { return strings.TrimRight(baseURL, "/") + ResourcePath }

// ProtectedResourceMetadataURL is the path-insert URL clients receive in
// WWW-Authenticate resource_metadata for the /mcp resource.
func ProtectedResourceMetadataURL(baseURL string) string {
	return strings.TrimRight(baseURL, "/") + WellKnownProtectedResource + ResourcePath
}

// ValidateBaseURL checks a public base URL: https (or http on a loopback
// host for development), no userinfo, query, fragment or path. The base URL
// must come from configuration, never from a request Host header.
func ValidateBaseURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("oauthmetadata: invalid base url: %w", err)
	}
	switch {
	case u.Host == "":
		return errors.New("oauthmetadata: base url has no host")
	case u.User != nil, u.RawQuery != "", u.Fragment != "", (u.Path != "" && u.Path != "/"):
		return errors.New("oauthmetadata: base url must be a bare origin")
	case u.Scheme == "https":
		return nil
	case u.Scheme == "http" && isLoopback(u.Hostname()):
		return nil
	}
	return errors.New("oauthmetadata: base url must be https (http only for loopback)")
}

func isLoopback(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// BuildProtectedResource builds the RFC 9728 document for baseURL.
func BuildProtectedResource(baseURL string, scopes []string) (ProtectedResource, error) {
	if err := ValidateBaseURL(baseURL); err != nil {
		return ProtectedResource{}, err
	}
	base := strings.TrimRight(baseURL, "/")
	return ProtectedResource{
		Resource:               ResourceURL(base),
		AuthorizationServers:   []string{base},
		ScopesSupported:        append([]string{}, scopes...),
		BearerMethodsSupported: []string{"header"},
	}, nil
}

// BuildAuthorizationServer builds the RFC 8414 document for issuer.
func BuildAuthorizationServer(issuer string, dcrEnabled bool, scopes []string) (AuthorizationServer, error) {
	if err := ValidateBaseURL(issuer); err != nil {
		return AuthorizationServer{}, err
	}
	base := strings.TrimRight(issuer, "/")
	doc := AuthorizationServer{
		Issuer:                                     base,
		AuthorizationEndpoint:                      base + AuthorizePath,
		TokenEndpoint:                              base + TokenPath,
		RevocationEndpoint:                         base + RevokePath,
		ResponseTypesSupported:                     []string{"code"},
		GrantTypesSupported:                        []string{"authorization_code", "refresh_token"},
		CodeChallengeMethodsSupported:              []string{"S256"},
		TokenEndpointAuthMethodsSupported:          []string{"none"},
		ScopesSupported:                            append([]string{}, scopes...),
		AuthorizationResponseIssParameterSupported: true,
	}
	if dcrEnabled {
		doc.RegistrationEndpoint = base + RegisterPath
	}
	return doc, nil
}
