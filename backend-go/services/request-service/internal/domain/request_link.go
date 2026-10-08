package domain

import "time"

type LinkReason string

const (
	LinkReasonRelatesTo   LinkReason = "relates_to"
	LinkReasonBlocks      LinkReason = "blocks"
	LinkReasonIsBlockedBy LinkReason = "is_blocked_by"
	LinkReasonDuplicates  LinkReason = "duplicates"

	// Child-request reasons (CR-REQ-006): the link records why the child was spawned.
	LinkReasonSpawnedBySpike    LinkReason = "spawned_by_spike"
	LinkReasonSpawnedByQuestion LinkReason = "spawned_by_question"
	LinkReasonFollowupHotfix    LinkReason = "followup_hotfix"
	LinkReasonEscalation        LinkReason = "escalation"
)

func ParseLinkReason(s string) (LinkReason, error) {
	switch LinkReason(s) {
	case LinkReasonRelatesTo, LinkReasonBlocks, LinkReasonIsBlockedBy, LinkReasonDuplicates,
		LinkReasonSpawnedBySpike, LinkReasonSpawnedByQuestion, LinkReasonFollowupHotfix, LinkReasonEscalation:
		return LinkReason(s), nil
	}
	return "", ErrChildNotAllowed("unknown link reason " + s)
}

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
