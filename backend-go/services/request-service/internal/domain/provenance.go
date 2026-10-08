package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
)

type GeneratorKind string

const (
	GeneratorNative   GeneratorKind = "native"
	GeneratorOpenSpec GeneratorKind = "openspec"
	GeneratorHuman    GeneratorKind = "human"
	GeneratorMCP      GeneratorKind = "mcp"
)

type ModelSource string

const (
	ModelSourceAgentResponse ModelSource = "agent_response"
	ModelSourceParam         ModelSource = "param"
	ModelSourceUnknown       ModelSource = "unknown"
)

// Provenance records where an artifact came from. It deliberately has no credential,
// key or env field: nothing secret can be stored by construction (CR-REQ-027 section 2.6).
type Provenance struct {
	Generator   ProvenanceGenerator `json:"generator"`
	Prompt      ProvenancePrompt    `json:"prompt"`
	RunID       string              `json:"run_id,omitempty"`
	Attempt     int                 `json:"attempt,omitempty"`
	InputDigest string              `json:"input_digest,omitempty"`
	InputRefs   []InputRef          `json:"input_refs,omitempty"`
	Actor       ProvenanceActor     `json:"actor"`
	GeneratedAt time.Time           `json:"generated_at"`
}

type ProvenanceGenerator struct {
	Kind        GeneratorKind `json:"kind"`
	Tool        string        `json:"tool,omitempty"`
	Model       string        `json:"model,omitempty"`
	ModelSource ModelSource   `json:"model_source,omitempty"`
}

type ProvenancePrompt struct {
	Template string `json:"template,omitempty"`
	Version  string `json:"version,omitempty"`
	Digest   string `json:"digest,omitempty"`
}

type InputRef struct {
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	Revision int    `json:"revision,omitempty"`
}

type ProvenanceActor struct {
	ID   string            `json:"id,omitempty"`
	Kind RevisionActorKind `json:"kind"`
}

func ErrProvenanceInvalid(reason string) error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_ARTIFACT_SCHEMA_INVALID", "provenance: "+reason, nil)
}

func (p Provenance) Validate() error {
	switch p.Generator.Kind {
	case GeneratorNative, GeneratorOpenSpec, GeneratorHuman, GeneratorMCP:
	default:
		return ErrProvenanceInvalid("generator.kind must be native|openspec|human|mcp")
	}
	switch p.Generator.ModelSource {
	case "", ModelSourceAgentResponse, ModelSourceParam, ModelSourceUnknown:
	default:
		return ErrProvenanceInvalid("generator.model_source must be agent_response|param|unknown")
	}
	// A model name without a source cannot be told apart from a guess.
	if p.Generator.Model != "" && p.Generator.ModelSource == ModelSourceUnknown {
		return ErrProvenanceInvalid("model is set but model_source is unknown")
	}
	switch p.Actor.Kind {
	case RevisionActorAI, RevisionActorUser, RevisionActorSystem:
	default:
		return ErrProvenanceInvalid("actor.kind must be ai|user|system")
	}
	if p.InputDigest != "" && !strings.HasPrefix(p.InputDigest, "sha256:") {
		return ErrProvenanceInvalid("input_digest must start with sha256:")
	}
	return nil
}

// InputDigestParts is everything that influenced a generation, so a changed prompt or Request revision changes the digest.
type InputDigestParts struct {
	RequestSnapshot      any
	PriorArtifactDigests []string
	PromptVersion        string
	ProjectContextDigest string
}

// ComputeProvenanceInputDigest returns "sha256:<hex>" over the canonical JSON of the parts.
func ComputeProvenanceInputDigest(in InputDigestParts) (string, error) {
	prior := append([]string(nil), in.PriorArtifactDigests...)
	doc := map[string]any{
		"request":         in.RequestSnapshot,
		"prior_artifacts": prior,
		"prompt_version":  in.PromptVersion,
		"project_context": in.ProjectContextDigest,
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("input digest: %w", err)
	}
	d, err := DigestRaw(raw)
	if err != nil {
		return "", err
	}
	return "sha256:" + d, nil
}

// ParseProvenance refuses unknown members, so a document carrying credential_ref, a key or env never loads.
func ParseProvenance(raw []byte) (Provenance, error) {
	var p Provenance
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return Provenance{}, ErrProvenanceInvalid(err.Error())
	}
	if err := p.Validate(); err != nil {
		return Provenance{}, err
	}
	return p, nil
}
