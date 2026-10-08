package domain

// PolicyRegistry maps a Request type to its TypePolicy. It is built by a constructor (no init) so tests can swap policies.
type PolicyRegistry struct {
	policies map[RequestType]TypePolicy
}

func NewPolicyRegistry(deps PolicyDeps) *PolicyRegistry {
	return &PolicyRegistry{policies: map[RequestType]TypePolicy{
		RequestTypeHotfix:      hotfixPolicy{},
		RequestTypeSecurity:    securityPolicy{},
		RequestTypePerformance: performancePolicy{checks: deps.Checks},
		RequestTypeRefactor:    refactorPolicy{},
		RequestTypeOpsRequest:  opsRequestPolicy{approvals: deps.Approvals},
	}}
}

// WithPolicy returns a copy with one policy replaced; used by tests and by features that add a type.
func (r *PolicyRegistry) WithPolicy(t RequestType, p TypePolicy) *PolicyRegistry {
	out := &PolicyRegistry{policies: make(map[RequestType]TypePolicy, len(r.policies)+1)}
	for k, v := range r.policies {
		out.policies[k] = v
	}
	out.policies[t] = p
	return out
}

// PolicyFor never returns nil: an unknown or empty type gets the no-op policy.
func (r *PolicyRegistry) PolicyFor(t RequestType) TypePolicy {
	if r != nil {
		if p, ok := r.policies[t]; ok {
			return p
		}
	}
	return noopPolicy{}
}

// HasPolicy reports whether t has a policy of its own (not the no-op).
func (r *PolicyRegistry) HasPolicy(t RequestType) bool {
	if r == nil {
		return false
	}
	_, ok := r.policies[t]
	return ok
}
