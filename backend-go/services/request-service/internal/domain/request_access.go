package domain

type AccessInput struct {
	Method      string
	Entry       Entry
	ProjectRole string
	GlobalRole  string
	IsReporter  bool
	ActorType   string
}

type AccessDecision struct {
	Allowed bool
	Reason  string
}
