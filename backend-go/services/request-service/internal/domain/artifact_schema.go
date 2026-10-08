package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/stablyai/orca-go/common/apperrors"
	"io/fs"
	"sort"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

const (
	CodeArtifactSchemaInvalid            = "REQUEST_ARTIFACT_SCHEMA_INVALID"
	CodeArtifactSchemaVersionUnsupported = "REQUEST_ARTIFACT_SCHEMA_VERSION_UNSUPPORTED"
	CodeArtifactLimitExceeded            = "REQUEST_ARTIFACT_LIMIT_EXCEEDED"

	// MaxViolations bounds the report so a hostile document cannot make it unbounded.
	MaxViolations = 50
)

// Upgrader turns a document of version N into N+1. It must be pure.
type Upgrader func(raw []byte) ([]byte, error)

type upgradeKey struct {
	kind ArtifactKind
	from int
}

// SchemaRegistry compiles the embedded schemas once; the compiled schemas are safe for concurrent use.
// Library choice (spike, see IMPLEMENTATION-NOTES): santhosh-tekuri/jsonschema/v6, because
// google/jsonschema-go reports only the first error and by schema path rather than instance pointer.
type SchemaRegistry struct {
	schemas   map[ArtifactKind]*jsonschema.Schema
	mu        sync.RWMutex
	upgraders map[upgradeKey]Upgrader
}

// NewSchemaRegistry compiles v1/<kind>.schema.json for every kind from fsys without any network access.
func NewSchemaRegistry(fsys fs.FS) (*SchemaRegistry, error) {
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	r := &SchemaRegistry{schemas: map[ArtifactKind]*jsonschema.Schema{}, upgraders: map[upgradeKey]Upgrader{}}
	urls := map[ArtifactKind]string{}
	for _, k := range AllArtifactKinds() {
		name := fmt.Sprintf("v1/%s.schema.json", k)
		raw, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, fmt.Errorf("schema registry: %w", err)
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			return nil, fmt.Errorf("schema registry: %s: %w", name, err)
		}
		url := fmt.Sprintf("https://orca.local/schemas/v1/%s.schema.json", k)
		if err := c.AddResource(url, doc); err != nil {
			return nil, fmt.Errorf("schema registry: %s: %w", name, err)
		}
		urls[k] = url
	}
	for k, url := range urls {
		s, err := c.Compile(url)
		if err != nil {
			return nil, fmt.Errorf("schema registry: compile %s: %w", k, err)
		}
		r.schemas[k] = s
	}
	return r, nil
}

// RegisterUpgrader installs the pure function that lifts kind from version `from` to from+1.
func (r *SchemaRegistry) RegisterUpgrader(kind ArtifactKind, from int, fn Upgrader) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.upgraders[upgradeKey{kind, from}] = fn
}

// Upgrade runs the registered chain from `from` up to LatestSchemaVersion. Readers call it; writers never write old versions back.
func (r *SchemaRegistry) Upgrade(kind ArtifactKind, from int, raw []byte) ([]byte, error) {
	latest := LatestSchemaVersion(kind)
	if from > latest {
		return nil, fmt.Errorf("%s: schema_version %d is newer than %d", CodeArtifactSchemaVersionUnsupported, from, latest)
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	cur := raw
	for v := from; v < latest; v++ {
		fn, ok := r.upgraders[upgradeKey{kind, v}]
		if !ok {
			return nil, fmt.Errorf("no upgrader for %s v%d to v%d", kind, v, v+1)
		}
		next, err := fn(cur)
		if err != nil {
			return nil, fmt.Errorf("upgrade %s v%d: %w", kind, v, err)
		}
		cur = next
	}
	return cur, nil
}

// DeclaredSchemaVersion reads the schema_version member without validating anything else.
func DeclaredSchemaVersion(raw []byte) (int, error) {
	var probe struct {
		V json.Number `json:"schema_version"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return 0, err
	}
	n, err := probe.V.Int64()
	if err != nil {
		return 0, fmt.Errorf("schema_version is not an integer")
	}
	return int(n), nil
}

// ValidateDocument reads the declared version itself, then validates.
func (r *SchemaRegistry) ValidateDocument(kind ArtifactKind, raw []byte) []Violation {
	v, err := DeclaredSchemaVersion(raw)
	if err != nil {
		return r.Validate(kind, 0, raw)
	}
	return r.Validate(kind, v, raw)
}

// Validate reports every violation (up to MaxViolations) instead of stopping at the first.
func (r *SchemaRegistry) Validate(kind ArtifactKind, version int, raw []byte) []Violation {
	schema, ok := r.schemas[kind]
	if !ok {
		return []Violation{{Code: CodeArtifactSchemaInvalid, Message: fmt.Sprintf("unknown artifact kind %q", kind)}}
	}
	if len(raw) > MaxArtifactBytes(kind) {
		return []Violation{{Code: CodeArtifactLimitExceeded, Message: fmt.Sprintf("%s document is %d bytes, limit is %d", kind, len(raw), MaxArtifactBytes(kind))}}
	}
	if version > LatestSchemaVersion(kind) {
		return []Violation{{Path: "/schema_version", Code: CodeArtifactSchemaVersionUnsupported,
			Message: fmt.Sprintf("schema_version %d is newer than the supported %d", version, LatestSchemaVersion(kind))}}
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return []Violation{syntaxViolation(raw, err)}
	}
	var out []Violation
	if err := schema.Validate(inst); err != nil {
		var ve *jsonschema.ValidationError
		if errors.As(err, &ve) {
			out = collectViolations(ve)
		} else {
			out = []Violation{{Code: CodeArtifactSchemaInvalid, Message: err.Error()}}
		}
	}
	out = append(out, uniqueIDViolations(kind, raw)...)
	return finalizeViolations(out)
}

func syntaxViolation(raw []byte, err error) Violation {
	v := Violation{Code: CodeArtifactSchemaInvalid, Message: "invalid JSON: " + err.Error()}
	var se *json.SyntaxError
	if errors.As(err, &se) {
		v.Line = 1 + bytes.Count(raw[:min(int(se.Offset), len(raw))], []byte("\n"))
	}
	return v
}

func collectViolations(ve *jsonschema.ValidationError) []Violation {
	p := message.NewPrinter(language.English)
	var out []Violation
	var walk func(e *jsonschema.ValidationError)
	walk = func(e *jsonschema.ValidationError) {
		if len(e.Causes) == 0 {
			out = append(out, Violation{
				Path:    jsonPointer(e.InstanceLocation),
				Code:    CodeArtifactSchemaInvalid,
				Message: e.ErrorKind.LocalizedString(p),
			})
			return
		}
		for _, c := range e.Causes {
			walk(c)
		}
	}
	walk(ve)
	return out
}

func jsonPointer(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	var b strings.Builder
	for _, s := range parts {
		b.WriteByte('/')
		b.WriteString(strings.NewReplacer("~", "~0", "/", "~1").Replace(s))
	}
	return b.String()
}

// finalizeViolations sorts, de-duplicates and caps so the report is deterministic.
func finalizeViolations(vs []Violation) []Violation {
	sort.SliceStable(vs, func(i, j int) bool {
		if vs[i].Path != vs[j].Path {
			return vs[i].Path < vs[j].Path
		}
		if vs[i].Code != vs[j].Code {
			return vs[i].Code < vs[j].Code
		}
		return vs[i].Message < vs[j].Message
	})
	out := vs[:0:0]
	for i, v := range vs {
		if i > 0 && v == vs[i-1] {
			continue
		}
		out = append(out, v)
		if len(out) == MaxViolations {
			break
		}
	}
	return out
}

// uniqueIDViolations covers what JSON Schema cannot say: ids that must be unique inside an object array.
func uniqueIDViolations(kind ArtifactKind, raw []byte) []Violation {
	var doc map[string]json.RawMessage
	if json.Unmarshal(raw, &doc) != nil {
		return nil
	}
	var targets []string
	switch kind {
	case ArtifactKindRequest:
		targets = []string{"acceptance_criteria"}
	case ArtifactKindSolution:
		targets = []string{"options", "assumptions", "open_questions"}
	case ArtifactKindTask:
		targets = []string{"checks"}
	}
	var out []Violation
	for _, name := range targets {
		var items []struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(doc[name], &items) != nil {
			continue
		}
		seen := map[string]bool{}
		for i, it := range items {
			if it.ID == "" {
				continue
			}
			if seen[it.ID] {
				out = append(out, Violation{Path: fmt.Sprintf("/%s/%d/id", name, i), Code: CodeArtifactSchemaInvalid, Message: fmt.Sprintf("duplicate id %q", it.ID)})
			}
			seen[it.ID] = true
		}
	}
	return out
}

// ErrArtifactSchemaInvalid wraps a violation list in one error; the first violations ride in the message.
func ErrArtifactSchemaInvalid(vs []Violation) error {
	var parts []string
	for i, v := range vs {
		if i == 3 {
			parts = append(parts, fmt.Sprintf("and %d more", len(vs)-3))
			break
		}
		parts = append(parts, fmt.Sprintf("%s %s", v.Path, v.Message))
	}
	return apperrors.New(apperrors.KindInvalidArgument, CodeArtifactSchemaInvalid, strings.Join(parts, "; "), nil)
}
