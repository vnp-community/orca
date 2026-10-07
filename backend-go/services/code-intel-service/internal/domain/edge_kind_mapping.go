package domain

import "strings"

type EdgeKind int

const (
	EdgeKindUnspecified EdgeKind = iota
	EdgeKindCalls
	EdgeKindImports
	EdgeKindAccesses
	EdgeKindExtends
	EdgeKindImplements
	EdgeKindMethodOverrides
	EdgeKindMethodImplements
	EdgeKindHasMethod
	EdgeKindHasProperty
	EdgeKindReferences
	EdgeKindInstantiates
	EdgeKindContains
	EdgeKindHandlesRoute
	EdgeKindFetches
)

func (k EdgeKind) String() string {
	switch k {
	case EdgeKindCalls:
		return "CALLS"
	case EdgeKindImports:
		return "IMPORTS"
	case EdgeKindAccesses:
		return "ACCESSES"
	case EdgeKindExtends:
		return "EXTENDS"
	case EdgeKindImplements:
		return "IMPLEMENTS"
	case EdgeKindMethodOverrides:
		return "METHOD_OVERRIDES"
	case EdgeKindMethodImplements:
		return "METHOD_IMPLEMENTS"
	case EdgeKindHasMethod:
		return "HAS_METHOD"
	case EdgeKindHasProperty:
		return "HAS_PROPERTY"
	case EdgeKindReferences:
		return "REFERENCES"
	case EdgeKindInstantiates:
		return "INSTANTIATES"
	case EdgeKindContains:
		return "CONTAINS"
	case EdgeKindHandlesRoute:
		return "HANDLES_ROUTE"
	case EdgeKindFetches:
		return "FETCHES"
	default:
		return "UNSPECIFIED"
	}
}

var structuralEdgeLabels = map[string]bool{
	"defines":         true,
	"member_of":       true,
	"step_in_process": true,
	"entry_point_of":  true,
}

var gitNexusEdgeMap = map[string]EdgeKind{
	"calls":             EdgeKindCalls,
	"imports":           EdgeKindImports,
	"accesses":          EdgeKindAccesses,
	"extends":           EdgeKindExtends,
	"implements":        EdgeKindImplements,
	"method_overrides":  EdgeKindMethodOverrides,
	"method_implements": EdgeKindMethodImplements,
	"has_method":        EdgeKindHasMethod,
	"has_property":      EdgeKindHasProperty,
	"references":        EdgeKindReferences,
	"instantiates":      EdgeKindInstantiates,
	"contains":          EdgeKindContains,
	"handles_route":     EdgeKindHandlesRoute,
	"fetches":           EdgeKindFetches,
}

var codeGraphEdgeMap = map[string]EdgeKind{
	"call":       EdgeKindCalls,
	"calls":      EdgeKindCalls,
	"import":     EdgeKindImports,
	"imports":    EdgeKindImports,
	"access":     EdgeKindAccesses,
	"accesses":   EdgeKindAccesses,
	"extend":     EdgeKindExtends,
	"extends":    EdgeKindExtends,
	"implement":  EdgeKindImplements,
	"implements": EdgeKindImplements,
	"override":   EdgeKindMethodOverrides,
	"overrides":  EdgeKindMethodOverrides,
	"reference":  EdgeKindReferences,
	"references": EdgeKindReferences,
	"instantiate": EdgeKindInstantiates,
	"contains":   EdgeKindContains,
	"contain":    EdgeKindContains,
	"route":      EdgeKindHandlesRoute,
	"fetch":      EdgeKindFetches,
}

// EdgeKindFromGitNexus maps GitNexus relationship labels.
// Structural edges return (0, false).
func EdgeKindFromGitNexus(label string) (EdgeKind, bool) {
	lower := strings.ToLower(label)
	if structuralEdgeLabels[lower] {
		return 0, false
	}
	if k, ok := gitNexusEdgeMap[lower]; ok {
		return k, true
	}
	return EdgeKindReferences, true
}

// EdgeKindFromCodeGraph maps CodeGraph relationship labels.
// Structural edges return (0, false).
func EdgeKindFromCodeGraph(rel string) (EdgeKind, bool) {
	lower := strings.ToLower(rel)
	if structuralEdgeLabels[lower] {
		return 0, false
	}
	if k, ok := codeGraphEdgeMap[lower]; ok {
		return k, true
	}
	return EdgeKindReferences, true
}

// ParseEdgeKind parses a relationship string into a canonical EdgeKind, checking
// both GitNexus and CodeGraph mappings. Unrecognized kinds default to EdgeKindReferences.
func ParseEdgeKind(s string) EdgeKind {
	lower := strings.ToLower(s)
	if k, ok := gitNexusEdgeMap[lower]; ok {
		return k
	}
	if k, ok := codeGraphEdgeMap[lower]; ok {
		return k
	}
	return EdgeKindReferences
}

