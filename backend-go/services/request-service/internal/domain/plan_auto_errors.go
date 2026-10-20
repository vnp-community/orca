package domain

// ErrPlanAutoDisabled is what Retry returns while automatic Plan generation is off for the deployment.
func ErrPlanAutoDisabled() error {
	return planInvalid(PlanAutoCodeDisabled, "automatic plan generation is turned off")
}
