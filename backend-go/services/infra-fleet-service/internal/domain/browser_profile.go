package domain

import "time"

// BrowserProfile is tenant/dev-server-scoped browser profile metadata — a
// profile's actual browser-data directory (cookies, local storage, etc.)
// lives on the dev server's filesystem, never in this struct or this
// service's database. See SOL-006 Group C.
type BrowserProfile struct {
	ID            string
	TenantID      string
	DevServerID   string
	Name          string
	SourceBrowser string // e.g. "chrome", "firefox" — empty if manually created
	IsDefault     bool
	CreatedAt     time.Time
}
