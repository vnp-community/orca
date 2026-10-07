package domain

import (
	"testing"
)

func TestViewKind_CacheName_Bijection(t *testing.T) {
	expectedNames := map[ViewKind]string{
		ViewKindStructure:        "structure",
		ViewKindArchitecture:     "architecture",
		ViewKindFlows:            "flows",
		ViewKindFlow:             "flow",
		ViewKindSubgraph:         "subgraph",
		ViewKindImpact:           "impact",
		ViewKindSymbol:           "symbol",
		ViewKindRoutes:           "routes",
		ViewKindChangeOverlay:    "changeOverlay",
		ViewKindStatus:           "status",
		ViewKindClusters:         "clusters",
		ViewKindReadingOrder:     "readingOrder",
		ViewKindErd:              "erd",
		ViewKindStorage:          "storage",
		ViewKindDataflows:        "dataflows",
		ViewKindDataflow:         "dataflow",
		ViewKindFindings:         "findings",
		ViewKindContractDiff:     "contractDiff",
		ViewKindContractCatalog:  "contractCatalog",
		ViewKindRequirementTrace: "requirementTrace",
		ViewKindAiSummary:        "aiSummary",
	}

	if len(expectedNames) != 21 {
		t.Fatalf("expected 21 contract cache names, got %d", len(expectedNames))
	}

	seenNames := make(map[string]ViewKind)
	for vk, wantName := range expectedNames {
		gotName := vk.CacheName()
		if gotName != wantName {
			t.Errorf("ViewKind %d: CacheName() = %q, want %q", vk, gotName, wantName)
		}

		if existingVk, duplicate := seenNames[gotName]; duplicate {
			t.Errorf("duplicate cache name %q for ViewKinds %d and %d", gotName, existingVk, vk)
		}
		seenNames[gotName] = vk

		parsed, err := ParseViewKind(gotName)
		if err != nil {
			t.Errorf("ParseViewKind(%q) returned error: %v", gotName, err)
		}
		if parsed != vk {
			t.Errorf("ParseViewKind(%q) = %d, want %d", gotName, parsed, vk)
		}
	}

	// Unspecified returns "unspecified" and errors on parse
	if ViewKindUnspecified.CacheName() != "unspecified" {
		t.Errorf("unspecified cache name mismatch: got %s", ViewKindUnspecified.CacheName())
	}
	if _, err := ParseViewKind("unknownView"); err == nil {
		t.Error("expected error for unknown view kind, got nil")
	}
}
