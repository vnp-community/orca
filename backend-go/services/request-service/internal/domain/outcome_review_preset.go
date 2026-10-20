package domain

// DrawPromptVersion belongs to every preset digest: a drawing made with other wording is another drawing.
const DrawPromptVersion = "outcome_review_draw/1"

// presetInputs is what a drawing of one section depends on besides the model: the part of the profile that changes the picture.
// Nothing about other sections is in it, so changing one section redraws only that section.
type presetInputs struct {
	Section  ReviewSection   `json:"section"`
	Variant  string          `json:"variant"`
	Spec     ResolvedSection `json:"spec"`
	Style    ResolvedStyle   `json:"style"`
	Language string          `json:"language"`
	Prompt   string          `json:"prompt"`
}

// PresetDigest is the key part of a drawing that stands for the profile. Sorting of maps is done by the canonical digest.
func (r ResolvedProfile) PresetDigest(section ReviewSection, variant string) (string, error) {
	spec := r.Sections[section]
	return DigestValue(presetInputs{Section: section, Variant: variant, Spec: spec, Style: r.Style, Language: r.Language, Prompt: DrawPromptVersion})
}
