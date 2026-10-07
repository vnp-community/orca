package domain

import "time"

type ExecutionState struct {
	TaskID            string
	BlockedByTaskIDs  []string
	LastEngine        string
	LastLinkStatus    string
	LastStartedAt     time.Time
	LastCompletedAt   *time.Time
	FailedAttempts    int
}
