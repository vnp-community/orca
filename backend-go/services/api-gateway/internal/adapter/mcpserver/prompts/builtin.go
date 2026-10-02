// Package prompts serves MCP prompts (BE-MCP-SOL-011): five built-in
// templates embedded in the binary plus tenant custom prompts stored by
// mcp-service. It never decides permissions: embedded resources are read with
// the caller's own identity through the ResourceProvider, and every role in a
// produced message is "user".
package prompts

import (
	"embed"
	"fmt"
	"net/url"
	"strings"
	"text/template"
)

//go:embed builtin/*.tmpl
var builtinFS embed.FS

// Locales with built-in text. English is the fallback.
var Locales = []string{"en", "vi"}

type argKind int

const (
	argText argKind = iota
	argUUID
	argEnum
	argIssueRef
	argNumber
	argRepo
	argSlug
)

type argSpec struct {
	Name     string
	Desc     map[string]string
	Required bool
	Kind     argKind
	Enum     []string
}

// builtinDef is one built-in prompt. URIs returns the resources embedded after
// the instruction text (validated arguments only).
type builtinDef struct {
	Name    string
	Version int
	Desc    map[string]string
	Args    []argSpec
	URIs    func(a map[string]string) []string
}

func d(en, vi string) map[string]string { return map[string]string{"en": en, "vi": vi} }

var builtinDefs = []builtinDef{
	{Name: "review_pull_request", Version: 1, Desc: d("Review a GitHub pull request or GitLab merge request.", "Review một pull request GitHub hoặc merge request GitLab."),
		Args: []argSpec{
			{Name: "provider", Required: true, Kind: argEnum, Enum: []string{"github", "gitlab"}, Desc: d("Hosting provider: github or gitlab.", "Nhà cung cấp: github hoặc gitlab.")},
			{Name: "repo", Required: true, Kind: argRepo, Desc: d("Repository, e.g. owner/name (GitLab subgroups allowed).", "Repository, ví dụ owner/name (cho phép subgroup GitLab).")},
			{Name: "number", Required: true, Kind: argNumber, Desc: d("Pull / merge request number.", "Số của pull / merge request.")},
		},
		URIs: func(a map[string]string) []string {
			return []string{"orca://review/" + a["provider"] + "/" + url.PathEscape(a["repo"]) + "/" + a["number"]}
		}},
	{Name: "triage_issue", Version: 1, Desc: d("Triage an issue: labels, priority and possible sub-tasks.", "Phân loại issue: nhãn, mức ưu tiên và các task con có thể có."),
		Args: []argSpec{{Name: "issue_ref", Required: true, Kind: argIssueRef, Desc: d("Issue reference, e.g. PROJ-123 or github:owner/repo#12.", "Tham chiếu issue, ví dụ PROJ-123 hoặc github:owner/repo#12.")}}},
	{Name: "plan_task", Version: 1, Desc: d("Plan an Orca task (proposal only, nothing runs).", "Lập kế hoạch cho một task Orca (chỉ đề xuất, không chạy gì)."),
		Args: []argSpec{{Name: "task_id", Required: true, Kind: argUUID, Desc: d("Task id.", "Id của task.")}},
		URIs: func(a map[string]string) []string { return []string{"orca://task/" + a["task_id"]} }},
	{Name: "summarize_worktree", Version: 1, Desc: d("Summarize the state of a worktree (read-only).", "Tóm tắt trạng thái của một worktree (chỉ đọc)."),
		Args: []argSpec{{Name: "worktree_id", Required: true, Kind: argUUID, Desc: d("Worktree id.", "Id của worktree.")}},
		URIs: func(a map[string]string) []string { return []string{"orca://worktree/" + a["worktree_id"] + "/status"} }},
	{Name: "handoff_to_agent", Version: 1, Desc: d("Draft a hand-off brief for an agent (starting it needs approval).", "Soạn bản bàn giao cho một agent (khởi chạy agent cần phê duyệt)."),
		Args: []argSpec{
			{Name: "task_id", Required: true, Kind: argUUID, Desc: d("Task id.", "Id của task.")},
			{Name: "agent", Required: true, Kind: argSlug, Desc: d("Agent kind, e.g. claude.", "Loại agent, ví dụ claude.")},
		},
		URIs: func(a map[string]string) []string { return []string{"orca://task/" + a["task_id"]} }},
}

// trailer is appended to every prompt. It is consistency guidance, NOT a
// security feature: the server enforces permissions on every call.
var trailer = map[string]string{
	"en": "Server enforces permissions; if an action needs approval, wait for the user - do not try to bypass it.",
	"vi": "Máy chủ luôn kiểm soát quyền; nếu một hành động cần phê duyệt, hãy chờ người dùng - không tìm cách vượt qua.",
}

// templates are parsed once at init: a broken template must fail the build's
// tests, not a request.
var templates = mustParseTemplates()

func mustParseTemplates() map[string]*template.Template {
	out := map[string]*template.Template{}
	for _, def := range builtinDefs {
		for _, loc := range Locales {
			file := fmt.Sprintf("builtin/%s.%s.tmpl", def.Name, loc)
			src, err := builtinFS.ReadFile(file)
			if err != nil {
				panic("prompts: missing embedded template " + file)
			}
			// No FuncMap: templates cannot call anything; missing keys fail.
			t, err := template.New(file).Option("missingkey=error").Parse(string(src))
			if err != nil {
				panic("prompts: " + file + ": " + err.Error())
			}
			out[def.Name+"."+loc] = t
		}
	}
	return out
}

func builtinByName(name string) (builtinDef, bool) {
	for _, b := range builtinDefs {
		if b.Name == name {
			return b, true
		}
	}
	return builtinDef{}, false
}

// BuiltinInfo describes a built-in prompt for the admin channel.
type BuiltinInfo struct {
	Name, Description string
	Version           int
	Arguments         []ArgumentInfo
	// Template is the English source (a Go template; shown read-only).
	Template string
}

type ArgumentInfo struct {
	Name, Description string
	Required          bool
}

// Builtins lists built-in prompts in definition order.
func Builtins() []BuiltinInfo {
	out := make([]BuiltinInfo, 0, len(builtinDefs))
	for _, b := range builtinDefs {
		src, _ := builtinFS.ReadFile("builtin/" + b.Name + ".en.tmpl")
		bi := BuiltinInfo{Name: b.Name, Description: b.Desc["en"], Version: b.Version, Template: strings.TrimSpace(string(src))}
		for _, a := range b.Args {
			bi.Arguments = append(bi.Arguments, ArgumentInfo{Name: a.Name, Description: a.Desc["en"], Required: a.Required})
		}
		out = append(out, bi)
	}
	return out
}
