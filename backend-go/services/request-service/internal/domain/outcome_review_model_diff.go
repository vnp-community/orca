package domain

import (
	"fmt"
	"sort"
)

// ModelDiff is what changed between two models of the same diagram: the boxes and the arrows that appear, disappear or change. It is
// computed, not asked of an agent, so comparing two revisions of a drawing costs nothing.
type ModelDiff struct {
	AddedNodes   []string `json:"addedNodes"`
	RemovedNodes []string `json:"removedNodes"`
	ChangedNodes []string `json:"changedNodes"`
	AddedEdges   []string `json:"addedEdges"`
	RemovedEdges []string `json:"removedEdges"`
	ChangedEdges []string `json:"changedEdges"`
}

// Empty says the two models are the same.
func (d ModelDiff) Empty() bool {
	return len(d.AddedNodes)+len(d.RemovedNodes)+len(d.ChangedNodes)+len(d.AddedEdges)+len(d.RemovedEdges)+len(d.ChangedEdges) == 0
}

func edgeIdentity(e DiagramEdge) string {
	return fmt.Sprintf("%s -> %s [%s] #%d", e.From, e.To, e.Protocol, e.Order)
}

// DiffModels compares from with to. Nodes are matched by id, edges by their ends, protocol and order; a node or an edge with the same
// identity but another state, label or kind is changed.
func DiffModels(from, to DiagramModel) ModelDiff {
	out := ModelDiff{AddedNodes: []string{}, RemovedNodes: []string{}, ChangedNodes: []string{}, AddedEdges: []string{}, RemovedEdges: []string{}, ChangedEdges: []string{}}
	oldN, newN := map[string]DiagramNode{}, map[string]DiagramNode{}
	for _, n := range from.Nodes {
		oldN[n.ID] = n
	}
	for _, n := range to.Nodes {
		newN[n.ID] = n
	}
	for id, n := range newN {
		o, ok := oldN[id]
		switch {
		case !ok:
			out.AddedNodes = append(out.AddedNodes, id)
		case o.State != n.State || o.Label != n.Label || o.Kind != n.Kind || o.Tech != n.Tech || o.ComponentsChanged != n.ComponentsChanged || len(o.Columns) != len(n.Columns):
			out.ChangedNodes = append(out.ChangedNodes, id)
		}
	}
	for id := range oldN {
		if _, ok := newN[id]; !ok {
			out.RemovedNodes = append(out.RemovedNodes, id)
		}
	}
	oldE, newE := map[string]DiagramEdge{}, map[string]DiagramEdge{}
	for _, e := range from.Edges {
		oldE[edgeIdentity(e)] = e
	}
	for _, e := range to.Edges {
		newE[edgeIdentity(e)] = e
	}
	for id, e := range newE {
		o, ok := oldE[id]
		switch {
		case !ok:
			out.AddedEdges = append(out.AddedEdges, id)
		case o.State != e.State || o.Label != e.Label || o.Advisory != e.Advisory || o.Unverified != e.Unverified:
			out.ChangedEdges = append(out.ChangedEdges, id)
		}
	}
	for id := range oldE {
		if _, ok := newE[id]; !ok {
			out.RemovedEdges = append(out.RemovedEdges, id)
		}
	}
	for _, l := range [][]string{out.AddedNodes, out.RemovedNodes, out.ChangedNodes, out.AddedEdges, out.RemovedEdges, out.ChangedEdges} {
		sort.Strings(l)
	}
	return out
}
