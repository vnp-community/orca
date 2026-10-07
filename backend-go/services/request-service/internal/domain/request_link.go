package domain

import "time"

type LinkReason string

const (
	LinkReasonRelatesTo   LinkReason = "relates_to"
	LinkReasonBlocks      LinkReason = "blocks"
	LinkReasonIsBlockedBy LinkReason = "is_blocked_by"
	LinkReasonDuplicates  LinkReason = "duplicates"
)

type RequestLink struct {
	ParentRequestID string
	ChildRequestID  string
	Reason          LinkReason
	CreatedBy       string
	CreatedAt       time.Time
}

func NewRequestLink(parentID, childID string, reason LinkReason, createdBy string) (RequestLink, error) {
	if parentID == childID {
		return RequestLink{}, ErrRequestLinkSelf()
	}
	return RequestLink{
		ParentRequestID: parentID,
		ChildRequestID:  childID,
		Reason:          reason,
		CreatedBy:       createdBy,
		CreatedAt:       time.Now().UTC(),
	}, nil
}
