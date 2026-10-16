package contracttest

// withExecutionModeColumns adds the requests column of migration 0098 (CR-REQ-038); the other two columns it adds belong to
// tables this check does not list.
func withExecutionModeColumns(m map[string][]Column) map[string][]Column {
	m["requests"] = append(m["requests"], Column{Name: "execution_mode", Nullable: true})
	return m
}
