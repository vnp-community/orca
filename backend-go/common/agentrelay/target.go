package agentrelay

import infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"

// Target says which dev server agent receives a call. A project whose dev server has no infra connection row yet is
// reached by DevServerID; every other case uses the connection.
type Target struct {
	DevServerID  string
	ConnectionID string
}

// byDevServer reports whether the call goes through RelayByDevServer. DevServerID wins over ConnectionID because a
// service only sets it when the connection id is not usable (no infra.connections row).
func (t Target) byDevServer() bool { return t.DevServerID != "" }

// relayRequest builds the Relay or RelayByDevServer message for method; exactly one of the two results is non-nil.
// An empty target is sent as is: infra-fleet answers with its own typed error, which callers already handle.
func (t Target) relayRequest(method string, paramsJSON []byte) (*infrafleetv1.RelayRequest, *infrafleetv1.RelayByDevServerRequest) {
	if t.byDevServer() {
		return nil, &infrafleetv1.RelayByDevServerRequest{DevServerId: t.DevServerID, Method: method, ParamsJson: string(paramsJSON)}
	}
	return &infrafleetv1.RelayRequest{ConnectionId: t.ConnectionID, Method: method, ParamsJson: string(paramsJSON)}, nil
}
