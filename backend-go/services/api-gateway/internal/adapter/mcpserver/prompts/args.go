package prompts

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
)

const maxArgLen = 200

var (
	uuidRe     = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	numberRe   = regexp.MustCompile(`^[0-9]{1,9}$`)
	issueRefRe = regexp.MustCompile(`^[A-Za-z0-9_.:/#-]{1,200}$`)
	repoRe     = regexp.MustCompile(`^[A-Za-z0-9_.\-]{1,100}(/[A-Za-z0-9_.\-]{1,100}){1,9}$`)
	slugRe     = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)
)

func invalid(format string, a ...any) error {
	return &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: fmt.Sprintf(format, a...)}
}

// validateBuiltinArgs checks required/extra arguments, length, UTF-8, control
// characters and the per-argument shape. The error names the argument only,
// never echoes its value.
func validateBuiltinArgs(def builtinDef, args map[string]string) (map[string]string, error) {
	known := map[string]argSpec{}
	for _, a := range def.Args {
		known[a.Name] = a
	}
	for name := range args {
		if _, ok := known[name]; !ok {
			return nil, invalid("unknown argument: %s", safeName(name))
		}
	}
	out := map[string]string{}
	for _, a := range def.Args {
		v, present := args[a.Name]
		if !present || v == "" {
			if a.Required {
				return nil, invalid("missing required argument: %s", a.Name)
			}
			out[a.Name] = ""
			continue
		}
		if err := checkValue(a, v); err != nil {
			return nil, invalid("invalid argument: %s", a.Name)
		}
		out[a.Name] = canonical(a, v)
	}
	return out, nil
}

func canonical(a argSpec, v string) string {
	if a.Kind == argUUID {
		return strings.ToLower(v)
	}
	return v
}

func safeName(s string) string {
	if len(s) > 40 || !slugRe.MatchString(strings.ToLower(s)) {
		return "(invalid name)"
	}
	return s
}

func checkValue(a argSpec, v string) error {
	if len(v) > maxArgLen || !utf8.ValidString(v) {
		return errBadArg
	}
	for _, r := range v {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return errBadArg
		}
	}
	ok := false
	switch a.Kind {
	case argUUID:
		ok = uuidRe.MatchString(v)
	case argNumber:
		ok = numberRe.MatchString(v)
	case argIssueRef:
		ok = issueRefRe.MatchString(v)
	case argRepo:
		ok = repoRe.MatchString(v) && !hasDotSegment(v)
	case argSlug:
		ok = slugRe.MatchString(v)
	case argEnum:
		for _, e := range a.Enum {
			ok = ok || e == v
		}
	default:
		ok = true
	}
	if !ok {
		return errBadArg
	}
	return nil
}

var errBadArg = fmt.Errorf("bad argument")

func hasDotSegment(v string) bool {
	for _, s := range strings.Split(v, "/") {
		if s == "." || s == ".." {
			return true
		}
	}
	return false
}
