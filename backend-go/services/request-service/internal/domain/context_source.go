package domain

import (
	"fmt"
	"regexp"
	"time"
)

type SourceKind string

const (
	KindRequestOrigin   SourceKind = "request_origin"
	KindConventions     SourceKind = "conventions"
	KindDecisions       SourceKind = "decisions"
	KindSpecs           SourceKind = "specs"
	KindServiceCatalog  SourceKind = "service_catalog"
	KindCodeGraph       SourceKind = "code_graph"
	KindGitHistory      SourceKind = "git_history"
	KindContracts       SourceKind = "contracts"
	KindSchema          SourceKind = "schema"
	KindDependencies    SourceKind = "dependencies"
	KindCIConfig        SourceKind = "ci_config"
	KindCIResults       SourceKind = "ci_results"
	KindCoverage        SourceKind = "coverage"
	KindObservability   SourceKind = "observability"
	KindIncidents       SourceKind = "incidents"
	KindFeatureFlags    SourceKind = "feature_flags"
	KindSecurityScan    SourceKind = "security_scan"
	KindPolicy          SourceKind = "policy"
	KindOwnership       SourceKind = "ownership"
	KindHistory         SourceKind = "history"
	KindDevServerProfile SourceKind = "dev_server_profile"
	KindExternalKnowledge SourceKind = "external_knowledge"
)

type Transport string

const (
	TransportInternal Transport = "internal"
	TransportMCP      Transport = "mcp"
)

type Trust string

const (
	TrustHigh   Trust = "high"
	TrustMedium Trust = "medium"
	TrustLow    Trust = "low"
)

type SourceStatus string

const (
	StatusDraft    SourceStatus = "draft"
	StatusActive   SourceStatus = "active"
	StatusDisabled SourceStatus = "disabled"
)

type EnabledFor struct {
	Projects     []string `json:"projects"`
	RequestTypes []string `json:"request_types"`
	Stages       []string `json:"stages"`
}

func (e EnabledFor) Match(projectID string, t string, st string) bool {
	matchList := func(list []string, val string) bool {
		if len(list) == 0 {
			return false
		}
		for _, v := range list {
			if v == "*" || v == val {
				return true
			}
		}
		return false
	}
	return matchList(e.Projects, projectID) && matchList(e.RequestTypes, t) && matchList(e.Stages, st)
}

type RedactionConfig struct {
	Profiles []string `json:"profiles"`
}

type ContextSource struct {
	ID                 string
	TenantID           string
	Key                string
	Kind               SourceKind
	Transport          Transport
	Adapter            string
	ServerRef          string
	Scopes             []string
	Trust              Trust
	TTLSeconds         int
	MaxBytes           int
	RateLimitPerMinute int
	Redaction          RedactionConfig
	EnabledFor         EnabledFor
	Status             SourceStatus
	OwnerID            string
	CreatedBy          string
	Version            int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type ErrSourceInvalid struct {
	Field  string
	Reason string
}

func (e ErrSourceInvalid) Error() string {
	return fmt.Sprintf("invalid context source: %s %s", e.Field, e.Reason)
}

var keyRegex = regexp.MustCompile(`^[a-z][a-z0-9_]{1,63}$`)

func (s ContextSource) Validate() error {
	if !keyRegex.MatchString(s.Key) {
		return ErrSourceInvalid{Field: "Key", Reason: "must match ^[a-z][a-z0-9_]{1,63}$"}
	}

	validKinds := map[SourceKind]bool{
		KindRequestOrigin: true, KindConventions: true, KindDecisions: true, KindSpecs: true,
		KindServiceCatalog: true, KindCodeGraph: true, KindGitHistory: true, KindContracts: true,
		KindSchema: true, KindDependencies: true, KindCIConfig: true, KindCIResults: true,
		KindCoverage: true, KindObservability: true, KindIncidents: true, KindFeatureFlags: true,
		KindSecurityScan: true, KindPolicy: true, KindOwnership: true, KindHistory: true,
		KindDevServerProfile: true, KindExternalKnowledge: true,
	}

	if !validKinds[s.Kind] {
		return ErrSourceInvalid{Field: "Kind", Reason: "invalid source kind"}
	}

	if s.Transport == TransportInternal {
		if s.Adapter == "" {
			return ErrSourceInvalid{Field: "Adapter", Reason: "must not be empty for internal transport"}
		}
		if s.ServerRef != "" {
			return ErrSourceInvalid{Field: "ServerRef", Reason: "must be empty for internal transport"}
		}
	} else if s.Transport == TransportMCP {
		if s.ServerRef == "" {
			return ErrSourceInvalid{Field: "ServerRef", Reason: "must be a valid UUID for mcp transport"}
		}
		if s.Adapter != "" {
			return ErrSourceInvalid{Field: "Adapter", Reason: "must be empty for mcp transport"}
		}
		if len(s.Scopes) == 0 {
			return ErrSourceInvalid{Field: "Scopes", Reason: "must not be empty for mcp transport"}
		}
		mcpKinds := map[SourceKind]bool{
			KindCIResults: true, KindCoverage: true, KindObservability: true, KindIncidents: true,
			KindFeatureFlags: true, KindSecurityScan: true, KindExternalKnowledge: true,
			KindDecisions: true, KindSpecs: true,
		}
		if !mcpKinds[s.Kind] {
			return ErrSourceInvalid{Field: "Kind", Reason: "cannot be sourced from mcp"}
		}
		if s.Trust == TrustHigh {
			return ErrSourceInvalid{Field: "Trust", Reason: "mcp source cannot have high trust"}
		}
	} else {
		return ErrSourceInvalid{Field: "Transport", Reason: "must be internal or mcp"}
	}

	if s.MaxBytes < 1024 || s.MaxBytes > 1048576 {
		return ErrSourceInvalid{Field: "MaxBytes", Reason: "must be between 1024 and 1048576"}
	}

	if s.OwnerID == "" {
		return ErrSourceInvalid{Field: "OwnerID", Reason: "must not be empty"}
	}

	return nil
}
