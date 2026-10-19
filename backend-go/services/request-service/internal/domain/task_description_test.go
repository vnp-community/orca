package domain

import "testing"

func TestTaskDescriptionLine_ReadsTheLineOfAnOpenSpecTask(t *testing.T) {
	d := "Serves: BRL-03\nCode: modify `src/round.go::Round`\n  Verify: AC-1 qua `go test`  \nDone when: kết quả đúng."
	if got := TaskDescriptionLine(d, "Verify:"); got != "AC-1 qua `go test`" {
		t.Fatalf("Verify = %q", got)
	}
	if got := TaskDescriptionLine(d, "Serves:"); got != "BRL-03" {
		t.Fatalf("Serves = %q", got)
	}
	if TaskDescriptionLine(d, "Owner:") != "" {
		t.Fatal("an absent line must read as empty")
	}
}

func TestHasVerifyStep_NeedsARealLine(t *testing.T) {
	cases := map[string]bool{
		"Verify: `go test`":                 true,
		"Serves: x\nVerify: manual check":   true,
		"Verify:":                           false,
		"Verify:   \nDone when: x":          false,
		"Serves: x":                         false,
		"":                                  false,
		"Please Verify: this in a sentence": false,
		"Done when: Verify: appears mid-line only": false,
	}
	for desc, want := range cases {
		if got := HasVerifyStep(desc); got != want {
			t.Errorf("HasVerifyStep(%q) = %v, want %v", desc, got, want)
		}
	}
}
