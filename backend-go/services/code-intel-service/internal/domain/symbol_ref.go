package domain

import (
	"fmt"
	"strings"

	"golang.org/x/text/unicode/norm"
)

type SymbolKind int

const (
	SymbolKindUnspecified SymbolKind = iota
	SymbolKindFunction
	SymbolKindMethod
	SymbolKindType
	SymbolKindValue
	SymbolKindFile
	SymbolKindFolder
	SymbolKindRoute
	SymbolKindComponent
	SymbolKindNamespace
	SymbolKindImport
	SymbolKindCluster
	SymbolKindFlow
	SymbolKindDoc
)

func (k SymbolKind) String() string {
	switch k {
	case SymbolKindFunction:
		return "function"
	case SymbolKindMethod:
		return "method"
	case SymbolKindType:
		return "type"
	case SymbolKindValue:
		return "value"
	case SymbolKindFile:
		return "file"
	case SymbolKindFolder:
		return "folder"
	case SymbolKindRoute:
		return "route"
	case SymbolKindComponent:
		return "component"
	case SymbolKindNamespace:
		return "namespace"
	case SymbolKindImport:
		return "import"
	case SymbolKindCluster:
		return "cluster"
	case SymbolKindFlow:
		return "flow"
	case SymbolKindDoc:
		return "doc"
	default:
		return "unspecified"
	}
}

func SymbolKindFromString(s string) SymbolKind {
	switch strings.ToLower(s) {
	case "function":
		return SymbolKindFunction
	case "method":
		return SymbolKindMethod
	case "type":
		return SymbolKindType
	case "value":
		return SymbolKindValue
	case "file":
		return SymbolKindFile
	case "folder":
		return SymbolKindFolder
	case "route":
		return SymbolKindRoute
	case "component":
		return SymbolKindComponent
	case "namespace":
		return SymbolKindNamespace
	case "import":
		return SymbolKindImport
	case "cluster":
		return SymbolKindCluster
	case "flow":
		return SymbolKindFlow
	case "doc":
		return SymbolKindDoc
	default:
		return SymbolKindUnspecified
	}
}

type SymbolRef struct {
	Key           string
	Kind          SymbolKind
	NativeKind    string
	Name          string
	QualifiedName string
	FilePath      string
	StartLine     int32
	EndLine       int32
	GitNexusID    string
	CodeGraphID   string
	Language      string
	Signature     string
	IsExported    bool
	Docstring     string
	Ordinal       int
}

type SymbolEdge struct {
	FromKey    string
	ToKey      string
	Kind       EdgeKind
	Sources    []string
	Confidence float32
}

// NewSymbolKey builds a deterministic symbol key in the format:
// "<kind>:<filePath>:<qn||name>" (NFC normalized, case-preserved).
func NewSymbolKey(kind SymbolKind, filePath, qualifiedName, name string) string {
	target := qualifiedName
	if target == "" {
		target = name
	}

	// Double colon syntax for workspace-level / virtual nodes like cluster/flow
	if filePath == "" && (kind == SymbolKindCluster || kind == SymbolKindFlow) {
		return norm.NFC.String(fmt.Sprintf("%s::%s", kind.String(), target))
	}

	return norm.NFC.String(fmt.Sprintf("%s:%s:%s", kind.String(), filePath, target))
}
