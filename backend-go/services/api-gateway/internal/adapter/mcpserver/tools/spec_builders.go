package tools

import (
	"strings"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
)

func newSpec(pack int, risk Risk, channel, desc string, fields []Field) *ToolSpec {
	s := &ToolSpec{
		Name: ChannelToToolName(channel), Channel: channel, Namespace: namespaceOf(channel),
		Pack: pack, Risk: risk, Description: desc, Fields: fields, Title: titleOf(channel),
	}
	switch risk {
	case mcpserver.RiskRead:
		s.Annotations = Annotations{ReadOnly: true, Idempotent: true}
	case mcpserver.RiskDestructive:
		s.Annotations = Annotations{Destructive: true}
	}
	return s
}

func titleOf(channel string) string {
	var b strings.Builder
	for i, r := range channel {
		switch {
		case r == '.':
			b.WriteByte(' ')
		case i > 0 && r >= 'A' && r <= 'Z':
			b.WriteByte(' ')
			b.WriteRune(r + ('a' - 'A'))
		default:
			b.WriteRune(r)
		}
	}
	t := b.String()
	if t == "" {
		return t
	}
	return strings.ToUpper(t[:1]) + t[1:]
}

// Pack 1: read-only discovery. Pack 2: reversible writes. Pack 3: exec.
// Pack 4: destructive/admin. Pack 3/4 specs are Declared until their Args are
// reviewed together with governance (BE-012/013).
func read(channel, desc string, f ...Field) *ToolSpec {
	return newSpec(1, mcpserver.RiskRead, channel, desc, f)
}
func write(channel, desc string, f ...Field) *ToolSpec {
	return newSpec(2, mcpserver.RiskWriteReversible, channel, desc, f)
}
func execTool(channel, desc string, f ...Field) *ToolSpec {
	return newSpec(3, mcpserver.RiskExec, channel, desc, f)
}
func destructive(channel, desc string, f ...Field) *ToolSpec {
	return newSpec(4, mcpserver.RiskDestructive, channel, desc, f)
}
func adminRead(channel, desc string, f ...Field) *ToolSpec {
	s := newSpec(4, mcpserver.RiskAdmin, channel, desc, f)
	s.Annotations = Annotations{ReadOnly: true, Idempotent: true}
	return s
}

func (s *ToolSpec) asList() *ToolSpec    { s.List = true; return s }
func (s *ToolSpec) untrusted() *ToolSpec { s.Untrusted = true; return s }
func (s *ToolSpec) openWorld() *ToolSpec { s.Annotations.OpenWorld = true; return s }
func (s *ToolSpec) idempotent() *ToolSpec {
	s.Annotations.Idempotent = true
	return s
}
func (s *ToolSpec) cached() *ToolSpec   { s.CacheTTL = 30; return s }
func (s *ToolSpec) declared() *ToolSpec { s.Declared = true; return s }
func (s *ToolSpec) consts(m map[string]any) *ToolSpec {
	s.Consts = m
	return s
}
func (s *ToolSpec) named(name, reason string) *ToolSpec {
	s.Name, s.NameReason = name, reason
	return s
}

// scm marks read tools hitting GitHub/GitLab: open world, untrusted text,
// cached 30s to spare the user's API rate limit.
func (s *ToolSpec) scm() *ToolSpec { return s.openWorld().untrusted().cached() }

// Shared field shapes.
func worktreeSel() Field {
	return Str("worktree_id", "worktree", "Worktree id (see worktree_list)", Req, IDSel)
}
func worktreeIDField() Field {
	return Str("worktree_id", "worktreeId", "Worktree id (see worktree_list)", Req)
}
func pageFields() []Field {
	return []Field{
		Int("page_size", "pageSize", "Items per page", Range(1, 200)),
		Str("page_token", "pageToken", "Token from a previous page"),
	}
}
func with(base []Field, more ...Field) []Field { return append(append([]Field{}, base...), more...) }

func (s *ToolSpec) pii() *ToolSpec { s.PII = true; return s }

// guardPath marks a files tool whose input path and result paths are screened.
func (s *ToolSpec) guardPath(arg string, optional bool, filter pathFilterKind) *ToolSpec {
	s.PathArg, s.PathOptional, s.PathFilter = arg, optional, filter
	return s
}
