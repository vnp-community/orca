package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// ContextSchema names the contract of the result block of a context run.
const ContextSchema = "outcome-review-context/1"

// ContextSchemaVersion is part of the design lock: a change of contract invalidates a locked design.
const ContextSchemaVersion = 1

type ContextMode string

const (
	ContextModeDesign    ContextMode = "design"
	ContextModeDelivered ContextMode = "delivered"
)

func (m ContextMode) Valid() bool { return m == ContextModeDesign || m == ContextModeDelivered }

// Limits of one result. Over any of them the result is refused and sent back once.
const (
	MaxContextItems        = 120
	MaxContextElements     = 400
	MaxContextContainers   = 40
	MaxContextDesignEdges  = 200
	MaxContextFlows        = 12
	MaxContextBlocks       = 8
	MaxContextBlockBytes   = 8 * 1024
	MaxContextDiagrams     = 16
	MaxContextNotesRunes   = 800
	MaxContextStringRunes  = 400
	MaxContextResultBytes  = 256 * 1024
	MaxContextDocs         = 24
	maxContextWarnings     = 40
	maxContextEvidencePerX = 8
)

// ContextResult is what the agent returns inside the nonce block. Only the form is trusted here; ValidateContext decides what
// of it Orca keeps. No state or verdict is read from it.
type ContextResult struct {
	Schema   string            `json:"schema"`
	KG       ContextKG         `json:"kg"`
	Items    []AgentReviewItem `json:"items"`
	Design   ContextDesign     `json:"design"`
	Diagrams []DiagramModel    `json:"diagrams"`
	Notes    string            `json:"notes"`
	Warnings []string          `json:"warnings"`
}

type ContextKG struct {
	Available bool     `json:"available"`
	Snapshot  string   `json:"snapshot"`
	Read      []string `json:"read"`
	Gaps      []string `json:"gaps"`
	Conflicts []string `json:"conflicts"`
}

// AgentReviewItem is a ReviewItem as an agent writes it. Subject keeps the key of an item stable between runs.
type AgentReviewItem struct {
	Subject       string           `json:"subject"`
	Section       ReviewSection    `json:"section"`
	Change        ReviewChange     `json:"change"`
	Title         string           `json:"title"`
	Detail        string           `json:"detail"`
	Severity      ReviewSeverity   `json:"severity"`
	NeedsDecision bool             `json:"needsDecision"`
	Containers    []string         `json:"containers"`
	Protocol      string           `json:"protocol"`
	Source        ComponentSource  `json:"source"`
	ReviewState   KGReviewState    `json:"reviewState"`
	Expected      *bool            `json:"expected"`
	Evidence      []ReviewEvidence `json:"evidence"`
	Before        string           `json:"before"`
	After         string           `json:"after"`
}

type ContextElement struct {
	ID          string        `json:"id"`
	Class       string        `json:"class"`
	ReviewState KGReviewState `json:"reviewState"`
}

type DesignContainer struct {
	ID          string           `json:"id"`
	KGID        *string          `json:"kgId"`
	Source      ComponentSource  `json:"source"`
	Label       string           `json:"label"`
	Kind        string           `json:"kind"`
	Tech        string           `json:"tech"`
	ReviewState KGReviewState    `json:"reviewState"`
	Aliases     []string         `json:"aliases"`
	Evidence    []ReviewEvidence `json:"evidence"`
	// Parent names the container this one is a part of; such a node is a component and is folded into its parent.
	Parent string `json:"parent"`
}

type SourceBlock struct {
	Doc     string `json:"doc"`
	Section string `json:"section"`
	Kind    string `json:"kind"` // mermaid|text
	Text    string `json:"text"`
}

type DesignEdge struct {
	From     string           `json:"from"`
	To       string           `json:"to"`
	Protocol string           `json:"protocol"`
	Label    string           `json:"label"`
	Source   ComponentSource  `json:"source"`
	Evidence []ReviewEvidence `json:"evidence"`
}

type DesignMessage struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Protocol string `json:"protocol"`
	Label    string `json:"label"`
	Order    int    `json:"order"`
}

type DesignFlow struct {
	Name     string           `json:"name"`
	Messages []DesignMessage  `json:"messages"`
	Evidence []ReviewEvidence `json:"evidence"`
}

type ContextDesign struct {
	Available    bool              `json:"available"`
	DocSha256    map[string]string `json:"docSha256"`
	ElementsRead []ContextElement  `json:"elementsRead"`
	Containers   []DesignContainer `json:"containers"`
	SourceBlocks []SourceBlock     `json:"sourceBlocks"`
	Edges        []DesignEdge      `json:"edges"`
	Flows        []DesignFlow      `json:"flows"`
}

// ContextViolations is a result that does not meet the contract. It carries every violation found, so the agent can fix them all
// in the one retry it gets.
type ContextViolations []string

func (v ContextViolations) Error() string {
	return fmt.Sprintf("%d violation(s): %s", len(v), strings.Join(v, "; "))
}

// AsContextViolations reports whether err is a violation list and returns it.
func AsContextViolations(err error) (ContextViolations, bool) {
	var v ContextViolations
	if errors.As(err, &v) {
		return v, true
	}
	return nil, false
}

// ParseContextResult decodes the block. Unknown members are ignored, which drops a state or verdict an agent might add;
// a block that is not one JSON object, is too large or names another schema is a violation.
func ParseContextResult(block []byte) (ContextResult, error) {
	if len(block) == 0 {
		return ContextResult{}, ContextViolations{"the result block is empty"}
	}
	if len(block) > MaxContextResultBytes {
		return ContextResult{}, ContextViolations{fmt.Sprintf("the result is %d bytes; the limit is %d", len(block), MaxContextResultBytes)}
	}
	dec := json.NewDecoder(bytes.NewReader(block))
	var r ContextResult
	if err := dec.Decode(&r); err != nil {
		return ContextResult{}, ContextViolations{"the result is not valid JSON of the contract: " + err.Error()}
	}
	if dec.More() {
		return ContextResult{}, ContextViolations{"the result has more than one JSON value"}
	}
	if r.Schema != ContextSchema {
		return ContextResult{}, ContextViolations{fmt.Sprintf("schema must be %q, not %q", ContextSchema, r.Schema)}
	}
	return r, nil
}

var sha256Hex = regexp.MustCompile(`^[0-9a-f]{64}$`)

// DesignDigest is the lock of an extracted design: the same snapshot, the same documents and the same prompt and contract give
// the same design, so it is read once and reused.
func DesignDigest(snapshot string, docSha256 map[string]string, promptVersion string) (string, error) {
	docs := make([]string, 0, len(docSha256))
	for d, s := range docSha256 {
		docs = append(docs, d+"="+s)
	}
	sort.Strings(docs)
	return DigestValue(struct {
		Snapshot string
		Docs     []string
		Prompt   string
		Schema   int
	}{snapshot, docs, promptVersion, ContextSchemaVersion})
}

// ContextDigest keys a whole context result: the facts it was built from, the locked design and the prompt.
func ContextDigest(mode ContextMode, inputsDigest, designDigest, promptVersion string) (string, error) {
	return DigestValue(struct {
		Mode, Inputs, Design, Prompt string
		Schema                       int
	}{string(mode), inputsDigest, designDigest, promptVersion, ContextSchemaVersion})
}
