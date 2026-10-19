package wscompat

import (
	"testing"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

func TestDecisionViewOf_NamesTheChosenOptionByItsLabel(t *testing.T) {
	v := decisionViewOf(&requestv1.Decision{Id: "d1", ChosenOptionId: "opt-2", RiskLevel: "high", Status: requestv1.DecisionStatus_DECISION_STATUS_CHOSEN,
		OptionsJson: `[{"id":"opt-1","label":"First"},{"id":"opt-2","label":"Second one"}]`})
	if v.ChosenOptionTitle != "Second one" || v.Status != "chosen" || v.RiskLevel != "high" {
		t.Fatalf("%+v", v)
	}
	if decisionViewOf(&requestv1.Decision{ChosenOptionId: "x", OptionsJson: "not json"}).ChosenOptionTitle != "" {
		t.Fatal("unreadable options leave the title empty instead of failing the view")
	}
}
