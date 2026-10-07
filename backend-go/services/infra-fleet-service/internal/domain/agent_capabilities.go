package domain

// AgentCapabilities represents the capabilities and tools advertised by a connected agent.
// Derived entirely from handshake in-memory cache without dialing the remote host.
type AgentCapabilities struct {
	Connected    bool
	Platform     string
	Arch         string
	NodeVersion  string
	AgentVersion string
	Capabilities []string
	Tools        []string
	SessionID    string
}
