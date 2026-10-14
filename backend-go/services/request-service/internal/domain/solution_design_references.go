package domain

// DesignReferences is what an agent with the project's knowledge graph says it read and found while proposing options.
// It is data to show a reviewer, never a gate: a graph that was absent or empty is a normal answer (Available=false).
type DesignReferences struct {
	Available bool `json:"available"`
	// Snapshot names the graph snapshot the answers came from, so a reader can tell how stale they may be.
	Snapshot  string   `json:"snapshot,omitempty"`
	Read      []string `json:"read,omitempty"`
	Gaps      []string `json:"gaps,omitempty"`
	Conflicts []string `json:"conflicts,omitempty"`
}

const (
	maxDesignSnapshotRunes = 120
	maxDesignReadItems     = 80
	maxDesignReadRunes     = 200
	maxDesignNoteItems     = 30
	maxDesignNoteRunes     = 1500
)

func (d *DesignReferences) validate() error {
	if d == nil {
		return nil
	}
	if runeLen(d.Snapshot) > maxDesignSnapshotRunes {
		return optionsErr("design_references.snapshot is longer than %d characters", maxDesignSnapshotRunes)
	}
	if err := checkDesignList("read", d.Read, maxDesignReadItems, maxDesignReadRunes); err != nil {
		return err
	}
	if err := checkDesignList("gaps", d.Gaps, maxDesignNoteItems, maxDesignNoteRunes); err != nil {
		return err
	}
	if err := checkDesignList("conflicts", d.Conflicts, maxDesignNoteItems, maxDesignNoteRunes); err != nil {
		return err
	}
	return nil
}

func checkDesignList(name string, items []string, maxItems, maxRunes int) error {
	if len(items) > maxItems {
		return optionsErr("design_references.%s has more than %d entries", name, maxItems)
	}
	for _, it := range items {
		if it == "" || runeLen(it) > maxRunes {
			return optionsErr("design_references.%s entries must be 1..%d characters", name, maxRunes)
		}
	}
	return nil
}
