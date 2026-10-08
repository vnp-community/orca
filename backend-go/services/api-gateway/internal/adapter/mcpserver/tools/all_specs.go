package tools

// AllSpecs returns every declared tool, all packs. The catalog keeps those of
// enabled packs; the parity test checks them all against the registry.
func AllSpecs() []*ToolSpec {
	var out []*ToolSpec
	for _, group := range [][]*ToolSpec{
		pack1Workspace(), pack1Git(), pack1SCM(), pack1Trackers(), pack1Operations(),
		pack2Workspace(), pack2Git(), pack2SCM(), pack2Trackers(),
		pack3Exec(), pack3TerminalAgent(), pack1WorkflowRunStatus(), pack4Admin(), packRequestFlow(),
	} {
		out = append(out, group...)
	}
	return out
}
