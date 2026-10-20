package domain

// ReviewPhase says what an Outcome Review can show. design exists once a Request has a Plan and reads no diff; delivered
// is made when Execution is over and adds what the code changed.
type ReviewPhase string

const (
	ReviewPhaseDesign    ReviewPhase = "design"
	ReviewPhaseDelivered ReviewPhase = "delivered"
)

// ReviewDesignHeadKey is the head key of a design review: it has no commit, so it is one row per Request.
const ReviewDesignHeadKey = "design"

// ReviewNoCodeHeadKey is the head key of a delivered review whose tasks left no worktree to read.
const ReviewNoCodeHeadKey = "no-code"

func (p ReviewPhase) Valid() bool { return p == ReviewPhaseDesign || p == ReviewPhaseDelivered }

type ReviewState string

const (
	ReviewStateDraft      ReviewState = "draft"
	ReviewStateReady      ReviewState = "ready"
	ReviewStateStale      ReviewState = "stale"
	ReviewStateApproved   ReviewState = "approved"
	ReviewStateSuperseded ReviewState = "superseded"
)

func (s ReviewState) Valid() bool {
	switch s {
	case ReviewStateDraft, ReviewStateReady, ReviewStateStale, ReviewStateApproved, ReviewStateSuperseded:
		return true
	}
	return false
}

type ReviewSection string

const (
	SectionArchitecture ReviewSection = "architecture"
	SectionFlow         ReviewSection = "flow"
	SectionBehavior     ReviewSection = "behavior"
	SectionData         ReviewSection = "data"
	SectionContracts    ReviewSection = "contracts"
	SectionRequirements ReviewSection = "requirements"
	SectionConfig       ReviewSection = "config"
)

func AllReviewSections() []ReviewSection {
	return []ReviewSection{SectionArchitecture, SectionFlow, SectionBehavior, SectionData, SectionContracts, SectionRequirements, SectionConfig}
}

func (s ReviewSection) Valid() bool {
	for _, v := range AllReviewSections() {
		if v == s {
			return true
		}
	}
	return false
}

// Drawn reports whether the section has a diagram; the others are tables only.
func (s ReviewSection) Drawn() bool {
	switch s {
	case SectionArchitecture, SectionFlow, SectionBehavior, SectionData:
		return true
	}
	return false
}

type ReviewChange string

const (
	ChangeAdded     ReviewChange = "added"
	ChangeModified  ReviewChange = "modified"
	ChangeRemoved   ReviewChange = "removed"
	ChangeUnchanged ReviewChange = "unchanged"
	ChangeMissing   ReviewChange = "missing"
)

// ReviewOrigin says who stands behind an item. Only fact items count towards a verdict Orca computes itself.
type ReviewOrigin string

const (
	OriginFact  ReviewOrigin = "fact"
	OriginAgent ReviewOrigin = "agent"
)

type ReviewSeverity string

const (
	SeverityInfo       ReviewSeverity = "info"
	SeverityWarning    ReviewSeverity = "warning"
	SeverityBreaking   ReviewSeverity = "breaking"
	SeverityDeviation  ReviewSeverity = "deviation"
	SeverityRegression ReviewSeverity = "regression"
)

func (s ReviewSeverity) rank() int {
	switch s {
	case SeverityRegression:
		return 5
	case SeverityBreaking:
		return 4
	case SeverityDeviation:
		return 3
	case SeverityWarning:
		return 2
	}
	return 1
}

// ComponentSource says where a component or edge of the design was found. Only code means "not in the design".
type ComponentSource string

const (
	SourceKG      ComponentSource = "kg"
	SourceSADText ComponentSource = "sad-text"
	SourceCode    ComponentSource = "code"
)

// KGReviewState is the review state of a knowledge graph element used as the basis of a statement.
type KGReviewState string

const (
	KGApproved    KGReviewState = "approved"
	KGNeedsReview KGReviewState = "needs_review"
	KGUnknown     KGReviewState = "unknown"
)

type EvidenceKind string

const (
	EvidenceKG        EvidenceKind = "kg"
	EvidenceSource    EvidenceKind = "source"
	EvidenceFile      EvidenceKind = "file"
	EvidenceRPC       EvidenceKind = "rpc"
	EvidenceChannel   EvidenceKind = "channel"
	EvidenceMigration EvidenceKind = "migration"
	EvidenceCheck     EvidenceKind = "check"
	EvidenceTask      EvidenceKind = "task"
)

func (k EvidenceKind) Valid() bool {
	switch k {
	case EvidenceKG, EvidenceSource, EvidenceFile, EvidenceRPC, EvidenceChannel, EvidenceMigration, EvidenceCheck, EvidenceTask:
		return true
	}
	return false
}

type ReviewEvidence struct {
	Kind     EvidenceKind `json:"kind"`
	Ref      string       `json:"ref"`
	Snapshot string       `json:"snapshot,omitempty"`
	Doc      string       `json:"doc,omitempty"`
	Section  string       `json:"section,omitempty"`
	Line     int          `json:"line,omitempty"`
}

// ReviewItem is one row of a review tab. Key is stable between builds so a decision outlives a regeneration.
type ReviewItem struct {
	Key           string         `json:"key"`
	Section       ReviewSection  `json:"section"`
	Change        ReviewChange   `json:"change"`
	Origin        ReviewOrigin   `json:"origin"`
	Title         string         `json:"title"`
	Detail        string         `json:"detail,omitempty"`
	Severity      ReviewSeverity `json:"severity"`
	NeedsDecision bool           `json:"needsDecision"`
	// Advisory marks a finding that rests on elements nobody approved; it never needs a decision.
	Advisory   bool            `json:"advisory,omitempty"`
	Unverified bool            `json:"unverified,omitempty"`
	Containers []string        `json:"containers,omitempty"`
	Protocol   string          `json:"protocol,omitempty"`
	Source     ComponentSource `json:"source,omitempty"`
	ReviewSt   KGReviewState   `json:"reviewState,omitempty"`
	// Expected is nil when there is no design to compare with.
	Expected *bool            `json:"expected,omitempty"`
	Evidence []ReviewEvidence `json:"evidence,omitempty"`
	Before   string           `json:"before,omitempty"`
	After    string           `json:"after,omitempty"`
}

type RequirementsVerdict struct {
	Delivered  int `json:"delivered"`
	Partial    int `json:"partial"`
	NotCovered int `json:"notCovered"`
	Unknown    int `json:"unknown"`
	Planned    int `json:"planned"`
	Total      int `json:"total"`
}

// DesignVerdict is the count the Architecture and Flow cards show. It is computed from a validated model but the model itself is
// what an agent reported, so the numbers carry the same label.
type DesignVerdict struct {
	ContainersChanged  int     `json:"containersChanged,omitempty"`
	NewEdges           int     `json:"newEdges,omitempty"`
	RemovedEdges       int     `json:"removedEdges,omitempty"`
	UnplannedEdges     int     `json:"unplannedEdges,omitempty"`
	NewCalls           int     `json:"newCalls,omitempty"`
	RemovedCalls       int     `json:"removedCalls,omitempty"`
	Reordered          int     `json:"reordered,omitempty"`
	UnplannedCalls     int     `json:"unplannedCalls,omitempty"`
	ReportedDeviations int     `json:"reportedDeviations"`
	WithEvidence       int     `json:"withEvidence"`
	Advisory           int     `json:"advisory"`
	DesignChecked      bool    `json:"designChecked"`
	ReviewedRatio      float64 `json:"reviewedRatio"`
}

type QualityVerdict struct {
	Gate string `json:"gate"` // pass|warn|fail|unknown|absent
}

type ReleaseVerdict struct {
	Risk  string   `json:"risk"` // low|medium|high|critical|unknown
	Flags []string `json:"flags"`
}

type Verdict struct {
	Requirements RequirementsVerdict `json:"requirements"`
	Architecture DesignVerdict       `json:"architecture"`
	Flow         DesignVerdict       `json:"flow"`
	Quality      QualityVerdict      `json:"quality"`
	Release      ReleaseVerdict      `json:"release"`
}

type ReviewHead struct {
	Branch string `json:"branch"`
	Commit string `json:"commit"`
	TaskID string `json:"taskId,omitempty"`
}

type ReviewSubject struct {
	BaseRef       string       `json:"baseRef"`
	BaseCommit    string       `json:"baseCommit,omitempty"`
	HeadCommit    string       `json:"headCommit,omitempty"`
	Heads         []ReviewHead `json:"heads,omitempty"`
	Engine        string       `json:"engine,omitempty"`
	ExecutionMode string       `json:"executionMode,omitempty"`
}

type KGInfo struct {
	Available bool     `json:"available"`
	Snapshot  string   `json:"snapshot,omitempty"`
	Gaps      []string `json:"gaps,omitempty"`
	Conflicts []string `json:"conflicts,omitempty"`
}

// DiagramRef points at a drawn diagram by the digest of its model.
type DiagramRef struct {
	Section     ReviewSection `json:"section"`
	Variant     string        `json:"variant"` // design|delta
	Name        string        `json:"name,omitempty"`
	Type        string        `json:"type"`
	InputDigest string        `json:"inputDigest"`
	ArtifactID  string        `json:"artifactId,omitempty"`
	Status      string        `json:"status"` // cached|missing|drawing|failed
}

// OutcomeReview is the whole review as stored and returned. InputsDigest covers what the facts were built from, never a time.
type OutcomeReview struct {
	SchemaVersion int           `json:"schemaVersion"`
	ReviewID      string        `json:"reviewId,omitempty"`
	RequestID     string        `json:"requestId"`
	Phase         ReviewPhase   `json:"phase"`
	Subject       ReviewSubject `json:"subject"`
	InputsDigest  string        `json:"inputsDigest"`
	// FactsDigest covers only database facts; a read compares it to notice a review that went stale without asking git.
	FactsDigest string       `json:"factsDigest"`
	State       ReviewState  `json:"state"`
	Verdict     Verdict      `json:"verdict"`
	Items       []ReviewItem `json:"items"`
	KG          KGInfo       `json:"kg"`
	Diagrams    []DiagramRef `json:"diagrams"`
	Warnings    []string     `json:"warnings"`
	Truncated   bool         `json:"truncated,omitempty"`
}

const OutcomeReviewSchemaVersion = 1
