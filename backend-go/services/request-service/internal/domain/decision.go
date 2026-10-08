package domain

import (
	"strings"
	"time"
)

type DecisionStatus string

const (
	DecisionStatusOpen       DecisionStatus = "open"
	DecisionStatusChosen     DecisionStatus = "chosen"
	DecisionStatusEffective  DecisionStatus = "effective"
	DecisionStatusSuperseded DecisionStatus = "superseded"
)

type DecisionSubjectKind string

const (
	DecisionSubjectSolutionOption DecisionSubjectKind = "solution_option"
	DecisionSubjectPlanAssumption DecisionSubjectKind = "plan_assumption"
	DecisionSubjectOther          DecisionSubjectKind = "other"
)

type RiskLevel string

const (
	RiskNormal RiskLevel = "normal"
	RiskHigh   RiskLevel = "high"
)

type DecisionRiskInfo struct {
	Level   string   `json:"level,omitempty"`
	Reasons []string `json:"reasons,omitempty"`
}

type DecisionOption struct {
	ID      string           `json:"id"`
	Label   string           `json:"label"`
	Summary string           `json:"summary,omitempty"`
	Risk    DecisionRiskInfo `json:"risk,omitempty"`
}

type DecisionAction string

const (
	DecisionActionChosen     DecisionAction = "chosen"
	DecisionActionRechosen   DecisionAction = "rechosen"
	DecisionActionConfirmed  DecisionAction = "confirmed"
	DecisionActionSuperseded DecisionAction = "superseded"
)

// Decision records one confirmed choice (CR-REQ-028 section 2.8): who chose what and why,
// and, for risky options, a second typed confirmation before it takes effect.
type Decision struct {
	ID                   string
	TenantID             string
	RequestID            string
	Seq                  int
	SubjectKind          DecisionSubjectKind
	SubjectID            string
	SubjectDigest        string
	Question             string
	Options              []DecisionOption
	RecommendedOptionID  string
	RecommendationReason string
	ChosenOptionID       string
	ChooserID            string
	ChosenAt             *time.Time
	Rationale            string
	RiskLevel            RiskLevel
	ConfirmedBy          string
	ConfirmedAt          *time.Time
	Status               DecisionStatus
	Version              int64
	CreatedAt            time.Time
}

type DecisionHistory struct {
	ID         string
	TenantID   string
	DecisionID string
	Action     DecisionAction
	OptionID   string
	ActorID    string
	Rationale  string
	At         time.Time
}

// DisplayID is DEC-<reqnum>.<seq>.
func (d Decision) DisplayID(reqNum int64) string { return FormatDecisionID(reqNum, d.Seq) }

// IsLive is true while the decision still governs its subject.
func (d Decision) IsLive() bool { return d.Status != DecisionStatusSuperseded }

func (d Decision) option(id string) (DecisionOption, bool) {
	for _, o := range d.Options {
		if o.ID == id {
			return o, true
		}
	}
	return DecisionOption{}, false
}

// Choose records the choice and returns the history action. A normal-risk choice is effective at once;
// a high-risk one waits for Confirm. Choosing again clears any earlier confirmation.
func (d *Decision) Choose(optionID, chooserID, rationale string, risk RiskLevel, digest string, at time.Time) (DecisionAction, error) {
	if !d.IsLive() {
		return "", ErrDecisionStateInvalid(d.ID, d.Status)
	}
	if _, ok := d.option(optionID); !ok {
		return "", ErrClarificationInvalidAnswer("decision", "unknown option "+optionID)
	}
	rationale = strings.TrimSpace(NormalizeNFC(rationale))
	if d.RecommendedOptionID != "" && optionID != d.RecommendedOptionID && rationale == "" {
		return "", ErrDecisionRationaleRequired()
	}
	action := DecisionActionChosen
	if d.ChosenOptionID != "" {
		action = DecisionActionRechosen
	}
	d.ChosenOptionID, d.ChooserID, d.ChosenAt = optionID, chooserID, &at
	d.Rationale, d.RiskLevel, d.SubjectDigest = rationale, risk, digest
	d.ConfirmedBy, d.ConfirmedAt = "", nil
	if risk == RiskHigh {
		d.Status = DecisionStatusChosen
	} else {
		d.Status = DecisionStatusEffective
	}
	return action, nil
}

// NormalizeConfirmationText is how confirmation text and option titles are compared:
// NFC, trimmed, inner whitespace collapsed, case-insensitive.
func NormalizeConfirmationText(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(NormalizeNFC(s)), " "))
}

// Confirm makes a chosen decision effective when text names the chosen option.
func (d *Decision) Confirm(by, text string, at time.Time) error {
	if d.Status != DecisionStatusChosen {
		return ErrDecisionStateInvalid(d.ID, d.Status)
	}
	opt, ok := d.option(d.ChosenOptionID)
	if !ok || NormalizeConfirmationText(text) == "" || NormalizeConfirmationText(text) != NormalizeConfirmationText(opt.Label) {
		return ErrDecisionConfirmationMismatch()
	}
	d.Status, d.ConfirmedBy, d.ConfirmedAt = DecisionStatusEffective, by, &at
	return nil
}

func (d *Decision) Supersede() {
	d.Status = DecisionStatusSuperseded
}
