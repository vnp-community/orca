package domain

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stablyai/orca-go/common/apperrors"
)

// RequestContent is the part of a Request that only AppendRequestRevision may change.
// TypeFields never holds ac_next in memory; it lives in AcceptanceCriteria.Next and is
// written into the type_fields column and the snapshot, as CR-REQ-027 section 2.4 puts it.
type RequestContent struct {
	Title              string
	Body               string
	Type               RequestType
	AcceptanceCriteria AcceptanceCriteria
	TypeFields         map[string]any
}

const typeFieldACNext = "ac_next"

func ErrRequestContentInvalid(reason string) error {
	return apperrors.New(apperrors.KindInvalidArgument, CodeArtifactSchemaInvalid, "request content: "+reason, nil)
}

// ContentFromRequest reads the content columns of r.
func ContentFromRequest(r Request) (RequestContent, error) {
	return ParseRequestContent(r.Title, r.Body, r.Type, r.AcceptanceCriteriaJSON, r.TypeFieldsJSON)
}

// ParseRequestContent builds content from the column values; empty JSON means "[]" and "{}".
func ParseRequestContent(title, body string, t RequestType, acJSON, tfJSON []byte) (RequestContent, error) {
	c := RequestContent{Title: title, Body: body, Type: t, TypeFields: map[string]any{}}
	if len(acJSON) > 0 {
		if err := json.Unmarshal(acJSON, &c.AcceptanceCriteria.Items); err != nil {
			return RequestContent{}, ErrRequestContentInvalid("acceptance_criteria: " + err.Error())
		}
	}
	if len(tfJSON) > 0 {
		if err := json.Unmarshal(tfJSON, &c.TypeFields); err != nil {
			return RequestContent{}, ErrRequestContentInvalid("type_fields: " + err.Error())
		}
		if c.TypeFields == nil {
			c.TypeFields = map[string]any{}
		}
	}
	if v, ok := c.TypeFields[typeFieldACNext]; ok {
		n, isNum := v.(float64)
		if !isNum || n < 1 || n != float64(int(n)) {
			return RequestContent{}, ErrRequestContentInvalid("type_fields.ac_next must be a positive integer")
		}
		c.AcceptanceCriteria.Next = int(n)
		delete(c.TypeFields, typeFieldACNext)
	}
	if max := c.AcceptanceCriteria.maxNumber(); c.AcceptanceCriteria.Next <= max {
		c.AcceptanceCriteria.Next = max + 1 // a missing or stale counter must never reissue a number
	}
	return c, nil
}

// AcceptanceCriteriaJSON is the value of the acceptance_criteria column (never null).
func (c RequestContent) AcceptanceCriteriaJSON() []byte {
	items := c.AcceptanceCriteria.Items
	if items == nil {
		items = []AcceptanceCriterion{}
	}
	b, _ := json.Marshal(items) // plain structs always marshal
	return b
}

// TypeFieldsJSON is the value of the type_fields column, with the ac_next counter included.
func (c RequestContent) TypeFieldsJSON() []byte {
	m := make(map[string]any, len(c.TypeFields)+1)
	for k, v := range c.TypeFields {
		m[k] = v
	}
	if c.AcceptanceCriteria.Next > 1 {
		m[typeFieldACNext] = c.AcceptanceCriteria.Next
	}
	b, _ := json.Marshal(m)
	return b
}

// Snapshot is the canonical JSON stored in request_revisions.snapshot. meta carries notes such
// as a readiness waiver; it is not part of the content digest.
func (c RequestContent) Snapshot(meta map[string]any) ([]byte, error) {
	doc := map[string]any{
		"schema_version":      LatestSchemaVersion(ArtifactKindRequest),
		"title":               NormalizeNFC(c.Title),
		"body":                NormalizeNFC(c.Body),
		"type":                string(c.Type),
		"acceptance_criteria": json.RawMessage(c.AcceptanceCriteriaJSON()),
		"type_fields":         json.RawMessage(c.TypeFieldsJSON()),
	}
	if len(meta) > 0 {
		doc["meta"] = meta
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("request snapshot: %w", err)
	}
	return CanonicalJSON(raw)
}

// Digest hashes the content only (no meta), so editing back to an earlier text yields the earlier digest.
func (c RequestContent) Digest() (string, error) {
	snap, err := c.Snapshot(nil)
	if err != nil {
		return "", err
	}
	return DigestOfCanonical(snap), nil
}

// ContentPatch is a partial edit; nil members stay as they are.
type ContentPatch struct {
	Title              *string
	Body               *string
	AcceptanceCriteria *[]ACInput
	// TypeFields is merged key by key; a nil value deletes the key.
	TypeFields map[string]any
}

// Apply returns the patched copy; the receiver is untouched.
func (c RequestContent) Apply(p ContentPatch) (RequestContent, error) {
	out := c
	out.AcceptanceCriteria = AcceptanceCriteria{Items: append([]AcceptanceCriterion(nil), c.AcceptanceCriteria.Items...), Next: c.AcceptanceCriteria.Next}
	out.TypeFields = make(map[string]any, len(c.TypeFields))
	for k, v := range c.TypeFields {
		out.TypeFields[k] = v
	}
	if p.Title != nil {
		out.Title = strings.TrimSpace(NormalizeNFC(*p.Title))
	}
	if p.Body != nil {
		out.Body = NormalizeNFC(*p.Body)
	}
	if p.AcceptanceCriteria != nil {
		ac, err := c.AcceptanceCriteria.Reconcile(*p.AcceptanceCriteria)
		if err != nil {
			return RequestContent{}, err
		}
		out.AcceptanceCriteria = ac
	}
	for k, v := range p.TypeFields {
		if k == typeFieldACNext {
			return RequestContent{}, ErrRequestContentInvalid("ac_next is managed by the server")
		}
		if v == nil {
			delete(out.TypeFields, k)
			continue
		}
		out.TypeFields[k] = v
	}
	return out, nil
}

// WithContent returns r with the content columns replaced and the digest recomputed.
// Type is not touched: the type changes through the classification flow, not through content edits.
func (r Request) WithContent(c RequestContent) (Request, error) {
	d, err := c.Digest()
	if err != nil {
		return Request{}, err
	}
	r.Title, r.Body = c.Title, c.Body
	r.AcceptanceCriteriaJSON, r.TypeFieldsJSON = c.AcceptanceCriteriaJSON(), c.TypeFieldsJSON()
	r.ContentDigest = d
	return r, nil
}

// InitialContent builds the content a request is created with. Criteria get their numbers here; ac_next belongs to the server.
func InitialContent(title, body string, ac []ACInput, typeFields map[string]any) (RequestContent, error) {
	c := RequestContent{Title: title, Body: body, TypeFields: map[string]any{}}
	if len(ac) > 0 {
		items, err := AcceptanceCriteria{}.Reconcile(ac)
		if err != nil {
			return RequestContent{}, err
		}
		c.AcceptanceCriteria = items
	}
	for k, v := range typeFields {
		if k == typeFieldACNext {
			return RequestContent{}, ErrRequestContentInvalid("ac_next is managed by the server")
		}
		if v != nil {
			c.TypeFields[k] = v
		}
	}
	return c, nil
}
