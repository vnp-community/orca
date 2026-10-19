package contracttest

// withBaseRefColumns adds the requests column of migration 0099 (CR-REQ-039).
func withBaseRefColumns(m map[string][]Column) map[string][]Column {
	m["requests"] = append(m["requests"], Column{Name: "base_ref"})
	return m
}
