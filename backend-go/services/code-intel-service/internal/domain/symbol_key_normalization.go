package domain

import (
	"fmt"
	"path"
	"strconv"
	"strings"
)

type GitNexusNode struct {
	ID            string
	Label         string
	FilePath      string
	QualifiedName string
	Name          string
	LineBase      int
	StartLine     int
	EndLine       int
	Language      string
	Signature     string
}

type CodeGraphNode struct {
	ID            string
	Kind          string
	FilePath      string
	QualifiedName string
	Name          string
	StartLine     int
	EndLine       int
	Language      string
	Signature     string
}

// SymbolRefFromGitNexus converts a raw GitNexus node to a canonical SymbolRef.
// It strips Label:/FilePath: prefixes, strips #<arity> while saving Ordinal,
// and adjusts 0-based lines (+1) when lineBase != 1.
func SymbolRefFromGitNexus(n GitNexusNode, lineBase int) SymbolRef {
	kind, _ := KindFromGitNexusLabel(n.Label)

	name := n.Name
	qn := n.QualifiedName

	// Clean prefixes like "Method:path:name"
	if strings.HasPrefix(qn, n.Label+":") {
		qn = strings.TrimPrefix(qn, n.Label+":")
	}
	if n.FilePath != "" && strings.HasPrefix(qn, n.FilePath+":") {
		qn = strings.TrimPrefix(qn, n.FilePath+":")
	}

	// Parse and strip #<arity>
	var ordinal int
	if idx := strings.LastIndex(qn, "#"); idx != -1 {
		if ord, err := strconv.Atoi(qn[idx+1:]); err == nil {
			ordinal = ord
			qn = qn[:idx]
		}
	}
	if idx := strings.LastIndex(name, "#"); idx != -1 {
		name = name[:idx]
	}

	startLine := int32(n.StartLine)
	endLine := int32(n.EndLine)
	if lineBase != 1 {
		startLine++
		if endLine > 0 {
			endLine++
		}
	}

	if kind == SymbolKindDoc && startLine > 0 {
		anchor := fmt.Sprintf("L%d:", startLine)
		if !strings.HasPrefix(qn, anchor) {
			qn = anchor + qn
		}
	}

	key := NewSymbolKey(kind, n.FilePath, qn, name)

	return SymbolRef{
		Key:           key,
		Kind:          kind,
		NativeKind:    n.Label,
		Name:          name,
		QualifiedName: qn,
		FilePath:      n.FilePath,
		StartLine:     startLine,
		EndLine:       endLine,
		GitNexusID:    n.ID,
		Language:      n.Language,
		Signature:     n.Signature,
		Ordinal:       ordinal,
	}
}

// SymbolRefFromCodeGraph converts a raw CodeGraph node to a canonical SymbolRef.
// It converts "::" to "." in qualified names and removes duplicate file prefix.
func SymbolRefFromCodeGraph(n CodeGraphNode) SymbolRef {
	kind, _ := KindFromCodeGraphKind(n.Kind)

	qn := n.QualifiedName
	if n.FilePath != "" {
		base := path.Base(n.FilePath)
		if strings.HasPrefix(qn, n.FilePath+"::") {
			qn = strings.TrimPrefix(qn, n.FilePath+"::")
		} else if strings.HasPrefix(qn, base+"::") {
			qn = strings.TrimPrefix(qn, base+"::")
		}
	}
	qn = strings.ReplaceAll(qn, "::", ".")
	name := n.Name

	startLine := int32(n.StartLine)
	endLine := int32(n.EndLine)
	if kind == SymbolKindDoc && startLine > 0 {
		anchor := fmt.Sprintf("L%d:", startLine)
		if !strings.HasPrefix(qn, anchor) {
			qn = anchor + qn
		}
	}

	key := NewSymbolKey(kind, n.FilePath, qn, name)

	return SymbolRef{
		Key:           key,
		Kind:          kind,
		NativeKind:    n.Kind,
		Name:          name,
		QualifiedName: qn,
		FilePath:      n.FilePath,
		StartLine:     startLine,
		EndLine:       endLine,
		CodeGraphID:   n.ID,
		Language:      n.Language,
		Signature:     n.Signature,
	}
}

// ResolveKeyCollisions dispatches distinct keys when multiple symbols share the exact key.
// It appends "#<arity>" if ordinal is present, otherwise "#L<startLine>".
func ResolveKeyCollisions(refs []SymbolRef) []SymbolRef {
	counts := make(map[string]int, len(refs))
	for _, r := range refs {
		counts[r.Key]++
	}

	result := make([]SymbolRef, len(refs))
	for i, r := range refs {
		if counts[r.Key] > 1 {
			if r.Ordinal > 0 {
				r.Key = fmt.Sprintf("%s#%d", r.Key, r.Ordinal)
			} else if r.StartLine > 0 {
				r.Key = fmt.Sprintf("%s#L%d", r.Key, r.StartLine)
			}
		}
		result[i] = r
	}
	return result
}

// ValidateAgentSymbolRef validates and normalizes an agent-supplied SymbolRef.
// If the key diverges from backend canonical derivation, the backend key takes precedence
// and keyMismatch is set to true.
func ValidateAgentSymbolRef(in SymbolRef, workspaceRoot string, plat Platform) (SymbolRef, bool, error) {
	normPath, err := NormalizeRepoPath(in.FilePath, workspaceRoot, plat)
	if err != nil && in.FilePath != "" {
		return in, false, err
	}
	if in.FilePath != "" {
		in.FilePath = normPath
	}

	canonicalKey := NewSymbolKey(in.Kind, in.FilePath, in.QualifiedName, in.Name)
	mismatch := (in.Key != canonicalKey)
	in.Key = canonicalKey
	return in, mismatch, nil
}
