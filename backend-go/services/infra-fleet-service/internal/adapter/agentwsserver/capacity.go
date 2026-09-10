package agentwsserver

// SessionCounter is the narrow seam Server needs to enforce
// Cfg.MaxConcurrentSessions — implemented by devserveragent.Client's
// LiveSessionCount.
type SessionCounter interface {
	LiveSessionCount() int
}
