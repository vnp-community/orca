package usecase

import (
	"fmt"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func BuildOpenSpecPlanPrompt(in PlanInput, profile domain.OpenSpecProfile, changeID string) string {
	prompt := fmt.Sprintf(`Do not modify any files outside openspec/changes/%s/.
Please generate a plan with tasks for the following request:
Title: %s
Profile: %s
`, changeID, in.Request.Title, profile)

	return prompt
}
