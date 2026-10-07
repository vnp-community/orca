package domain

type RouteNode struct {
	ID           string
	Path         string
	Method       string
	FilePath     string
	Middleware   []string
	ResponseKeys []string
	ErrorKeys    []string
	Side         string
}

type RouteEdge struct {
	RouteID string
	Handler string
	Kind    string
}

// RouteMap captures HTTP/RPC endpoint mapping to backend handlers.
type RouteMap struct {
	Routes []RouteNode
	Edges  []RouteEdge
}
