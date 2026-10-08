package domain

import "encoding/json"

// DefaultHighRiskServices is the proposed number of affected services that makes a choice high risk.
const DefaultHighRiskServices = 3

// RiskSignals are the facts DecisionRisk looks at, taken from one solution option.
type RiskSignals struct {
	BreakingChange  bool
	HighSeverity    bool
	AffectedService int
}

type DecisionRisk struct{}

// Assess is high when the option breaks compatibility, names a high-severity risk, or touches at least threshold services.
func (DecisionRisk) Assess(s RiskSignals, threshold int) RiskLevel {
	if threshold <= 0 {
		threshold = DefaultHighRiskServices
	}
	if s.BreakingChange || s.HighSeverity || s.AffectedService >= threshold {
		return RiskHigh
	}
	return RiskNormal
}

// RiskSignalsFromOption reads the signals out of one option object of a solution document.
// Both risks[].severity and the older risk.level count as a severity signal.
func RiskSignalsFromOption(raw json.RawMessage) (RiskSignals, error) {
	var o struct {
		BreakingChange bool `json:"breaking_change"`
		Risk           struct {
			Level string `json:"level"`
		} `json:"risk"`
		Risks []struct {
			Severity string `json:"severity"`
		} `json:"risks"`
		AffectedAreas []struct {
			Kind string `json:"kind"`
		} `json:"affected_areas"`
	}
	if err := json.Unmarshal(raw, &o); err != nil {
		return RiskSignals{}, ErrRequestContentInvalid("solution option: " + err.Error())
	}
	s := RiskSignals{BreakingChange: o.BreakingChange, HighSeverity: o.Risk.Level == "high"}
	for _, r := range o.Risks {
		if r.Severity == "high" {
			s.HighSeverity = true
		}
	}
	for _, a := range o.AffectedAreas {
		if a.Kind == "service" {
			s.AffectedService++
		}
	}
	return s, nil
}

// OptionIndexByID maps an option id such as opt-2 to its 0-based index in solutions.chosen_option (one place only).
func OptionIndexByID(opts []Option, id string) (int, bool) {
	for i, o := range opts {
		if o.ID == id {
			return i, true
		}
	}
	return 0, false
}
