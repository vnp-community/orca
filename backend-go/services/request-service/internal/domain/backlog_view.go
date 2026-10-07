package domain

import "time"

type BacklogView int

const (
	BacklogViewUnspecified BacklogView = 0
	BacklogViewRequest     BacklogView = 1
	BacklogViewTask        BacklogView = 2
	BacklogViewExecute     BacklogView = 3
)

type BacklogRequestRow struct {
	Request          Request
	ReturnedBy       string
	ReturnedAt       time.Time
	ParentRequestIDs []string
}

type ReturnEvent struct {
	RequestID string
	ActorID   string
	At        time.Time
}
