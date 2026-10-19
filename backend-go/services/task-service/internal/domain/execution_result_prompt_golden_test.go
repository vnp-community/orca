package domain

import (
	"strconv"
	"testing"
)

func TestPromptGolden_ContractPromptSuffix(t *testing.T) {
	suffix := func(nonce string, requirePassingCheck, knowledgeHub bool) string {
		t.Helper()
		got, err := ContractPromptSuffix(nonce, requirePassingCheck, knowledgeHub)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	rendered := map[string]string{}
	for _, requirePassingCheck := range []bool{false, true} {
		for _, knowledgeHub := range []bool{false, true} {
			name := "require_check_" + strconv.FormatBool(requirePassingCheck) + "_knowledge_hub_" + strconv.FormatBool(knowledgeHub)
			rendered[name] = suffix("nonce0123456789abcdef", requirePassingCheck, knowledgeHub)
		}
	}
	rendered["empty_nonce"] = suffix("", true, true)
	checkPromptGoldens(t, "ContractPromptSuffix", rendered)
}
