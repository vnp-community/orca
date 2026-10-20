package domain

import (
	"fmt"
	"regexp"
	"strings"
)

// kgIDPattern accepts the ids of the knowledge graph: SVC-BUG-09, CT-02, FR-12, ENT-02:attribute:email.
var kgIDPattern = regexp.MustCompile(`^[A-Z]{2,6}(-[A-Z0-9]+)*-[0-9A-Za-z._:-]+$`)

var (
	rpcRefPattern     = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.]*(/[A-Za-z][A-Za-z0-9_]*)?$`)
	channelRefPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.]*$`)
	checkRefPattern   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,79}$`)
)

// ValidKGID reports whether id has the shape of a knowledge graph element id. It cannot say the element exists: Orca does not
// read the graph.
func ValidKGID(id string) bool { return len(id) <= 80 && kgIDPattern.MatchString(id) }

// EvidenceContext is what Orca knows to check an agent's evidence against. A nil set means the source is not known, and evidence
// that needs it is not verifiable.
type EvidenceContext struct {
	// KGRead are the element ids the agent declared it read.
	KGRead map[string]bool
	// Docs maps a document the agent declared it read to its sha256.
	Docs map[string]string
	// ChangedFiles are the paths the diff changed (empty in the design phase).
	ChangedFiles map[string]bool
	// BaseHasFile tells whether a path exists at the base ref; nil means it cannot be asked.
	BaseHasFile func(path string) bool
	// ContractChanged is true when the diff touched a contract file (a .proto or a gateway channel).
	ContractChanged bool
	CheckIDs        map[string]bool
	TaskIDs         map[string]bool
}

// EvidenceShapeError is evidence that is malformed, as opposed to well formed but not verifiable. A malformed result is sent
// back to the agent once; evidence that is merely unverifiable is kept and marked.
type EvidenceShapeError struct{ Reason string }

func (e EvidenceShapeError) Error() string { return e.Reason }

// CheckEvidenceShape looks only at the form of the evidence.
func CheckEvidenceShape(e ReviewEvidence) error {
	ref := strings.TrimSpace(e.Ref)
	bad := func(format string, a ...any) error { return EvidenceShapeError{Reason: fmt.Sprintf(format, a...)} }
	if !e.Kind.Valid() {
		return bad("evidence kind %q is not known", e.Kind)
	}
	if e.Kind != EvidenceSource && (ref == "" || len(ref) > 200) {
		return bad("%s evidence needs a reference of at most 200 characters", e.Kind)
	}
	switch e.Kind {
	case EvidenceKG:
		if !ValidKGID(ref) {
			return bad("kg evidence %q is not a knowledge graph id", ref)
		}
	case EvidenceSource:
		if e.Doc == "" || (e.Section == "" && e.Line <= 0) {
			return bad("source evidence needs a document and a section or line")
		}
	case EvidenceFile, EvidenceMigration:
		if normalizeRel(ref) == "" {
			return bad("%s evidence %q is not a relative path inside the repository", e.Kind, ref)
		}
	case EvidenceRPC:
		if !rpcRefPattern.MatchString(ref) {
			return bad("rpc evidence %q is not an rpc name", ref)
		}
	case EvidenceChannel:
		if !channelRefPattern.MatchString(ref) {
			return bad("channel evidence %q is not a channel name", ref)
		}
	case EvidenceCheck:
		if !checkRefPattern.MatchString(ref) {
			return bad("check evidence %q is not a check id", ref)
		}
	}
	return nil
}

// CheckEvidence returns nil when the evidence can be trusted as far as Orca can tell, or the reason it cannot. A malformed
// piece of evidence is an EvidenceShapeError.
func (c EvidenceContext) CheckEvidence(e ReviewEvidence) error {
	if err := CheckEvidenceShape(e); err != nil {
		return err
	}
	ref := strings.TrimSpace(e.Ref)
	switch e.Kind {
	case EvidenceKG:
		if !ValidKGID(ref) {
			return fmt.Errorf("kg evidence %q is not a knowledge graph id", ref)
		}
		if !c.KGRead[ref] {
			return fmt.Errorf("kg evidence %q is not among the elements the agent said it read", ref)
		}
	case EvidenceSource:
		if e.Doc == "" || (e.Section == "" && e.Line <= 0) {
			return fmt.Errorf("source evidence needs a document and a section or line")
		}
		if sum, ok := c.Docs[e.Doc]; !ok || sum == "" {
			return fmt.Errorf("source evidence %q is not a document the agent said it read", e.Doc)
		}
	case EvidenceFile:
		p := normalizeRel(ref)
		if p == "" {
			return fmt.Errorf("file evidence %q is not a relative path inside the repository", ref)
		}
		if !c.ChangedFiles[p] && !(c.BaseHasFile != nil && c.BaseHasFile(p)) {
			return fmt.Errorf("file evidence %q is neither changed nor present at the base", ref)
		}
	case EvidenceRPC:
		if !rpcRefPattern.MatchString(ref) {
			return fmt.Errorf("rpc evidence %q is not an rpc name", ref)
		}
		if !c.ContractChanged {
			return fmt.Errorf("rpc evidence %q but no contract file changed", ref)
		}
	case EvidenceChannel:
		if !channelRefPattern.MatchString(ref) {
			return fmt.Errorf("channel evidence %q is not a channel name", ref)
		}
		if !c.ContractChanged {
			return fmt.Errorf("channel evidence %q but no contract file changed", ref)
		}
	case EvidenceMigration:
		p := normalizeRel(ref)
		if p == "" || !isMigrationPath(p) || !c.ChangedFiles[p] {
			return fmt.Errorf("migration evidence %q is not a changed migration file", ref)
		}
	case EvidenceCheck:
		if !checkRefPattern.MatchString(ref) || !c.CheckIDs[ref] {
			return fmt.Errorf("check evidence %q is not a check of this Request", ref)
		}
	case EvidenceTask:
		if !c.TaskIDs[ref] {
			return fmt.Errorf("task evidence %q is not a task of this Request", ref)
		}
	default:
		return fmt.Errorf("evidence kind %q is not known", e.Kind)
	}
	return nil
}

// VerifiedEvidence keeps the evidence that passes and reports why the rest did not.
func (c EvidenceContext) VerifiedEvidence(in []ReviewEvidence) (kept []ReviewEvidence, problems []string) {
	for _, e := range in {
		if err := c.CheckEvidence(e); err != nil {
			problems = append(problems, err.Error())
			continue
		}
		kept = append(kept, e)
	}
	return kept, problems
}

// ContractPathChanged reports whether a changed path is a contract file: a .proto, or a gateway channel file.
func ContractPathChanged(paths []string) bool {
	for _, p := range paths {
		p = normalizeRel(p)
		if strings.HasSuffix(p, ".proto") || strings.Contains(p, "/wscompat/channels_") {
			return true
		}
	}
	return false
}
