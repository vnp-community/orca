package domain

type SourceGroup string

const (
	GroupA SourceGroup = "A" // Repository Foundation
	GroupB SourceGroup = "B" // Core Design
	GroupC SourceGroup = "C" // Interface & Contracts
	GroupD SourceGroup = "D" // SDLC & Quality
	GroupE SourceGroup = "E" // Operations & Live State
	GroupF SourceGroup = "F" // Request Context
	GroupG SourceGroup = "G" // Workspace Environment
	GroupH SourceGroup = "H" // AI Memory & Retrieval
)

type Requirement string

const (
	Required    Requirement = "Required"
	Recommended Requirement = "Recommended"
	None        Requirement = "None"
)

var DefaultStageMatrix = map[SourceGroup]map[string]Requirement{
	GroupA: {
		"solution": Recommended,
		"plan":     Recommended,
		"task":     Recommended,
		"execute":  Recommended,
	},
	GroupB: {
		"solution": Required,
		"plan":     Required,
		"task":     Required,
		"execute":  Required,
		"risk":     Required,
	},
	GroupC: {
		"solution": Recommended,
		"plan":     Recommended,
		"task":     Required,
		"execute":  Required,
	},
	GroupD: {
		"plan":    Recommended,
		"task":    Recommended,
		"execute": Recommended,
		"risk":    Recommended,
	},
	GroupE: {
		"solution": Recommended,
		"risk":     Recommended,
	},
	GroupF: {
		"classify": Required,
		"solution": Required,
		"plan":     Required,
		"task":     Required,
		"execute":  Required,
		"risk":     Required,
	},
	GroupG: {
		"task":    Recommended,
		"execute": Recommended,
	},
	GroupH: {
		"solution": Recommended,
		"plan":     Recommended,
		"task":     Recommended,
		"execute":  Recommended,
	},
}

func DefaultCatalog() []ContextSource {
	return []ContextSource{
		{
			Key:        "repo_conventions",
			Kind:       KindConventions,
			Transport:  TransportInternal,
			Adapter:    "repo_files",
			Trust:      TrustHigh,
			TTLSeconds: 3600,
			MaxBytes:   65536,
			Status:     StatusActive,
			EnabledFor: EnabledFor{
				Projects:     []string{"*"},
				RequestTypes: []string{"*"},
				Stages:       []string{"solution", "plan", "task", "execute", "risk"},
			},
		},
		{
			Key:        "repo_decisions",
			Kind:       KindDecisions,
			Transport:  TransportInternal,
			Adapter:    "repo_files",
			Trust:      TrustHigh,
			TTLSeconds: 3600,
			MaxBytes:   65536,
			Status:     StatusActive,
			EnabledFor: EnabledFor{
				Projects:     []string{"*"},
				RequestTypes: []string{"*"},
				Stages:       []string{"solution", "plan", "task", "execute", "risk"},
			},
		},
		{
			Key:        "repo_specs",
			Kind:       KindSpecs,
			Transport:  TransportInternal,
			Adapter:    "repo_files",
			Trust:      TrustHigh,
			TTLSeconds: 3600,
			MaxBytes:   65536,
			Status:     StatusActive,
			EnabledFor: EnabledFor{
				Projects:     []string{"*"},
				RequestTypes: []string{"*"},
				Stages:       []string{"solution", "plan", "task", "execute", "risk"},
			},
		},
		{
			Key:        "repo_contracts",
			Kind:       KindContracts,
			Transport:  TransportInternal,
			Adapter:    "repo_files",
			Trust:      TrustHigh,
			TTLSeconds: 3600,
			MaxBytes:   65536,
			Status:     StatusActive,
			EnabledFor: EnabledFor{
				Projects:     []string{"*"},
				RequestTypes: []string{"*"},
				Stages:       []string{"solution", "plan", "task", "execute"},
			},
		},
		{
			Key:        "repo_schema",
			Kind:       KindSchema,
			Transport:  TransportInternal,
			Adapter:    "repo_files",
			Trust:      TrustHigh,
			TTLSeconds: 3600,
			MaxBytes:   65536,
			Status:     StatusActive,
			EnabledFor: EnabledFor{
				Projects:     []string{"*"},
				RequestTypes: []string{"*"},
				Stages:       []string{"solution", "plan", "task", "execute"},
			},
		},
		{
			Key:        "repo_dependencies",
			Kind:       KindDependencies,
			Transport:  TransportInternal,
			Adapter:    "repo_files",
			Trust:      TrustHigh,
			TTLSeconds: 3600,
			MaxBytes:   65536,
			Status:     StatusActive,
			EnabledFor: EnabledFor{
				Projects:     []string{"*"},
				RequestTypes: []string{"*"},
				Stages:       []string{"plan", "task", "execute"},
			},
		},
		{
			Key:        "ci_config",
			Kind:       KindCIConfig,
			Transport:  TransportInternal,
			Adapter:    "repo_files",
			Trust:      TrustHigh,
			TTLSeconds: 3600,
			MaxBytes:   65536,
			Status:     StatusActive,
			EnabledFor: EnabledFor{
				Projects:     []string{"*"},
				RequestTypes: []string{"*"},
				Stages:       []string{"plan", "task", "execute"},
			},
		},
		{
			Key:        "policy_opa",
			Kind:       KindPolicy,
			Transport:  TransportInternal,
			Adapter:    "repo_files",
			Trust:      TrustHigh,
			TTLSeconds: 3600,
			MaxBytes:   65536,
			Status:     StatusActive,
			EnabledFor: EnabledFor{
				Projects:     []string{"*"},
				RequestTypes: []string{"*"},
				Stages:       []string{"plan", "task", "execute", "risk"},
			},
		},
		{
			Key:        "code_graph",
			Kind:       KindCodeGraph,
			Transport:  TransportInternal,
			Adapter:    "code_graph",
			Trust:      TrustMedium,
			TTLSeconds: 600,
			MaxBytes:   65536,
			Status:     StatusActive,
			EnabledFor: EnabledFor{
				Projects:     []string{"*"},
				RequestTypes: []string{"*"},
				Stages:       []string{"solution", "plan", "task", "execute"},
			},
		},
		{
			Key:        "git_history",
			Kind:       KindGitHistory,
			Transport:  TransportInternal,
			Adapter:    "git_history",
			Trust:      TrustHigh,
			TTLSeconds: 600,
			MaxBytes:   65536,
			Status:     StatusActive,
			EnabledFor: EnabledFor{
				Projects:     []string{"*"},
				RequestTypes: []string{"*"},
				Stages:       []string{"solution", "plan", "task", "execute"},
			},
		},
		{
			Key:        "request_origin",
			Kind:       KindRequestOrigin,
			Transport:  TransportInternal,
			Adapter:    "request_origin",
			Trust:      TrustHigh,
			TTLSeconds: 300,
			MaxBytes:   65536,
			Status:     StatusActive,
			EnabledFor: EnabledFor{
				Projects:     []string{"*"},
				RequestTypes: []string{"*"},
				Stages:       []string{"classify", "solution", "plan", "task", "execute", "risk"},
			},
		},
		{
			Key:        "history",
			Kind:       KindHistory,
			Transport:  TransportInternal,
			Adapter:    "history",
			Trust:      TrustMedium,
			TTLSeconds: 300,
			MaxBytes:   65536,
			Status:     StatusActive,
			EnabledFor: EnabledFor{
				Projects:     []string{"*"},
				RequestTypes: []string{"*"},
				Stages:       []string{"solution", "plan", "task", "execute"},
			},
		},
		{
			Key:        "ownership",
			Kind:       KindOwnership,
			Transport:  TransportInternal,
			Adapter:    "ownership",
			Trust:      TrustHigh,
			TTLSeconds: 3600,
			MaxBytes:   65536,
			Status:     StatusActive,
			EnabledFor: EnabledFor{
				Projects:     []string{"*"},
				RequestTypes: []string{"*"},
				Stages:       []string{"solution", "plan", "task", "execute", "risk"},
			},
		},
		{
			Key:        "dev_server_profile",
			Kind:       KindDevServerProfile,
			Transport:  TransportInternal,
			Adapter:    "dev_server_profile",
			Trust:      TrustHigh,
			TTLSeconds: 300,
			MaxBytes:   65536,
			Status:     StatusActive,
			EnabledFor: EnabledFor{
				Projects:     []string{"*"},
				RequestTypes: []string{"*"},
				Stages:       []string{"task", "execute"},
			},
		},
	}
}

func MergeCatalog(defaults, overrides []ContextSource) []ContextSource {
	m := make(map[string]ContextSource)
	for _, d := range defaults {
		m[d.Key] = d
	}
	for _, o := range overrides {
		m[o.Key] = o
	}
	res := make([]ContextSource, 0, len(m))
	// Maintain order from defaults where possible
	for _, d := range defaults {
		if o, ok := m[d.Key]; ok {
			res = append(res, o)
			delete(m, d.Key)
		}
	}
	// Append remaining overrides
	for _, o := range overrides {
		if _, ok := m[o.Key]; ok {
			res = append(res, o)
			delete(m, o.Key)
		}
	}
	return res
}
