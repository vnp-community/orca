package usecase

import (
	"context"
	"encoding/json"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type PublishApprovalNotifications struct {
	Expander *ExpandApprovalRecipients
	Publisher func(ctx context.Context, ev domain.OutboxEvent) error // Stub outbox publisher
}

func (uc *PublishApprovalNotifications) ProcessAndPublish(ctx context.Context, ev domain.OutboxEvent) error {
	var payload map[string]any
	if err := json.Unmarshal(ev.Payload, &payload); err != nil {
		return err // Should log and skip actually in relay
	}

	appID, _ := payload["approval_id"].(string)
	repID, _ := payload["reporter_id"].(string) // Assuming this was included
	selfAllowed, _ := payload["self_approval_allowed"].(bool) // Assuming included

	switch ev.Subject {
	case "orca.request.approval.requested":
		users, err := uc.Expander.Execute(ctx, appID, selfAllowed, repID)
		if err != nil {
			return err
		}
		if len(users) == 0 {
			// No notification
			return nil
		}
		payload["user_ids"] = users
		payload["title"] = "Approval needed"
		payload["body"] = "Approval required for request"
		payload["deep_link"] = "/?section=requests&request=req_id&approval=" + appID
	case "orca.request.approval.decided":
		// logic for decided
		decidedBy, _ := payload["decided_by"].(string)
		requestedBy, _ := payload["requested_by"].(string)
		
		users := []string{repID}
		if requestedBy != "system" {
			users = append(users, requestedBy)
		}

		var finalUsers []string
		for _, u := range users {
			if u != decidedBy && u != "" {
				finalUsers = append(finalUsers, u)
			}
		}

		if len(finalUsers) == 0 {
			return nil
		}

		payload["user_ids"] = finalUsers
		payload["title"] = "Approval decided"
		payload["body"] = "Decision: " + payload["decision"].(string)
		payload["deep_link"] = "/?section=requests&request=req_id&approval=" + appID
	default:
		return uc.Publisher(ctx, ev) // Pass through
	}

	newBytes, _ := json.Marshal(payload)
	ev.Payload = newBytes

	return uc.Publisher(ctx, ev)
}
