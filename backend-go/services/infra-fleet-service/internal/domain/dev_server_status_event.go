package domain

// DevServerStatusEvent is a snapshot of one dev server, not a delta: a consumer that misses or reorders events is
// right again at the next one, and a repeated event changes nothing.
type DevServerStatusEvent struct {
	DevServerID      string
	Connected        bool
	ActiveExecutions int
}
