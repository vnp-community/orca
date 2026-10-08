package domain

// RequestStatusEvent mirrors the JSON payload request-service publishes to
// orca.request.request.status_changed and orca.request.request.completed.
// Completed is stamped by the subscriber from the subject, like
// WorktreeLifecycleEvent.Deleted.
type RequestStatusEvent struct {
	EventID   string
	TenantID  string
	RequestID string
	ProjectID string
	From      string
	To        string
	Trigger   string
	Type      string
	ActorID   string
	// ActorKind is "user", "ai" or "system"; only a user's own credential is used for Jira.
	ActorKind      string
	SourceProvider string
	SourceSite     string
	SourceRef      string
	// ReporterID is the fallback Jira credential owner when the actor is not a user.
	ReporterID string
	Number     int64
	// Version orders status changes of one Request; stale versions are not applied.
	Version   int64
	Completed bool
	// TraceParent is a W3C traceparent used only to link spans.
	TraceParent string
}
