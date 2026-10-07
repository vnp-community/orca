package domain

type RiskScore struct {
	ImpactLevel string
	Probability float64
	Score       int
}

func CalculateRiskScore() RiskScore {
	return RiskScore{Score: 50}
}
