package domain

type TaskSpecV2 struct {
	ID          string
	ExecutionID string
	Payload     []byte
}
