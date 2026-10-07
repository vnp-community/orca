package domain

import "strings"

var gitNexusKindMap = map[string]SymbolKind{
	"function":  SymbolKindFunction,
	"method":    SymbolKindMethod,
	"type":      SymbolKindType,
	"interface": SymbolKindType,
	"class":     SymbolKindType,
	"struct":    SymbolKindType,
	"enum":      SymbolKindType,
	"variable":  SymbolKindValue,
	"constant":  SymbolKindValue,
	"property":  SymbolKindValue,
	"field":     SymbolKindValue,
	"file":      SymbolKindFile,
	"folder":    SymbolKindFolder,
	"route":     SymbolKindRoute,
	"component": SymbolKindComponent,
	"namespace": SymbolKindNamespace,
	"module":    SymbolKindNamespace,
	"package":   SymbolKindNamespace,
	"import":    SymbolKindImport,
	"cluster":   SymbolKindCluster,
	"flow":      SymbolKindFlow,
	"doc":       SymbolKindDoc,
	"section":   SymbolKindDoc,
}

var codeGraphKindMap = map[string]SymbolKind{
	"function":  SymbolKindFunction,
	"method":    SymbolKindMethod,
	"type":      SymbolKindType,
	"interface": SymbolKindType,
	"class":     SymbolKindType,
	"struct":    SymbolKindType,
	"enum":      SymbolKindType,
	"variable":  SymbolKindValue,
	"constant":  SymbolKindValue,
	"property":  SymbolKindValue,
	"field":     SymbolKindValue,
	"file":      SymbolKindFile,
	"folder":    SymbolKindFolder,
	"directory": SymbolKindFolder,
	"route":     SymbolKindRoute,
	"component": SymbolKindComponent,
	"namespace": SymbolKindNamespace,
	"module":    SymbolKindNamespace,
	"package":   SymbolKindNamespace,
	"import":    SymbolKindImport,
	"cluster":   SymbolKindCluster,
	"flow":      SymbolKindFlow,
	"doc":       SymbolKindDoc,
}

// KindFromGitNexusLabel maps a GitNexus node label to domain SymbolKind.
// Unknown labels return (SymbolKindValue, true).
func KindFromGitNexusLabel(label string) (SymbolKind, bool) {
	lower := strings.ToLower(label)
	if k, ok := gitNexusKindMap[lower]; ok {
		return k, false
	}
	return SymbolKindValue, true
}

// KindFromCodeGraphKind maps a CodeGraph node kind to domain SymbolKind.
// Unknown kinds return (SymbolKindValue, true).
func KindFromCodeGraphKind(kind string) (SymbolKind, bool) {
	lower := strings.ToLower(kind)
	if k, ok := codeGraphKindMap[lower]; ok {
		return k, false
	}
	return SymbolKindValue, true
}
