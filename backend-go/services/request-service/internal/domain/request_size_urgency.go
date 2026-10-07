package domain

type RequestSize string

const (
	RequestSizeS RequestSize = "S"
	RequestSizeM RequestSize = "M"
	RequestSizeL RequestSize = "L"
)

func ParseSize(s string) (RequestSize, error) {
	if s == "S" || s == "M" || s == "L" {
		return RequestSize(s), nil
	}
	return "", ErrRequestInvalidSize(s)
}

type Urgency string

const (
	UrgencyNormal Urgency = "normal"
	UrgencyUrgent Urgency = "urgent"
)

func ParseUrgency(s string) (Urgency, error) {
	if s == "normal" || s == "urgent" {
		return Urgency(s), nil
	}
	return "", ErrRequestInvalidUrgency(s)
}

type ReturnStage string

const (
	ReturnStageClassification ReturnStage = "classification"
	ReturnStageAnalysis       ReturnStage = "analysis"
	ReturnStagePlan           ReturnStage = "plan"
	ReturnStagePhase          ReturnStage = "phase"
	ReturnStageTask           ReturnStage = "task"
)

type TypeSource string

const (
	TypeSourceAI    TypeSource = "ai"
	TypeSourceHuman TypeSource = "human"
)
