package wscompat

import (
	"strings"
	"unicode"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// codeIntelEnumWireOverrides provides custom string mappings for enums where
// standard camel/lower casing does not meet the UI-API contract (PQ-32).
// Risk is explicitly uppercase in the contract (LOW, MEDIUM, HIGH, CRITICAL, UNKNOWN).
var codeIntelEnumWireOverrides = map[protoreflect.FullName]map[protoreflect.EnumNumber]string{
	"orca.codeintel.v1.Risk": {
		0: "UNKNOWN",
		1: "LOW",
		2: "MEDIUM",
		3: "HIGH",
		4: "CRITICAL",
		5: "UNKNOWN",
	},
	"orca.codeintel.v1.IndexScope": {
		0: "unresolved",
		1: "exact",
		2: "repo_root",
		3: "stale",
		4: "unresolved",
	},
}

func enumNameToSnake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteRune(r)
		}
	}
	return strings.ToUpper(b.String())
}

// enumWire converts an enum descriptor and value number to its wire format.
// Strips prefix, lowercases (except overrides), and maps 0/unspecified/unrecognized to "unknown".
func enumWire(ed protoreflect.EnumDescriptor, number protoreflect.EnumNumber) string {
	if ed == nil {
		return "unknown"
	}
	if overrides, ok := codeIntelEnumWireOverrides[ed.FullName()]; ok {
		if val, exists := overrides[number]; exists {
			return val
		}
	}

	vd := ed.Values().ByNumber(number)
	if vd == nil || number == 0 {
		return "unknown"
	}

	name := string(vd.Name())
	if strings.HasSuffix(name, "_UNSPECIFIED") {
		return "unknown"
	}

	prefix := enumNameToSnake(string(ed.Name())) + "_"
	if strings.HasPrefix(name, prefix) {
		name = strings.TrimPrefix(name, prefix)
	}

	lowered := strings.ToLower(name)
	if lowered == "unspecified" {
		return "unknown"
	}
	return lowered
}
