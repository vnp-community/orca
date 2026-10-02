package domain

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/stablyai/orca-go/common/apperrors"
)

// Custom prompt error codes (CONTRACT section 2.3).
const (
	CodePromptInvalid         = "MCP_PROMPT_INVALID"
	CodePromptNameConflict    = "MCP_PROMPT_NAME_CONFLICT"
	CodePromptVersionConflict = "MCP_PROMPT_VERSION_CONFLICT"

	SubjectPromptChanged = "orca.mcp.prompt.changed"

	AuditActionPromptUpsert = "mcp.prompt.upsert"
	AuditActionPromptDelete = "mcp.prompt.delete"

	MaxPromptsPerTenant   = 50
	MaxPromptArguments    = 10
	MaxPromptTemplateSize = 8192
	MaxPromptDescription  = 500
)

// BuiltinPromptNames are reserved: the gateway ships these templates.
var BuiltinPromptNames = map[string]bool{
	"review_pull_request": true, "triage_issue": true, "plan_task": true,
	"summarize_worktree": true, "handoff_to_agent": true,
}

type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

type CustomPrompt struct {
	ID          string
	Name        string
	Description string
	Arguments   []PromptArgument
	Template    string
	Version     int
	UpdatedAt   time.Time
	UpdatedBy   string
	CreatedBy   string
}

// ErrPromptInvalid renders as "MCP_PROMPT_INVALID: <field>: <reason>".
func ErrPromptInvalid(field, reason string) error {
	return apperrors.New(apperrors.KindInvalidArgument, CodePromptInvalid, field+": "+reason, nil)
}

func ErrPromptNameConflict(name string) error {
	return apperrors.New(apperrors.KindAlreadyExists, CodePromptNameConflict, "a prompt named "+name+" already exists", nil)
}

func ErrPromptVersionConflict(current int) error {
	return apperrors.New(apperrors.KindAlreadyExists, CodePromptVersionConflict,
		fmt.Sprintf("prompt was changed by someone else; reload and retry (current=%d)", current), nil)
}

var (
	promptNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]{2,47}$`)
	promptArgRe  = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)
	// PromptVarRe is the only template syntax custom prompts support.
	PromptVarRe = regexp.MustCompile(`\{\{([^{}]*)\}\}`)

	// Heuristic lint, NOT a security boundary: real permissions are enforced
	// by token scopes + the tool policy gate on every call.
	widenPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)ignore\s+(all\s+)?(the\s+)?(previous|prior|above|earlier)\s+(instructions|rules|prompts)`),
		regexp.MustCompile(`(?i)(bypass|skip|disable|circumvent|override)\b.{0,30}(approval|confirmation|policy|policies|kill\s?switch|permission|guardrail|safety)`),
		regexp.MustCompile(`(?i)without\s+(asking|approval|confirmation|permission|consent)`),
		regexp.MustCompile(`(?i)(do\s+not|don'?t|never)\s+(ask|wait|request)\b.{0,20}(user|approval|confirmation|permission)`),
		regexp.MustCompile(`(?i)(b[oỏ]\s*qua|bo\s*qua)\b.{0,30}(phê\s*duyệt|phe\s*duyet|xác\s*nhận|xac\s*nhan|chính\s*sách|chinh\s*sach|quyền|quyen)`),
		regexp.MustCompile(`(?i)kh[oô]ng\s+(c[aầ]n\s+)?(h[oỏ]i|x[aá]c\s*nh[aậ]n|ph[eê]\s*duy[eệ]t)`),
		regexp.MustCompile(`<\|[^|]*\|>`),
		regexp.MustCompile(`(?im)^\s*(system|assistant|developer)\s*:`),
		regexp.MustCompile(`(?i)you\s+(are|have)\s+(now\s+)?(admin|root|an?\s+administrator|unrestricted)`),
	}
	bareURLRe   = regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.\-]*://\S+`)
	bareEmailRe = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
)

// Validate applies every per-prompt rule. Tenant-wide limits (50 prompts) and
// version checks belong to the usecase/repository.
func (p CustomPrompt) Validate() error {
	if !promptNameRe.MatchString(p.Name) {
		return ErrPromptInvalid("name", "must match ^[a-z][a-z0-9_]{2,47}$")
	}
	if BuiltinPromptNames[p.Name] {
		return ErrPromptInvalid("name", "is reserved by a built-in prompt")
	}
	if utf8.RuneCountInString(p.Description) > MaxPromptDescription {
		return ErrPromptInvalid("description", "must be at most 500 characters")
	}
	if p.Template == "" || len(p.Template) > MaxPromptTemplateSize || !utf8.ValidString(p.Template) {
		return ErrPromptInvalid("template", "must be valid UTF-8 between 1 and 8192 bytes")
	}
	if hasControlChars(p.Template) {
		return ErrPromptInvalid("template", "must not contain control characters")
	}
	if len(p.Arguments) > MaxPromptArguments {
		return ErrPromptInvalid("arguments", "at most 10 arguments are allowed")
	}
	declared := map[string]PromptArgument{}
	for i, a := range p.Arguments {
		field := fmt.Sprintf("arguments[%d].name", i)
		if !promptArgRe.MatchString(a.Name) {
			return ErrPromptInvalid(field, "must match ^[a-z][a-z0-9_]{0,31}$")
		}
		if _, dup := declared[a.Name]; dup {
			return ErrPromptInvalid(field, "is declared twice")
		}
		if utf8.RuneCountInString(a.Description) > MaxPromptDescription {
			return ErrPromptInvalid(fmt.Sprintf("arguments[%d].description", i), "must be at most 500 characters")
		}
		declared[a.Name] = a
	}
	// Strip well-formed placeholders; any brace left is malformed.
	used := map[string]bool{}
	for _, m := range PromptVarRe.FindAllStringSubmatch(p.Template, -1) {
		name := m[1]
		if _, ok := declared[name]; !ok {
			return ErrPromptInvalid("template", "uses undeclared variable {{"+name+"}}")
		}
		used[name] = true
	}
	if rest := PromptVarRe.ReplaceAllString(p.Template, ""); strings.ContainsAny(rest, "{}") {
		return ErrPromptInvalid("template", "contains an unbalanced '{{' or '}}'")
	}
	for _, a := range p.Arguments {
		if a.Required && !used[a.Name] {
			return ErrPromptInvalid("template", "required argument "+a.Name+" is never used")
		}
	}
	// Lint literal text only, so tenant data can never be mistaken for text.
	literal := PromptVarRe.ReplaceAllString(p.Template, " ")
	for _, re := range widenPatterns {
		if re.MatchString(literal) || re.MatchString(p.Description) {
			return ErrPromptInvalid("template", "contains instructions that try to change permissions or approvals")
		}
	}
	if bareURLRe.MatchString(literal) || bareEmailRe.MatchString(literal) {
		return ErrPromptInvalid("template", "must not embed URLs or e-mail addresses; pass them as arguments")
	}
	return nil
}

func hasControlChars(s string) bool {
	for _, r := range s {
		if r < 0x20 && r != '\n' && r != '\t' && r != '\r' {
			return true
		}
		if r == 0x7f {
			return true
		}
	}
	return false
}
