package domain

type RelatedSymbol struct {
	Ref  SymbolRef
	Kind EdgeKind
}

type RelatedSymbolList struct {
	Symbols []RelatedSymbol
}

type SymbolFlowRef struct {
	FlowID    string
	FlowLabel string
	Step      int32
}

type SymbolSource struct {
	Text      string
	StartLine int32
	EndLine   int32
	Truncated bool
}

// SymbolDetail provides complete single-symbol introspection data.
type SymbolDetail struct {
	Symbol        SymbolRef
	Incoming      map[string]RelatedSymbolList
	Outgoing      map[string]RelatedSymbolList
	Flows         []SymbolFlowRef
	Source        SymbolSource
	SourceOmitted string
}
