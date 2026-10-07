package domain

import (
	"fmt"
	"sort"
)

// MergeReport summarizes the outcome of merging two symbol sets.
type MergeReport struct {
	PrimaryMatches   int
	SecondaryMatches int
	Unmatched        int
	Disagreements    []string
}

// MergeSymbolSets merges symbol sets from two sources (GitNexus and CodeGraph).
// Primary matching is done on canonical Key.
// Secondary matching is done on (FilePath, Name) within ±2 lines for compatible kinds
// when both sources have exactly one candidate.
// Results are deterministically sorted by Key.
func MergeSymbolSets(a, b []SymbolRef) ([]SymbolRef, MergeReport) {
	var report MergeReport

	// Index primary matches by key
	bByKey := make(map[string][]int, len(b))
	for i, ref := range b {
		bByKey[ref.Key] = append(bByKey[ref.Key], i)
	}

	usedA := make([]bool, len(a))
	usedB := make([]bool, len(b))
	var merged []SymbolRef

	// 1. Primary matching by Key
	for i, refA := range a {
		indices, ok := bByKey[refA.Key]
		if ok && len(indices) > 0 {
			// Take the first unused matching candidate
			for _, bIdx := range indices {
				if !usedB[bIdx] {
					usedA[i] = true
					usedB[bIdx] = true
					report.PrimaryMatches++

					refB := b[bIdx]
					if lineDiff(refA.StartLine, refB.StartLine) > 2 {
						report.Disagreements = append(report.Disagreements,
							fmt.Sprintf("sources_disagree:%s (line %d vs %d)", refA.Key, refA.StartLine, refB.StartLine))
					}

					m := mergePair(refA, refB)
					merged = append(merged, m)
					break
				}
			}
		}
	}

	// 2. Secondary matching for remaining candidates
	// Group unmatched by (FilePath, Name)
	type fileNamedKey struct {
		FilePath string
		Name     string
	}

	candA := make(map[fileNamedKey][]int)
	for i, ref := range a {
		if !usedA[i] && ref.FilePath != "" && ref.Name != "" {
			k := fileNamedKey{FilePath: ref.FilePath, Name: ref.Name}
			candA[k] = append(candA[k], i)
		}
	}

	candB := make(map[fileNamedKey][]int)
	for j, ref := range b {
		if !usedB[j] && ref.FilePath != "" && ref.Name != "" {
			k := fileNamedKey{FilePath: ref.FilePath, Name: ref.Name}
			candB[k] = append(candB[k], j)
		}
	}

	for k, aIdxs := range candA {
		bIdxs, exists := candB[k]
		if exists && len(aIdxs) == 1 && len(bIdxs) == 1 {
			aIdx := aIdxs[0]
			bIdx := bIdxs[0]
			refA := a[aIdx]
			refB := b[bIdx]

			if isKindCompatible(refA.Kind, refB.Kind) && lineDiff(refA.StartLine, refB.StartLine) <= 2 {
				usedA[aIdx] = true
				usedB[bIdx] = true
				report.SecondaryMatches++

				m := mergePair(refA, refB)
				merged = append(merged, m)
			}
		}
	}

	// 3. Collect unmatched nodes
	for i, ref := range a {
		if !usedA[i] {
			merged = append(merged, ref)
			report.Unmatched++
		}
	}
	for j, ref := range b {
		if !usedB[j] {
			merged = append(merged, ref)
			report.Unmatched++
		}
	}

	// Sort merged slice by Key deterministically
	sort.Slice(merged, func(i, j int) bool {
		if merged[i].Key != merged[j].Key {
			return merged[i].Key < merged[j].Key
		}
		if merged[i].StartLine != merged[j].StartLine {
			return merged[i].StartLine < merged[j].StartLine
		}
		if merged[i].GitNexusID != merged[j].GitNexusID {
			return merged[i].GitNexusID < merged[j].GitNexusID
		}
		return merged[i].CodeGraphID < merged[j].CodeGraphID
	})

	sort.Strings(report.Disagreements)

	return merged, report
}

// MergeRoutes concatenates route nodes from two sources without cross-source merging.
// Deduplication is performed by source identifier and key.
func MergeRoutes(a, b []SymbolRef) []SymbolRef {
	seen := make(map[string]bool)
	var result []SymbolRef

	appendIfNew := func(ref SymbolRef) {
		srcID := ref.GitNexusID
		if srcID == "" {
			srcID = ref.CodeGraphID
		}
		dedupKey := fmt.Sprintf("%s|%s|%d", srcID, ref.Key, ref.StartLine)
		if !seen[dedupKey] {
			seen[dedupKey] = true
			result = append(result, ref)
		}
	}

	for _, r := range a {
		appendIfNew(r)
	}
	for _, r := range b {
		appendIfNew(r)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Key != result[j].Key {
			return result[i].Key < result[j].Key
		}
		if result[i].GitNexusID != result[j].GitNexusID {
			return result[i].GitNexusID < result[j].GitNexusID
		}
		return result[i].CodeGraphID < result[j].CodeGraphID
	})

	return result
}

// MergeEdges unions edges from two sources, matching on (FromKey, ToKey, Kind).
// Sources are unioned and sorted, confidence is maxed, and output is deterministically sorted.
func MergeEdges(a, b []SymbolEdge) []SymbolEdge {
	type edgeKey struct {
		from string
		to   string
		kind EdgeKind
	}

	mergedMap := make(map[edgeKey]*SymbolEdge)

	add := func(e SymbolEdge) {
		k := edgeKey{from: e.FromKey, to: e.ToKey, kind: e.Kind}
		existing, found := mergedMap[k]
		if !found {
			srcCopy := make([]string, len(e.Sources))
			copy(srcCopy, e.Sources)
			mergedMap[k] = &SymbolEdge{
				FromKey:    e.FromKey,
				ToKey:      e.ToKey,
				Kind:       e.Kind,
				Sources:    srcCopy,
				Confidence: e.Confidence,
			}
			return
		}

		if e.Confidence > existing.Confidence {
			existing.Confidence = e.Confidence
		}
		existing.Sources = unionStrings(existing.Sources, e.Sources)
	}

	for _, e := range a {
		add(e)
	}
	for _, e := range b {
		add(e)
	}

	result := make([]SymbolEdge, 0, len(mergedMap))
	for _, e := range mergedMap {
		sort.Strings(e.Sources)
		result = append(result, *e)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].FromKey != result[j].FromKey {
			return result[i].FromKey < result[j].FromKey
		}
		if result[i].ToKey != result[j].ToKey {
			return result[i].ToKey < result[j].ToKey
		}
		return result[i].Kind < result[j].Kind
	})

	return result
}

func unionStrings(s1, s2 []string) []string {
	seen := make(map[string]bool, len(s1)+len(s2))
	var out []string
	for _, s := range s1 {
		if !seen[s] && s != "" {
			seen[s] = true
			out = append(out, s)
		}
	}
	for _, s := range s2 {
		if !seen[s] && s != "" {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func lineDiff(x, y int32) int32 {
	if x > y {
		return x - y
	}
	return y - x
}

func isKindCompatible(ka, kb SymbolKind) bool {
	if ka == kb {
		return true
	}
	isCallable := func(k SymbolKind) bool {
		return k == SymbolKindFunction || k == SymbolKindMethod || k == SymbolKindValue
	}
	return isCallable(ka) && isCallable(kb)
}

// mergePair combines two SymbolRefs into a single canonical SymbolRef.
// GitNexus fields take precedence for line numbers and key; CodeGraph takes precedence
// for signature, language, and docstring.
func mergePair(r1, r2 SymbolRef) SymbolRef {
	var gn, cg SymbolRef
	if r1.GitNexusID != "" && r2.CodeGraphID != "" {
		gn, cg = r1, r2
	} else if r2.GitNexusID != "" && r1.CodeGraphID != "" {
		gn, cg = r2, r1
	} else if r1.GitNexusID != "" {
		gn, cg = r1, r2
	} else if r2.GitNexusID != "" {
		gn, cg = r2, r1
	} else {
		if r1.Key <= r2.Key {
			gn, cg = r1, r2
		} else {
			gn, cg = r2, r1
		}
	}

	startLine := gn.StartLine
	if startLine <= 0 {
		startLine = cg.StartLine
	}
	endLine := gn.EndLine
	if endLine <= 0 {
		endLine = cg.EndLine
	}

	signature := cg.Signature
	if signature == "" {
		signature = gn.Signature
	}

	language := cg.Language
	if language == "" {
		language = gn.Language
	}

	docstring := cg.Docstring
	if docstring == "" {
		docstring = gn.Docstring
	}

	nativeKind := gn.NativeKind
	if gn.NativeKind != "" && cg.NativeKind != "" && gn.NativeKind != cg.NativeKind {
		nativeKind = gn.NativeKind + "|" + cg.NativeKind
	} else if nativeKind == "" {
		nativeKind = cg.NativeKind
	}

	gitNexusID := gn.GitNexusID
	if gitNexusID == "" {
		gitNexusID = cg.GitNexusID
	}
	codeGraphID := cg.CodeGraphID
	if codeGraphID == "" {
		codeGraphID = gn.CodeGraphID
	}

	key := gn.Key
	if key == "" {
		key = cg.Key
	}

	kind := gn.Kind
	if kind == SymbolKindUnspecified {
		kind = cg.Kind
	}

	name := gn.Name
	if name == "" {
		name = cg.Name
	}

	qualifiedName := gn.QualifiedName
	if qualifiedName == "" {
		qualifiedName = cg.QualifiedName
	}

	filePath := gn.FilePath
	if filePath == "" {
		filePath = cg.FilePath
	}

	ordinal := gn.Ordinal
	if ordinal <= 0 {
		ordinal = cg.Ordinal
	}

	return SymbolRef{
		Key:           key,
		Kind:          kind,
		NativeKind:    nativeKind,
		Name:          name,
		QualifiedName: qualifiedName,
		FilePath:      filePath,
		StartLine:     startLine,
		EndLine:       endLine,
		GitNexusID:    gitNexusID,
		CodeGraphID:   codeGraphID,
		Language:      language,
		Signature:     signature,
		IsExported:    gn.IsExported || cg.IsExported,
		Docstring:     docstring,
		Ordinal:       ordinal,
	}
}
