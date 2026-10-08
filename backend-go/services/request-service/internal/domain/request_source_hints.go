package domain

// SourceHints are hints taken from the originating issue; classification reads them
// and nothing else interprets them. JSON keys (issue_type, labels, priority, type_hint)
// are the storage contract of requests.source_hints.
type SourceHints struct {
	IssueType string   `json:"issue_type,omitempty"`
	Labels    []string `json:"labels,omitempty"`
	Priority  string   `json:"priority,omitempty"`
	TypeHint  string   `json:"type_hint,omitempty"`
}

func (h SourceHints) IsZero() bool {
	return h.IssueType == "" && len(h.Labels) == 0 && h.Priority == "" && h.TypeHint == ""
}
