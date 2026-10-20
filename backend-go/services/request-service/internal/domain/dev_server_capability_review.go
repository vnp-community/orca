package domain

// ReviewContextSkill is the one skill a context run needs: the read tools of the knowledge graph. The write-back skill is never used.
const ReviewContextSkill = "tdd-knowledge"

// ReviewContextGap is one reason a dev server cannot run the context run of an Outcome Review.
type ReviewContextGap struct {
	Code    string
	Message string
}

// ReviewContextGaps lists why the agent behind the profile cannot run a context run; empty means it can. reviewContext is
// the knowledge hub, an agent that enforces read-only mode itself, and the tdd-knowledge skill together: the hub for the graph,
// enforced read-only so the run cannot write (a prompt that asks nicely is not enough for a run that reads a whole repository),
// the skill so the agent uses the read tools instead of guessing.
func ReviewContextGaps(c DevServerCapability) []ReviewContextGap {
	var gaps []ReviewContextGap
	if !SupportsKnowledgeHub(c) {
		gaps = append(gaps, ReviewContextGap{Code: CodeReviewAgentTooOld, Message: "the dev server agent cannot load the knowledge hub"})
	}
	if !c.HasFeature(FeatureReadonly) {
		gaps = append(gaps, ReviewContextGap{Code: CodeReviewAgentTooOld, Message: "the dev server agent cannot enforce read-only mode"})
	}
	if c.SkillsKnown {
		if st, ok := c.Skills[ReviewContextSkill]; !ok || !st.Installed {
			gaps = append(gaps, ReviewContextGap{Code: CodeReviewSkillMissing, Message: "the " + ReviewContextSkill + " skill is not installed on the dev server"})
		}
	}
	return dedupeGaps(gaps)
}

// SupportsReviewContext reports whether a context run can start on this agent.
func SupportsReviewContext(c DevServerCapability) bool { return len(ReviewContextGaps(c)) == 0 }

// ReviewSkillMissing reports whether the agent said, and not merely failed to say, that the skill is not there.
func ReviewSkillMissing(c DevServerCapability) bool {
	if !c.SkillsKnown {
		return false
	}
	st, ok := c.Skills[ReviewContextSkill]
	return !ok || !st.Installed
}

func dedupeGaps(in []ReviewContextGap) []ReviewContextGap {
	seen := map[string]bool{}
	var out []ReviewContextGap
	for _, g := range in {
		if k := g.Code + g.Message; !seen[k] {
			seen[k] = true
			out = append(out, g)
		}
	}
	return out
}
