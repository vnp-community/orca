package usecase

import (
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// PlanAutoView is what a reader learns about the automatic Plan generation of a Request. State is empty when no job
// applies, so a screen can tell "nothing was tried" from "it gave up".
type PlanAutoView struct {
	// Enabled says whether Orca would start the Plan by itself for this Request's project.
	Enabled     bool
	State       string
	Origin      string
	Attempt     int
	MaxAttempts int
	ReasonCode  string
	// NextAttemptAt is set only while a pending job waits for its next try after a failed attempt or a busy deferral.
	NextAttemptAt time.Time
}

func planAutoViewOf(job domain.PlanAutoJob, enabled bool) PlanAutoView {
	v := PlanAutoView{Enabled: enabled, State: string(job.State), Origin: string(job.Origin), Attempt: job.Attempt, MaxAttempts: job.MaxAttempts, ReasonCode: job.ReasonCode}
	if job.State == domain.PlanAutoPending && (job.Attempt > 1 || job.ReasonCode != "") {
		v.NextAttemptAt = job.NextCheckAt
	}
	return v
}
