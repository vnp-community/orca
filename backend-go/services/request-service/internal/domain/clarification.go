package domain

const (
	ClarificationStatusPending   = "pending"
	ClarificationStatusAnswered  = "answered"
	ClarificationStatusCancelled = "cancelled"
)

type Clarification struct {
	ID        string
	RequestID string
	Question  string
	Answer    string
	Status    string
}
