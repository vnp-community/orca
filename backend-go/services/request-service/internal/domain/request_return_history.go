package domain

import "time"

type ReturnAction string

const (
	ReturnActionReturned  ReturnAction = "returned"
	ReturnActionReopened  ReturnAction = "reopened"
	ReturnActionCancelled ReturnAction = "cancelled"
)

// ReturnHistoryEntry is one row of request_return_history; the requests table only keeps the latest return.
type ReturnHistoryEntry struct {
	ID        string
	RequestID string
	Action    ReturnAction
	Stage     ReturnStage
	Category  ReturnCategory
	Reason    string
	ActorID   string
	ActorKind ActorKind
	At        time.Time
}
