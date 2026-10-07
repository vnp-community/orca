package domain

type RequestRevision struct {
	ID         string
	RequestID  string
	Revision   int
	Content    []byte
	CreatedAt  int64
}
