package resources

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver/tools"
)

// call is one read-only channel invocation.
type call struct {
	channel string
	args    map[string]any
	// preview marks files.readPreview results (base64 -> text).
	preview bool
}

// plan maps a Ref to the policy meta and the channels that back it. The meta
// is what the PolicyGate sees: Name "resource:<kind>", risk read.
type plan struct {
	meta      mcpserver.ToolMeta
	calls     []call
	untrusted bool
	// text selects text rendering of calls[0] instead of a JSON document.
	text bool
	mime string
}

func readMeta(kind Kind, channel string, untrusted, openWorld bool) mcpserver.ToolMeta {
	ns := channel
	if i := strings.IndexByte(channel, '.'); i > 0 {
		ns = channel[:i]
	}
	return mcpserver.ToolMeta{Name: "resource:" + string(kind), Channel: channel, Namespace: ns, Risk: mcpserver.RiskRead,
		RequiredScope: "orca:read", OpenWorld: openWorld, UntrustedOutput: untrusted}
}

// errNotReadable folds "disabled", "sensitive path" and the like into the
// not-found answer.
var errNotReadable = errors.New("resources: not readable")

func (r *Provider) planFor(ref Ref) (plan, error) {
	sel := "id:" + ref.ID // git.* channels take an "id:"-prefixed selector under "worktree"
	switch ref.Kind {
	case KindProjects:
		return plan{meta: readMeta(ref.Kind, "project.list", false, false), calls: []call{{channel: "project.list", args: map[string]any{}}}}, nil
	case KindProject:
		return plan{meta: readMeta(ref.Kind, "project.get", false, false), calls: []call{
			{channel: "project.get", args: map[string]any{"projectId": ref.ID}},
			{channel: "worktree.list", args: map[string]any{"projectId": ref.ID}},
		}}, nil
	case KindTask:
		return plan{meta: readMeta(ref.Kind, "task.get", true, false), untrusted: true, calls: []call{
			{channel: "task.get", args: map[string]any{"id": ref.ID}},
			{channel: "task.getDependencies", args: map[string]any{"taskId": ref.ID}},
			{channel: "task.listComments", args: map[string]any{"taskId": ref.ID, "pageSize": 50}},
		}}, nil
	case KindWorktreeStatus:
		return plan{meta: readMeta(ref.Kind, "git.status", false, false), calls: []call{{channel: "git.status", args: map[string]any{"worktree": sel}}}}, nil
	case KindWorktreeDiff:
		if tools.IsSensitivePath(ref.Path, r.cfg.SensitivePathExtra) {
			return plan{}, errNotReadable
		}
		c := call{channel: "git.diff", args: map[string]any{"worktree": sel, "filePath": ref.Path, "staged": ref.Staged == "true"}}
		if ref.Base != "" {
			c = call{channel: "git.branchDiff", args: map[string]any{"worktree": sel, "baseRef": ref.Base, "filePath": ref.Path}}
		}
		return plan{meta: readMeta(ref.Kind, c.channel, true, false), untrusted: true, calls: []call{c}, text: true, mime: "text/x-diff"}, nil
	case KindWorktreeFile:
		if !r.cfg.FileEnabled || tools.IsSensitivePath(ref.Path, r.cfg.SensitivePathExtra) {
			return plan{}, errNotReadable
		}
		c := call{channel: "files.readPreview", preview: true, args: map[string]any{"worktreeId": ref.ID, "path": ref.Path, "maxBytes": minInt(r.cfg.MaxBytes, 64<<10)}}
		if ref.hasOffset {
			length := ref.Length
			if length == 0 {
				length = 64 << 10
			}
			c = call{channel: "files.readChunk", args: map[string]any{"worktreeId": ref.ID, "path": ref.Path, "offsetBytes": ref.Offset, "lengthBytes": length}}
		}
		return plan{meta: readMeta(ref.Kind, c.channel, true, false), untrusted: true, calls: []call{c}, text: true, mime: mimeForPath(ref.Path)}, nil
	case KindReview:
		c := call{channel: "github.project.workItemDetailsBySlug", args: map[string]any{"itemSlug": ref.Repo + "#" + ref.Number}}
		if ref.Provider == "gitlab" {
			iid, _ := strconv.Atoi(ref.Number)
			c = call{channel: "gitlab.workItemDetails", args: map[string]any{"repo": ref.Repo, "iid": iid, "itemType": "merge_request"}}
		}
		return plan{meta: readMeta(ref.Kind, c.channel, true, true), untrusted: true, calls: []call{c}}, nil
	}
	return plan{}, errBadURI
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// subscribable kinds: only those with a real event source. Worktree changes
// need git-gateway WatchWorktree behind a per-connection registry that the
// MCP edge does not own, and terminals need the BE-009 ring; neither exists.
func subscribable(k Kind) bool { return k == KindTask }

// decodeChunk turns a files.readChunk result (base64 content) into text.
func decodeChunk(obj map[string]any) (string, error) {
	enc, _ := obj["content"].(string)
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil || !utf8.Valid(raw) {
		return "", errBinary
	}
	s := string(raw)
	if tools.ContainsPrivateKey(s) {
		return "[redacted: private key material]", nil
	}
	return tools.RedactText(s), nil
}

var errBinary = errors.New("resources: binary content")

// textOf extracts the text of a text-rendered call result.
func textOf(c call, obj map[string]any) (string, bool, error) {
	switch {
	case c.preview:
		if bin, _ := obj["binary"].(bool); bin {
			return "", false, errBinary
		}
		s, _ := obj["text"].(string)
		trunc, _ := obj["truncated"].(bool)
		return s, trunc, nil
	case c.channel == "files.readChunk":
		s, err := decodeChunk(obj)
		return s, false, err
	}
	if s, ok := obj["unifiedDiff"].(string); ok {
		return s, false, nil
	}
	b, err := json.Marshal(obj) // diff shapes without unifiedDiff stay valid JSON
	return string(b), false, err
}

func mimeForPath(p string) string {
	switch strings.ToLower(p[strings.LastIndexByte(p, '.')+1:]) {
	case "md", "markdown":
		return "text/markdown"
	case "json":
		return "application/json"
	case "html", "htm":
		return "text/html"
	case "css":
		return "text/css"
	case "xml":
		return "application/xml"
	}
	return "text/plain"
}
