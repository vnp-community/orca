package domain

import (
	"errors"
	"testing"
)

func TestEvaluateCondition_SimpleComparisons(t *testing.T) {
	ctx := map[string]string{"status": "active", "count": "5"}

	tests := []struct {
		name string
		expr string
		want bool
	}{
		{"eq true", `status == "active"`, true},
		{"eq false", `status == "inactive"`, false},
		{"neq true", `status != "inactive"`, true},
		{"neq false", `status != "active"`, false},
		{"single-quoted string", `status == 'active'`, true},
		{"bare word value", `status == active`, true},
		{"missing key resolves empty", `missing == ""`, true},
		{"missing key not equal to non-empty", `missing == present`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EvaluateCondition(tt.expr, ctx)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("EvaluateCondition(%q) = %v, want %v", tt.expr, got, tt.want)
			}
		})
	}
}

func TestEvaluateCondition_AndOr(t *testing.T) {
	ctx := map[string]string{"status": "active", "count": "5", "role": "admin"}

	tests := []struct {
		name string
		expr string
		want bool
	}{
		{"and both true", `status == "active" && count == "5"`, true},
		{"and one false", `status == "active" && count == "9"`, false},
		{"or one true", `status == "inactive" || count == "5"`, true},
		{"or both false", `status == "inactive" || count == "9"`, false},
		{
			// && binds tighter than ||: this reads as
			// (status == "x" || status == "active") && role == "admin"... no —
			// actually per our grammar (no parens): a || (b && c) since || is
			// the outermost combinator and && groups the term immediately
			// following it.
			"precedence: and binds tighter than or",
			`status == "nope" || count == "5" && role == "admin"`,
			true,
		},
		{
			"precedence: and binds tighter than or (false case)",
			`status == "nope" || count == "5" && role == "guest"`,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EvaluateCondition(tt.expr, ctx)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("EvaluateCondition(%q) = %v, want %v", tt.expr, got, tt.want)
			}
		})
	}
}

func TestEvaluateCondition_UnparseableInputFailsSafe(t *testing.T) {
	tests := []string{
		"",
		"status ==",
		"status === active",
		"status == active &&",
		"status == active && && role == admin",
		"status @ active",
		`status == "unterminated`,
		"status == active role == admin", // missing && / ||
	}
	for _, expr := range tests {
		t.Run(expr, func(t *testing.T) {
			got, err := EvaluateCondition(expr, map[string]string{"status": "active"})
			if err == nil {
				t.Fatalf("expected an error for unparseable expression %q", expr)
			}
			if !errors.Is(err, ErrConditionSyntax) {
				t.Errorf("expected error to wrap ErrConditionSyntax, got %v", err)
			}
			if got != false {
				t.Errorf("expected fail-safe false result, got %v", got)
			}
		})
	}
}

func TestEvaluateCondition_NoEvalNoInjection(t *testing.T) {
	// A closed grammar rejects anything resembling code injection outright
	// — there is no path from user input to Go code execution, matching
	// TS's fix for the new Function()-based evaluator's code-injection risk
	// (§9).
	dangerous := []string{
		`status == "x"); os.Exit(1); (`,
		"`rm -rf /`",
		"${status}",
		"status == \"a\" ; DROP TABLE workflow.executions;",
	}
	for _, expr := range dangerous {
		t.Run(expr, func(t *testing.T) {
			if _, err := EvaluateCondition(expr, nil); err == nil {
				t.Errorf("expected %q to be rejected as unparseable", expr)
			}
		})
	}
}
