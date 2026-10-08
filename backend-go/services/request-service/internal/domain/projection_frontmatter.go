package domain

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Projection violation codes. They are positional (Line) rather than JSON-pointer based.
const (
	CodeProjectionTooLarge            = "PROJECTION_TOO_LARGE"
	CodeProjectionControlChar         = "PROJECTION_CONTROL_CHAR"
	CodeProjectionFrontmatterMissing  = "PROJECTION_FRONTMATTER_MISSING"
	CodeProjectionFrontmatterTooLarge = "PROJECTION_FRONTMATTER_TOO_LARGE"
	CodeProjectionFrontmatterInvalid  = "PROJECTION_FRONTMATTER_INVALID"
	CodeProjectionFrontmatterField    = "PROJECTION_FRONTMATTER_FIELD_REQUIRED"
	CodeProjectionYAMLUnsafe          = "PROJECTION_YAML_UNSAFE"
	CodeProjectionRegionMissing       = "PROJECTION_REGION_MISSING"
	CodeProjectionRegionMalformed     = "PROJECTION_REGION_MALFORMED"
	CodeProjectionJSONBlock           = "PROJECTION_ORCA_JSON_BLOCK"
	CodeProjectionDigestMismatch      = "PROJECTION_DIGEST_MISMATCH"
	CodeProjectionKindMismatch        = "PROJECTION_KIND_MISMATCH"

	MaxProjectionBytes   = 1 << 20
	MaxFrontmatterBytes  = 64 * 1024
	frontmatterDelimiter = "---"
)

// GeneratedBy mirrors the generated_by flow mapping of the frontmatter.
type GeneratedBy struct {
	Kind  string `yaml:"kind,omitempty"`
	Tool  string `yaml:"tool,omitempty"`
	Model string `yaml:"model,omitempty"`
	Run   string `yaml:"run,omitempty"`
}

// Frontmatter is the only YAML the projection accepts (CR-REQ-027 section 2.7 rule 1).
type Frontmatter struct {
	OrcaSchema  int          `yaml:"orca_schema"`
	Kind        ArtifactKind `yaml:"kind"`
	ID          string       `yaml:"id"`
	Request     string       `yaml:"request,omitempty"`
	Status      string       `yaml:"status,omitempty"`
	Supersedes  string       `yaml:"supersedes,omitempty"`
	Digest      string       `yaml:"digest,omitempty"`
	GeneratedBy GeneratedBy  `yaml:"generated_by,omitempty"`
}

var (
	yamlAliasOrAnchor = regexp.MustCompile(`(^|[\s\[{,:])[&*][A-Za-z0-9_-]`)
	yamlTag           = regexp.MustCompile(`(^|[\s\[{,:])![!A-Za-z<]`)
	yamlMergeKey      = regexp.MustCompile(`(^|\s)<<\s*:`)
	yamlErrLine       = regexp.MustCompile(`line (\d+)`)
	quotedYAML        = regexp.MustCompile(`"(?:[^"\\]|\\.)*"|'(?:[^']|'')*'`)
)

// scanYAMLUnsafe rejects anchors, aliases, tags and merge keys before the decoder sees them:
// yaml.v3 has no switch to disable them and aliases have a history of denial-of-service.
func scanYAMLUnsafe(lines []string, firstLine int) []Violation {
	var out []Violation
	for i, l := range lines {
		bare := quotedYAML.ReplaceAllString(l, `""`)
		if c := strings.Index(bare, " #"); c >= 0 {
			bare = bare[:c]
		}
		if yamlAliasOrAnchor.MatchString(bare) || yamlTag.MatchString(bare) || yamlMergeKey.MatchString(bare) {
			out = append(out, Violation{Line: firstLine + i, Code: CodeProjectionYAMLUnsafe, Message: "anchors, aliases, tags and merge keys are not allowed in the frontmatter"})
		}
	}
	return out
}

// ParseFrontmatter reads the leading ---/--- block of md (already normalised to \n) and returns the rest.
func ParseFrontmatter(md string) (Frontmatter, string, []Violation) {
	lines := strings.Split(md, "\n")
	if len(lines) == 0 || strings.TrimRight(lines[0], " \t") != frontmatterDelimiter {
		return Frontmatter{}, md, []Violation{{Line: 1, Code: CodeProjectionFrontmatterMissing, Message: "the first line must be ---"}}
	}
	end, size := -1, len(lines[0])+1
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], " \t") == frontmatterDelimiter {
			end = i
			break
		}
		size += len(lines[i]) + 1
		if size > MaxFrontmatterBytes {
			return Frontmatter{}, md, []Violation{{Line: i + 1, Code: CodeProjectionFrontmatterTooLarge, Message: fmt.Sprintf("the frontmatter is larger than %d bytes", MaxFrontmatterBytes)}}
		}
	}
	if end < 0 {
		return Frontmatter{}, md, []Violation{{Line: 1, Code: CodeProjectionFrontmatterMissing, Message: "the frontmatter is not closed by ---"}}
	}
	body := lines[1:end]
	if vs := scanYAMLUnsafe(body, 2); len(vs) > 0 {
		return Frontmatter{}, md, vs
	}
	var fm Frontmatter
	dec := yaml.NewDecoder(strings.NewReader(strings.Join(body, "\n")))
	dec.KnownFields(true)
	if err := dec.Decode(&fm); err != nil {
		line := 1
		if m := yamlErrLine.FindStringSubmatch(err.Error()); m != nil {
			fmt.Sscanf(m[1], "%d", &line)
			line++ // the decoder counts from the first line after the opening ---
		}
		return Frontmatter{}, md, []Violation{{Line: line, Code: CodeProjectionFrontmatterInvalid, Message: strings.TrimPrefix(err.Error(), "yaml: ")}}
	}
	var out []Violation
	req := func(key string, missing bool) {
		if missing {
			out = append(out, Violation{Line: keyLine(body, key), Code: CodeProjectionFrontmatterField, Message: key + " is required"})
		}
	}
	req("orca_schema", fm.OrcaSchema == 0)
	req("kind", fm.Kind == "")
	req("id", strings.TrimSpace(fm.ID) == "")
	rest := strings.Join(lines[end+1:], "\n")
	return fm, rest, out
}

// keyLine finds the absolute line of key in the frontmatter body, or the opening delimiter's line.
func keyLine(body []string, key string) int {
	for i, l := range body {
		if strings.HasPrefix(l, key+":") {
			return i + 2
		}
	}
	return 1
}
