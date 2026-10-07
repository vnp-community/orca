package domain

type Snapshot struct {
	ID   string
	ETag string
	Data []byte
}
