package domain

type TaskSpec struct {
	ID         string
	TaskID     string
	SpecJSON   []byte
	Provenance string
	Locked     bool
}
