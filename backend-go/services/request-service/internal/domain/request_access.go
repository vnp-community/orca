package domain

// AccessInput is what request.rego decides on (the Rego input document).
type AccessInput struct {
	Action            Group
	RPC               string
	CallerGlobalRole  string
	CallerProjectRole string
	IsReporter        bool
	ActorType         string
}
