package domain

import "time"

type ActorKind string

const (
	ActorKindAI     ActorKind = "ai"
	ActorKindUser   ActorKind = "user"
	ActorKindSystem ActorKind = "system"
)

type RequestTypeChange struct {
	ID        string
	RequestID string
	FromType  RequestType // nullable equivalent, empty if not set
	ToType    RequestType
	ActorID   string
	ActorKind ActorKind
	Reason    string
	At        time.Time
}
