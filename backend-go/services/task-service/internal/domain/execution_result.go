package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Result block codes. The first four mirror what a protocol-2 agent reports in `parsed`;
// RESULT_TOO_LARGE and RESULT_SCHEMA_INVALID come from this side.
const (
	ResultCodeBlockMissing = "RESULT_BLOCK_MISSING"
	ResultCodeInvalidJSON  = "RESULT_BLOCK_INVALID_JSON"
	ResultCodeNotObject    = "RESULT_BLOCK_NOT_OBJECT"
	ResultCodeTooLarge     = "RESULT_TOO_LARGE"
	ResultCodeSchema       = "RESULT_SCHEMA_INVALID"
)

const (
	MaxResultBlockBytes   = 256 * 1024
	MaxResultSummaryBytes = 2048
	MaxResultListEntries  = 200
	MaxResultOutputsBytes = 16384
	resultSchemaVersion   = 1
)

type ResultStatus string

const (
	ResultDone      ResultStatus = "done"
	ResultBlocked   ResultStatus = "blocked"
	ResultFailed    ResultStatus = "failed"
	ResultNeedsInfo ResultStatus = "needs_info"
)

type CheckRun struct {
	ID   string `json:"id"`
	Exit int    `json:"exit"`
}

// ExecutionResult is the JSON object an agent prints between the nonce markers (CR-REQ-029 2.5).
type ExecutionResult struct {
	SchemaVersion int                        `json:"schema_version"`
	Status        ResultStatus               `json:"status"`
	Summary       string                     `json:"summary"`
	FilesChanged  []string                   `json:"files_changed"`
	ChecksRun     []CheckRun                 `json:"-"`
	Outputs       map[string]json.RawMessage `json:"outputs"`
	Questions     []string                   `json:"questions"`
	Notes         string                     `json:"notes"`
}

// AgentParsed is what a protocol-2 agent reports instead of making us scan stdout.
type AgentParsed struct {
	OK     bool
	Value  json.RawMessage
	Code   string
	Detail string
}

// ParsedExecution is the verdict on one run's result block. Raw is the compacted JSON body
// whenever it was a valid document (even if the schema check failed) for the audit column.
type ParsedExecution struct {
	Status ParseStatus
	Code   string
	Result *ExecutionResult
	Raw    []byte
}

var resultNoncePattern = regexp.MustCompile(`^[A-Za-z0-9]{16,64}$`)

// FindResultBlock returns the body of the last whole-line ORCA_RESULT_BEGIN/END pair carrying
// the nonce. Only the nonce holder can write a valid marker, so a block quoted from Request
// content (wrong or absent nonce) never matches; scanning from the end avoids walking 50 MB twice.
func FindResultBlock(stdout, nonce string) (body []byte, code string) {
	if !resultNoncePattern.MatchString(nonce) {
		return nil, ResultCodeBlockMissing
	}
	end := lastMarkerLine(stdout, "ORCA_RESULT_END "+nonce, len(stdout))
	if end < 0 {
		return nil, ResultCodeBlockMissing
	}
	begin := lastMarkerLine(stdout, "ORCA_RESULT_BEGIN "+nonce, end)
	if begin < 0 {
		return nil, ResultCodeBlockMissing
	}
	start := begin + len("ORCA_RESULT_BEGIN "+nonce)
	inner := strings.TrimSpace(stdout[start:end])
	if len(inner) > MaxResultBlockBytes {
		return nil, ResultCodeTooLarge
	}
	return []byte(inner), ""
}

// lastMarkerLine finds the last occurrence of marker that fills a whole line within s[:limit].
func lastMarkerLine(s, marker string, limit int) int {
	for limit > 0 {
		i := strings.LastIndex(s[:limit], marker)
		if i < 0 {
			return -1
		}
		before := i == 0 || s[i-1] == '\n'
		after := i + len(marker)
		atEnd := after == len(s) || s[after] == '\n' || (s[after] == '\r' && (after+1 == len(s) || s[after+1] == '\n'))
		if before && atEnd {
			return i
		}
		limit = i
	}
	return -1
}

// ParseExecutionResult decides parse status and content. An agent-supplied `parsed` is trusted
// for locating the block (it found it with the same nonce rule); the schema is always ours to check.
func ParseExecutionResult(stdout, nonce string, parsed *AgentParsed) ParsedExecution {
	var body []byte
	if parsed != nil {
		if !parsed.OK {
			if parsed.Code == ResultCodeBlockMissing || parsed.Code == "" {
				return ParsedExecution{Status: ParseStatusMissing, Code: ResultCodeBlockMissing}
			}
			return ParsedExecution{Status: ParseStatusInvalid, Code: parsed.Code}
		}
		body = parsed.Value
	} else {
		var code string
		body, code = FindResultBlock(stdout, nonce)
		switch code {
		case "":
		case ResultCodeBlockMissing:
			return ParsedExecution{Status: ParseStatusMissing, Code: code}
		default:
			return ParsedExecution{Status: ParseStatusInvalid, Code: code}
		}
	}
	if len(body) > MaxResultBlockBytes {
		return ParsedExecution{Status: ParseStatusInvalid, Code: ResultCodeTooLarge}
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, body); err != nil {
		return ParsedExecution{Status: ParseStatusInvalid, Code: ResultCodeInvalidJSON}
	}
	raw := compact.Bytes()
	if raw[0] != '{' {
		return ParsedExecution{Status: ParseStatusInvalid, Code: ResultCodeNotObject}
	}
	res, err := decodeExecutionResult(raw)
	if err == nil {
		err = res.Validate()
	}
	if err != nil {
		return ParsedExecution{Status: ParseStatusInvalid, Code: ResultCodeSchema, Raw: raw}
	}
	return ParsedExecution{Status: ParseStatusOK, Result: &res, Raw: raw}
}

// decodeExecutionResult rejects unknown root keys (a typo must not pass as a valid result);
// entries of checks_run stay lenient so an agent adding a per-check detail is not punished.
func decodeExecutionResult(raw []byte) (ExecutionResult, error) {
	var wire struct {
		ExecutionResult
		ChecksRun []json.RawMessage `json:"checks_run"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&wire); err != nil {
		return ExecutionResult{}, err
	}
	res := wire.ExecutionResult
	for _, c := range wire.ChecksRun {
		var cr CheckRun
		if err := json.Unmarshal(c, &cr); err != nil {
			return ExecutionResult{}, fmt.Errorf("checks_run entry: %w", err)
		}
		res.ChecksRun = append(res.ChecksRun, cr)
	}
	return res, nil
}

// Validate enforces the CR size limits and the status/questions coupling.
func (r ExecutionResult) Validate() error {
	if r.SchemaVersion != resultSchemaVersion {
		return fmt.Errorf("schema_version must be %d", resultSchemaVersion)
	}
	switch r.Status {
	case ResultDone, ResultBlocked, ResultFailed, ResultNeedsInfo:
	default:
		return fmt.Errorf("unknown status %q", r.Status)
	}
	if len(r.Summary) > MaxResultSummaryBytes {
		return fmt.Errorf("summary exceeds %d bytes", MaxResultSummaryBytes)
	}
	if len(r.FilesChanged) > MaxResultListEntries || len(r.ChecksRun) > MaxResultListEntries || len(r.Questions) > MaxResultListEntries {
		return fmt.Errorf("a list exceeds %d entries", MaxResultListEntries)
	}
	total := 0
	for k, v := range r.Outputs {
		total += len(k) + len(v)
	}
	if total > MaxResultOutputsBytes {
		return fmt.Errorf("outputs exceed %d bytes", MaxResultOutputsBytes)
	}
	for _, c := range r.ChecksRun {
		if c.ID == "" {
			return fmt.Errorf("checks_run entry without id")
		}
	}
	if r.Status == ResultNeedsInfo && len(r.Questions) == 0 {
		return fmt.Errorf("needs_info requires at least one question")
	}
	return nil
}

// OutputsByName keeps only the outputs the TaskSpec declared (name -> type) and lists declared
// names that are absent or of the wrong type, sorted. Undeclared outputs never flow downstream.
func OutputsByName(r ExecutionResult, declared map[string]string) (map[string]json.RawMessage, []string) {
	kept := map[string]json.RawMessage{}
	var missing []string
	for name, typ := range declared {
		raw, ok := r.Outputs[name]
		if !ok || !outputMatchesType(raw, typ) {
			missing = append(missing, name)
			continue
		}
		kept[name] = raw
	}
	sort.Strings(missing)
	return kept, missing
}

func outputMatchesType(raw json.RawMessage, typ string) bool {
	if !json.Valid(raw) {
		return false
	}
	switch typ {
	case "file_list":
		var v []string
		return json.Unmarshal(raw, &v) == nil && v != nil
	case "number":
		var v json.Number
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.UseNumber()
		return dec.Decode(&v) == nil && raw[0] != '"'
	case "text":
		var v string
		return json.Unmarshal(raw, &v) == nil
	case "api_schema":
		var v map[string]json.RawMessage
		return json.Unmarshal(raw, &v) == nil && v != nil
	case "json":
		return true
	}
	return false
}
