package wscompat

import (
	"context"
	"errors"
)

type architectureArgs struct {
	codeIntelSelector
	Container     string `json:"container,omitempty"`
	IncludeHidden bool   `json:"includeHidden,omitempty"`
	IfNoneMatch   string `json:"ifNoneMatch,omitempty"`
}

func (a architectureArgs) validate() error {
	if err := a.codeIntelSelector.validate(); err != nil {
		return err
	}
	if len(a.Container) > 512 {
		return invalidParam("container", "too_long")
	}
	if err := checkStringMax("ifNoneMatch", a.IfNoneMatch, 80); err != nil {
		return err
	}
	return nil
}

type dataFlowsArgs struct {
	codeIntelSelector
	TriggerKind string `json:"triggerKind,omitempty"`
	Query       string `json:"query,omitempty"`
	Service     string `json:"service,omitempty"`
	Limit       *int   `json:"limit,omitempty"`
	PageToken   string `json:"pageToken,omitempty"`
	IfNoneMatch string `json:"ifNoneMatch,omitempty"`
}

func (a dataFlowsArgs) validate() error {
	if err := a.codeIntelSelector.validate(); err != nil {
		return err
	}
	if err := checkBoundedInt("limit", a.Limit, 1, 100); err != nil {
		return err
	}
	if len(a.Query) > 128 {
		return invalidParam("query", "too_long")
	}
	if err := checkPageToken("pageToken", a.PageToken); err != nil {
		return err
	}
	if err := checkStringMax("ifNoneMatch", a.IfNoneMatch, 80); err != nil {
		return err
	}
	return nil
}

type dataFlowArgs struct {
	codeIntelSelector
	FlowID         string `json:"flowId"`
	Dialect        string `json:"dialect,omitempty"`
	Detail         string `json:"detail,omitempty"`
	MaxServiceHops *int   `json:"maxServiceHops,omitempty"`
	MaxSteps       *int   `json:"maxSteps,omitempty"`
	IncludeSequence bool  `json:"includeSequence,omitempty"`
	IncludeDfd     bool   `json:"includeDfd,omitempty"`
	IfNoneMatch    string `json:"ifNoneMatch,omitempty"`
}

func (a dataFlowArgs) validate() error {
	if err := a.codeIntelSelector.validate(); err != nil {
		return err
	}
	if a.FlowID == "" {
		return invalidParam("flowId", "required")
	}
	if err := checkBoundedInt("maxServiceHops", a.MaxServiceHops, 0, 8); err != nil {
		return err
	}
	if err := checkBoundedInt("maxSteps", a.MaxSteps, 0, 200); err != nil {
		return err
	}
	if a.Dialect != "" && a.Dialect != "postgres" && a.Dialect != "mysql" {
		return invalidParam("dialect", "invalid_enum")
	}
	if a.Detail != "" && a.Detail != "service" && a.Detail != "component" {
		return invalidParam("detail", "invalid_enum")
	}
	if err := checkStringMax("ifNoneMatch", a.IfNoneMatch, 80); err != nil {
		return err
	}
	return nil
}

// registerCodeIntelSourcesPlaceholders registers placeholders for architecture and data flow channels that wait for CR-033/034.
func registerCodeIntelSourcesPlaceholders(r *Registry, d codeIntelDeps) {
	specArch := mustCatalogSpec("codeIntel.architecture")
	registerCodeIntelUnary(r, d, specArch.Name, func(ctx context.Context, c codeIntelCaller, _ Identity, in architectureArgs) (any, error) {
		return nil, errors.New("CODEINTEL_UNAVAILABLE: channel not wired (waiting for CR-033)")
	})

	specFlows := mustCatalogSpec("codeIntel.dataFlows")
	registerCodeIntelUnary(r, d, specFlows.Name, func(ctx context.Context, c codeIntelCaller, _ Identity, in dataFlowsArgs) (any, error) {
		return nil, errors.New("CODEINTEL_UNAVAILABLE: channel not wired (waiting for CR-034)")
	})

	specFlow := mustCatalogSpec("codeIntel.dataFlow")
	registerCodeIntelUnary(r, d, specFlow.Name, func(ctx context.Context, c codeIntelCaller, _ Identity, in dataFlowArgs) (any, error) {
		return nil, errors.New("CODEINTEL_UNAVAILABLE: channel not wired (waiting for CR-034)")
	})
}
