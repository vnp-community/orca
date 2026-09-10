package domain

import (
	"errors"
	"strings"
)

// TargetKind discriminates the three forms a step's ConnectionID field may
// now hold — see AgentStepConfig's doc comment (step.go) for why the field
// name stays "connectionId" while its semantics widen from "a literal,
// already-resolved connection ID" to "a target spec a ServerResolver must
// resolve first".
type TargetKind int

const (
	TargetKindProject  TargetKind = iota // "project:<id>"
	TargetKindServer                     // "server:<id>"
	TargetKindFleetTag                   // "fleet:tag:<tag>"
)

// ErrUnknownTargetKind is returned by ParseTargetSpec for a raw string
// matching none of the three known prefixes.
var ErrUnknownTargetKind = errors.New("domain: unknown target spec kind")

// TargetSpec is a step config's ConnectionID field, parsed into its kind
// plus the remaining identifier/tag.
type TargetSpec struct {
	Kind TargetKind
	ID   string // set for TargetKindProject / TargetKindServer
	Tag  string // set for TargetKindFleetTag
}

// ParseTargetSpec parses "project:<id>" / "server:<id>" / "fleet:tag:<tag>".
// A raw string matching none of these three prefixes is a caller/config
// error, not a silent default — surfaced as ErrUnknownTargetKind.
func ParseTargetSpec(raw string) (TargetSpec, error) {
	switch {
	case strings.HasPrefix(raw, "project:"):
		return TargetSpec{Kind: TargetKindProject, ID: strings.TrimPrefix(raw, "project:")}, nil
	case strings.HasPrefix(raw, "server:"):
		return TargetSpec{Kind: TargetKindServer, ID: strings.TrimPrefix(raw, "server:")}, nil
	case strings.HasPrefix(raw, "fleet:tag:"):
		return TargetSpec{Kind: TargetKindFleetTag, Tag: strings.TrimPrefix(raw, "fleet:tag:")}, nil
	default:
		return TargetSpec{}, ErrUnknownTargetKind
	}
}
