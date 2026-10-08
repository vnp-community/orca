package usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ClarificationNotice is the payload of orca.request.clarification.requested and .expired as
// notification-service reads it (specs/backend-go/crs/v6/approval/IMPLEMENTATION-NOTES.md, golden files
// copied into testdata/notification). Title and body are short and never carry question or answer text,
// because notifications are stored. The members after DeepLink are request-service's own and are ignored downstream.
type ClarificationNotice struct {
	ClarificationID string   `json:"clarification_id"`
	RequestID       string   `json:"request_id"`
	DisplayID       string   `json:"display_id"`
	Source          string   `json:"source"`
	DueAt           string   `json:"due_at"`
	Reminder        bool     `json:"reminder,omitempty"`
	Reason          string   `json:"reason,omitempty"`
	UserIDs         []string `json:"user_ids"`
	Title           string   `json:"title"`
	Body            string   `json:"body"`
	DeepLink        string   `json:"deep_link"`

	ClarificationDisplayID string `json:"clarification_display_id,omitempty"`
	ResumeStatus           string `json:"resume_status,omitempty"`
	Round                  int    `json:"round,omitempty"`
}

const (
	noticeTitleRequested = "Request cần bổ sung thông tin"
	noticeTitleReminder  = "Nhắc: cần bổ sung thông tin"
	noticeTitleExpired   = "Yêu cầu bổ sung thông tin đã hết hạn"
)

func noticeBase(r domain.Request, c domain.Clarification, users []string) ClarificationNotice {
	return ClarificationNotice{
		ClarificationID: c.ID, RequestID: r.ID, DisplayID: domain.FormatRequestID(r.Number), Source: string(c.Source),
		DueAt: c.DueAt.UTC().Format(time.RFC3339), UserIDs: nonNil(users),
		ClarificationDisplayID: c.DisplayID(r.Number), ResumeStatus: string(c.ResumeStatus), Round: c.Round,
	}
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func clarificationRequestedNotice(r domain.Request, c domain.Clarification, users []string, reminder bool) ClarificationNotice {
	n := noticeBase(r, c, users)
	n.DeepLink = "/?section=requests&request=" + r.ID + "&clarification=" + c.ID
	if reminder {
		n.Reminder, n.Reason = true, "reminder"
		n.Title, n.Body = noticeTitleReminder, n.DisplayID+" sắp hết hạn bổ sung thông tin"
		return n
	}
	n.Title, n.Body = noticeTitleRequested, n.DisplayID+" đang chờ bổ sung thông tin"
	return n
}

func clarificationExpiredNotice(r domain.Request, c domain.Clarification, users []string) ClarificationNotice {
	n := noticeBase(r, c, users)
	n.DeepLink = "/?section=requests&request=" + r.ID
	n.Title, n.Body = noticeTitleExpired, n.DisplayID+" đã trả về backlog do quá hạn"
	return n
}

// RecipientResolver expands assignees to user ids for notifications.
type RecipientResolver interface {
	Resolve(ctx context.Context, r domain.Request, assignees []domain.Principal) ([]string, error)
}

// PrincipalRecipients resolves users and the reporter locally, teams and the admin role through the directories when wired.
// A directory failure is logged and skipped: the clarification still exists and shows in the UI.
type PrincipalRecipients struct {
	Teams  TeamMembershipResolver
	Admins AdminDirectoryResolver
}

func (p PrincipalRecipients) Resolve(ctx context.Context, r domain.Request, assignees []domain.Principal) ([]string, error) {
	var out []string
	seen := map[string]bool{}
	add := func(ids ...string) {
		for _, id := range ids {
			if id != "" && !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
	}
	for _, a := range assignees {
		switch a.Kind {
		case domain.PrincipalKindUser:
			add(a.ID)
		case domain.PrincipalKindReporter:
			add(r.ReporterID)
		case domain.PrincipalKindTeam:
			if p.Teams == nil {
				continue
			}
			members, err := p.Teams.MembersOfTeam(ctx, a.ID)
			if err != nil {
				slog.WarnContext(ctx, "clarification recipients: team lookup failed", slog.String("team", a.ID), slog.Any("error", err))
				continue
			}
			add(members...)
		case domain.PrincipalKindRole:
			if a.ID != "admin" || p.Admins == nil {
				continue
			}
			tenantID, _ := tenantOf(ctx)
			admins, err := p.Admins.ListAdmins(ctx, tenantID)
			if err != nil {
				slog.WarnContext(ctx, "clarification recipients: admin lookup failed", slog.Any("error", err))
				continue
			}
			add(admins...)
		}
	}
	const maxRecipients = 50
	if len(out) > maxRecipients {
		out = out[:maxRecipients]
	}
	return out, nil
}
