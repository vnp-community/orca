package domain

type Risk int

const (
	RiskUnspecified Risk = iota
	RiskLow
	RiskMedium
	RiskHigh
	RiskCritical
	RiskUnknown
)

func (r Risk) String() string {
	switch r {
	case RiskLow:
		return "LOW"
	case RiskMedium:
		return "MEDIUM"
	case RiskHigh:
		return "HIGH"
	case RiskCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

type ImpactSymbol struct {
	Symbol     SymbolRef
	Via        EdgeKind
	Direct     bool
	Confidence float32
}

type ImpactLevel struct {
	Depth   int32
	Symbols []ImpactSymbol
}

type AffectedFlow struct {
	FlowID      string
	Label       string
	StepCount   int32
	ChangedStep *int32
}

type AffectedCluster struct {
	ID     string
	Label  string
	Hits   int32
	Impact string
}

// ImpactGraph captures blast radius analysis for a target symbol.
type ImpactGraph struct {
	Target           SymbolRef
	Direction        string
	Risk             Risk
	Levels           []ImpactLevel
	AffectedFlows    []AffectedFlow
	AffectedClusters []AffectedCluster
	TestsCovering    []string
	ImpactedCount    int32
}
