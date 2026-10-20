package usecase

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// plantedSecrets are made-up values of every kind secretscan knows, built from parts so no scanner flags this file.
func plantedSecrets() map[string]string {
	return map[string]string{
		"github token":      "gh" + "p_" + strings.Repeat("a1B2", 8),
		"aws access key":    "AK" + "IA" + "ABCDEFGHIJKLMNOP",
		"bearer token":      "Bearer " + "abcdef0123456789xyz",
		"anthropic key":     "sk-" + "ant-" + strings.Repeat("q9Z_", 8),
		"openai key":        "sk-" + strings.Repeat("Ab3", 8),
		"slack token":       "xox" + "b-" + "1234567890-abcdefghij",
		"jwt":               "ey" + "JhbGciOiJIUzI1" + "." + "eyJzdWIiOiIxMjM0" + "." + "abcDEF123456ghi",
		"google api key":    "AI" + "za" + strings.Repeat("A1b2C3d4E5", 3) + "xyzab",
		"vault token":       "hv" + "s." + strings.Repeat("kT7_", 7),
		"connection string": "postgres://app:" + "hunter2hunter2" + "@db.internal/orca",
		"dotenv secret":     "STRIPE_SECRET_KEY=" + "sk_live_" + "notAFakeNoMore99",
		"assignment":        "password: " + "correct-horse-battery",
		"private key":       "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA\n-----END RSA PRIVATE KEY-----",
	}
}

func TestRedactBundle_MasksEveryKindInEveryDocument(t *testing.T) {
	for name, secret := range plantedSecrets() {
		for _, path := range []string{"design.md", "tasks.md", "specs/ai/spec.md"} {
			t.Run(name+" in "+path, func(t *testing.T) {
				files := []bundleFile{{Path: path, Content: []byte("line before\n" + secret + "\nline after\n")}, {Path: "proposal.md", Content: []byte("clean\n")}}
				bundle, n := redactBundle(files, maxBundleBytes)
				if n < 1 {
					t.Fatalf("no secret was counted")
				}
				if strings.Contains(bundle[path], strings.Fields(secret)[len(strings.Fields(secret))-1]) {
					t.Fatalf("the secret survived: %q", bundle[path])
				}
				if !strings.Contains(bundle[path], "[REDACTED]") || !strings.Contains(bundle[path], "line before") || !strings.Contains(bundle[path], "line after") {
					t.Fatalf("surrounding text must stay: %q", bundle[path])
				}
				if bundle["proposal.md"] != "clean\n" {
					t.Fatalf("a clean file must stay as it was: %q", bundle["proposal.md"])
				}
			})
		}
	}
}

func TestRedactBundle_SecretInThePlanRegionOfTasksMd(t *testing.T) {
	tasks := strings.Replace(syncTasksMd, "Done when: one", "Done when: token = "+"abcd1234efgh5678", 1)
	bundle, n := redactBundle([]bundleFile{{Path: "tasks.md", Content: []byte(tasks)}}, maxBundleBytes)
	region, ok := domain.PlanRegionText(bundle["tasks.md"])
	if n != 1 || !ok || !strings.Contains(region, "[REDACTED]") || strings.Contains(region, "abcd1234efgh5678") {
		t.Fatalf("n=%d ok=%v region=%q", n, ok, region)
	}
}

func TestRedactBundle_CountsManySecretsAndScansTheTailOfALargeFile(t *testing.T) {
	secrets := plantedSecrets()
	body := strings.Repeat("filler line of a design document\n", (MaxProposalReadBytes-2048)/33)
	tail := secrets["github token"] + "\n" + secrets["aws access key"] + "\n" + secrets["anthropic key"]
	content := body + tail
	if len(content) > MaxProposalReadBytes {
		t.Fatalf("fixture too large: %d", len(content))
	}
	bundle, n := redactBundle([]bundleFile{{Path: "design.md", Content: []byte(content)}}, maxBundleBytes)
	if n != 3 {
		t.Fatalf("want 3 secrets at the tail, counted %d", n)
	}
	for _, s := range []string{secrets["github token"], secrets["aws access key"], secrets["anthropic key"]} {
		if strings.Contains(bundle["design.md"], s) {
			t.Fatalf("a secret at the tail survived")
		}
	}
}

func TestRedactBundle_OmittedFileKeepsTheRealSizeAndHash(t *testing.T) {
	big := []byte(strings.Repeat("x", 300))
	small := []byte("kept\n")
	bundle, n := redactBundle([]bundleFile{{Path: "big.md", Content: big}, {Path: "small.md", Content: small}}, 100)
	size, sum, ok := domain.ParseOmittedMarker(bundle["big.md"])
	if !ok || size != 300 || len(sum) != 64 || n != 0 || bundle["small.md"] != "kept\n" {
		t.Fatalf("marker=%q ok=%v size=%d n=%d", bundle["big.md"], ok, size, n)
	}
}

func TestRedactBundle_AnOmittedFileDoesNotCountItsSecrets(t *testing.T) {
	big := []byte(plantedSecrets()["github token"] + "\n" + strings.Repeat("x", 300))
	bundle, n := redactBundle([]bundleFile{{Path: "big.md", Content: big}}, 100)
	if n != 0 || !isOmittedMarker(bundle["big.md"]) {
		t.Fatalf("n=%d %q", n, bundle["big.md"])
	}
}

func TestRedactBundle_DrawsTheBudgetFromTheMaskedText(t *testing.T) {
	files := []bundleFile{{Path: "a.md", Content: []byte(strings.Repeat("a", 60))}, {Path: "b.md", Content: []byte(strings.Repeat("b", 60))}}
	bundle, _ := redactBundle(files, 100)
	if bundle["a.md"] == "" || isOmittedMarker(bundle["a.md"]) || !isOmittedMarker(bundle["b.md"]) {
		t.Fatalf("a=%q b=%q", bundle["a.md"], bundle["b.md"])
	}
}

// --- through the proposal run and the readers ---

func TestFinish_StoresRedactedBundle_AndCount(t *testing.T) {
	eng := &scriptedEngine{answers: []ProposalResult{answer(tasksDoc(twoPhaseBody))}}
	k, run, w := proposalKit(t, eng, nil)
	secret := plantedSecrets()["github token"]
	eng.afterWrite = func(dir string) {
		k.proposal.ws.write(dir+"design.md", "## Context\nthe deploy uses "+secret+" today\n")
	}
	if err := w.Execute(k.w.ctx(), run); err != nil {
		t.Fatal(err)
	}
	change, _, _ := k.proposal.changes.GetByRequest(context.Background(), run.RequestID)
	if change.RedactedSecrets != 1 {
		t.Fatalf("RedactedSecrets = %d", change.RedactedSecrets)
	}
	if got := change.Bundle["design.md"]; strings.Contains(got, secret) || !strings.Contains(got, "[REDACTED]") {
		t.Fatalf("the stored design.md must be masked: %q", got)
	}
	if !strings.Contains(string(k.proposal.ws.files[dirOfChange(change)+"design.md"]), secret) {
		t.Fatal("the file on the branch must stay raw")
	}
}

func dirOfChange(c domain.OpenSpecChange) string { return "openspec/changes/" + c.ChangeID + "/" }

func TestFinish_NoSecret_CountsZeroAndAddsNoCheck(t *testing.T) {
	eng := &scriptedEngine{answers: []ProposalResult{answer(tasksDoc(twoPhaseBody))}}
	k, run, w := proposalKit(t, eng, nil)
	if err := w.Execute(k.w.ctx(), run); err != nil {
		t.Fatal(err)
	}
	change, _, _ := k.proposal.changes.GetByRequest(context.Background(), run.RequestID)
	if change.RedactedSecrets != 0 {
		t.Fatalf("RedactedSecrets = %d", change.RedactedSecrets)
	}
	for _, c := range planChecksFor(change) {
		if c.Code == "SECRETS_REDACTED" {
			t.Fatal("no warning without a masked secret")
		}
	}
}

func TestPlanChecks_SecretsRedacted(t *testing.T) {
	checks := planChecksFor(domain.OpenSpecChange{RedactedSecrets: 3, KGAvailable: true, KGReadIDs: []string{"FR-1"}, SkillHash: "h"})
	var found *PlanCheckView
	for i := range checks {
		if checks[i].Code == "SECRETS_REDACTED" {
			found = &checks[i]
		}
	}
	if found == nil || found.Severity != "info" || found.Message != "3 secrets were masked in the stored copy; the file on the proposal branch is unchanged" {
		t.Fatalf("%+v", found)
	}
}

func TestGetPlanBundleFile_RedactsLegacyRow(t *testing.T) {
	secret := plantedSecrets()["aws access key"]
	w := newSolWorld()
	req := w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusPlanning)
	changes := newFakeChanges()
	if _, err := changes.Upsert(context.Background(), domain.OpenSpecChange{ID: "c", RequestID: req.ID, ChangeID: "req-1-x", Bundle: map[string]string{
		"design.md": "key " + secret + " end", "big.md": fmt.Sprintf("<omitted size=%d sha256=%s>", 5000000, strings.Repeat("0", 64)),
	}}); err != nil {
		t.Fatal(err)
	}
	uc := NewGetPlanBundleFile(w, changes)
	got, err := uc.Execute(w.ctx(), req.ID, "design.md", 0)
	if err != nil || strings.Contains(got.Content, secret) || !strings.Contains(got.Content, "[REDACTED]") || got.Omitted {
		t.Fatalf("%+v %v", got, err)
	}
	omitted, err := uc.Execute(w.ctx(), req.ID, "big.md", 0)
	if err != nil || !omitted.Omitted || !strings.HasPrefix(omitted.Content, "<omitted size=5000000 ") {
		t.Fatalf("an omitted marker is returned as it is: %+v %v", omitted, err)
	}
}

func TestGetPlanProposal_OmittedFileListsItsRealSize(t *testing.T) {
	c := domain.OpenSpecChange{ChangeID: "req-1-x", Bundle: map[string]string{
		"big.md":   fmt.Sprintf("<omitted size=%d sha256=%s>", 3145728, strings.Repeat("a", 64)),
		"small.md": "12345",
	}}
	changes := newFakeChanges()
	changes.byReq["r"] = c
	uc := NewGetPlanProposal(nil, nil).WithChanges(changes)
	view := &PlanProposalView{}
	if err := uc.addChange(context.Background(), view, "r"); err != nil {
		t.Fatal(err)
	}
	sizes := map[string]int64{}
	omitted := map[string]bool{}
	for _, f := range view.Bundle.Files {
		sizes[f.Path], omitted[f.Path] = f.Size, f.Omitted
	}
	if sizes["big.md"] != 3145728 || !omitted["big.md"] || sizes["small.md"] != 5 || omitted["small.md"] {
		t.Fatalf("sizes=%v omitted=%v", sizes, omitted)
	}
}
