package domain

// TechStack is TechStackDetector's best-effort result — languages/
// frameworks inferred from marker-file presence, never validated further.
type TechStack struct {
	Languages  []string
	Frameworks []string
}
