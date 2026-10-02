package domain

import (
	"regexp"
	"sort"
	"strings"
	"time"
)

// Scope, transport and status vocabulary of CONTRACT McpExternalServer.
const (
	ScopeTenant = "tenant"
	ScopeTeam   = "team"
	ScopeUser   = "user"

	TransportHTTP  = "http"
	TransportStdio = "stdio"

	StatusPendingReview = "pending_review"
	StatusApproved      = "approved"
	StatusDisabled      = "disabled"

	SecretKindEnv    = "env"
	SecretKindHeader = "header"

	DecisionReject = "reject"
)

var (
	serverNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,62}$`)
	secretNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]{0,127}$`)
	// forbiddenHeaders can never be configured: the prober owns them or they
	// would enable request smuggling / identity confusion.
	forbiddenHeaders = map[string]bool{"host": true, "cookie": true, "content-length": true, "transfer-encoding": true, "connection": true, "upgrade": true}
)

// SecretRef names a secret slot (env var or header). The value lives only in
// credential-broker; BrokerOwnerID is the pointer, empty until set.
type SecretRef struct {
	Kind          string
	Name          string
	BrokerOwnerID string
	SetBy         string
	SetAt         *time.Time
}

func (r SecretRef) HasSecret() bool { return r.BrokerOwnerID != "" }

// BrokerOwner is the credential-broker owner_id convention for a secret slot.
func BrokerOwner(serverID, kind, name string) string {
	return "mcp:" + serverID + ":" + kind + ":" + name
}

type Health struct {
	OK        bool
	CheckedAt time.Time
	Error     string
}

// ExternalServer is a registry entry. It never carries a secret value.
type ExternalServer struct {
	ID, TenantID, Scope, ScopeID, Name string
	Transport, URL, Command            string
	Args                               []string
	EnvRefs, HeaderRefs                []SecretRef
	Status                             string
	SpecDigest                         string
	LastProbeDigest                    string
	LastProbeAt                        *time.Time
	ApprovedDigest                     string
	ApprovedTools                      []ToolInfo
	Health                             *Health
	CreatedBy, ReviewedBy              string
	ReviewedAt                         *time.Time
	Version                            int
	CreatedAt, UpdatedAt               time.Time
}

// ToolsDigest is the digest shown in the UI: what was approved, else what was last seen.
func (s ExternalServer) ToolsDigest() string {
	if s.ApprovedDigest != "" {
		return s.ApprovedDigest
	}
	return s.LastProbeDigest
}

// ToolsChanged is the rug-pull signal: the live digest differs from the approved one.
func (s ExternalServer) ToolsChanged() bool {
	return s.ApprovedDigest != "" && s.LastProbeDigest != s.ApprovedDigest
}

// Usable reports whether the server may be handed to an agent (fail closed).
func (s ExternalServer) Usable() bool {
	return s.Status == StatusApproved && !s.ToolsChanged()
}

func (s ExternalServer) Ref(kind, name string) (SecretRef, bool) {
	for _, r := range s.refs(kind) {
		if r.Name == name {
			return r, true
		}
	}
	return SecretRef{}, false
}

func (s ExternalServer) refs(kind string) []SecretRef {
	if kind == SecretKindHeader {
		return s.HeaderRefs
	}
	return s.EnvRefs
}

// SpecDigest covers everything that changes what runs or where it connects.
// Secret VALUES are excluded (they are not in the registry); their names are
// included so adding a new env var or header requires a new review.
func ComputeSpecDigest(transport, url, command string, args, envNames, headerNames []string) string {
	e := append([]string(nil), envNames...)
	h := append([]string(nil), headerNames...)
	sort.Strings(e)
	sort.Strings(h)
	if args == nil {
		args = []string{}
	}
	return digestOf(map[string]any{"transport": transport, "url": url, "command": command, "args": args, "env": e, "headers": h})
}

// ServerPolicy is the operator-controlled validation context.
type ServerPolicy struct {
	URL          ExternalURLPolicy
	StdioEnabled bool
}

// ServerSpec is the client-supplied part of an upsert.
type ServerSpec struct {
	Scope, ScopeID, Name, Transport, URL, Command string
	Args, EnvNames, HeaderNames                   []string
}

// ValidateSpec checks a spec before it is stored.
func ValidateSpec(sp ServerSpec, pol ServerPolicy) error {
	if !serverNameRe.MatchString(sp.Name) {
		return ErrServerInvalid("name must match ^[a-z0-9][a-z0-9_-]{0,62}$")
	}
	switch sp.Scope {
	case ScopeTenant, ScopeTeam, ScopeUser:
	default:
		return ErrServerInvalid("scope must be tenant, team or user")
	}
	if len(sp.Args) > 64 {
		return ErrServerInvalid("too many args")
	}
	for _, n := range sp.EnvNames {
		if !secretNameRe.MatchString(n) {
			return ErrServerInvalid("env reference names must be identifiers")
		}
	}
	for _, n := range sp.HeaderNames {
		if !secretNameRe.MatchString(n) || forbiddenHeaders[strings.ToLower(n)] || strings.HasPrefix(strings.ToLower(n), "proxy-") {
			return ErrServerInvalid("header reference name is not allowed")
		}
	}
	if hasDuplicate(sp.EnvNames) || hasDuplicate(sp.HeaderNames) {
		return ErrServerInvalid("duplicate reference names")
	}
	switch sp.Transport {
	case TransportHTTP:
		if sp.Command != "" || len(sp.Args) > 0 || len(sp.EnvNames) > 0 {
			return ErrServerInvalid("http servers take a url and header references only")
		}
		if _, err := ValidateExternalURL(sp.URL, pol.URL); err != nil {
			return err
		}
	case TransportStdio:
		if sp.URL != "" || len(sp.HeaderNames) > 0 {
			return ErrServerInvalid("stdio servers take a command, args and env references only")
		}
		if !pol.StdioEnabled {
			return ErrStdioNotAllowed("stdio servers are disabled on this deployment")
		}
		return ValidateStdioCommand(sp.Command, sp.Args)
	default:
		return ErrServerInvalid("transport must be http or stdio")
	}
	return nil
}

func hasDuplicate(xs []string) bool {
	seen := map[string]bool{}
	for _, x := range xs {
		if seen[x] {
			return true
		}
		seen[x] = true
	}
	return false
}

var (
	bareCommandRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
	// pinnedPkgRe: name@x.y.z with a digit-led version (no tags like @latest).
	pinnedPkgRe = regexp.MustCompile(`^(@[a-z0-9._-]+/)?[A-Za-z0-9._-]+@[0-9]+\.[0-9]+\.[0-9]+([-+][0-9A-Za-z.-]+)?$`)
	runners     = map[string]bool{"npx": true, "uvx": true, "bunx": true, "pnpx": true}
	evalFlags   = map[string]bool{"-c": true, "-e": true, "-p": true, "--eval": true, "--print": true, "--command": true, "--exec": true, "/c": true, "/k": true, "-r": true, "--require": true, "--import": true}
)

// ValidateStdioCommand is the launch-spec policy: a bare executable name (no
// path, so PATH/launcher decides), no inline-code flags, package runners must
// pin an exact version, and no secrets in args.
func ValidateStdioCommand(command string, args []string) error {
	if !bareCommandRe.MatchString(command) {
		return ErrServerInvalid("command must be a bare executable name")
	}
	pkgChecked := !runners[command]
	for _, a := range args {
		if len(a) > 1024 || strings.ContainsRune(a, 0) {
			return ErrServerInvalid("argument is too long or contains a NUL byte")
		}
		if evalFlags[strings.ToLower(a)] {
			return ErrServerInvalid("inline-code flags are not allowed")
		}
		if _, red := (SecretRedactor{}).Redact(a); red {
			return ErrServerInvalid("args must not embed secrets; use an env reference")
		}
		if !pkgChecked && !strings.HasPrefix(a, "-") {
			if !pinnedPkgRe.MatchString(a) {
				return ErrServerInvalid("package must be pinned as name@x.y.z")
			}
			pkgChecked = true
		}
	}
	if !pkgChecked {
		return ErrServerInvalid("package must be pinned as name@x.y.z")
	}
	return nil
}
