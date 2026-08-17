package domain

import (
	"errors"
	"fmt"
)

// EvaluateCondition evaluates a Condition step's boolean expression against
// a flat key-value context (the accumulated outputs of earlier steps, per
// workflow-service.md §4/§9). It is a closed, fixed grammar — comparisons
// (==, !=) combined with && / || — deliberately with NO eval, NO
// reflection, and NO arbitrary function calls, matching TS's already-
// sandboxed evaluateSafeCondition (§9: "condition's evaluator stays
// sandboxed").
//
// Grammar (&& binds tighter than ||, left-associative, no parentheses —
// kept intentionally minimal, not a general expression language):
//
//	expr       := andExpr ( '||' andExpr )*
//	andExpr    := comparison ( '&&' comparison )*
//	comparison := IDENT ( '==' | '!=' ) operand
//	operand    := IDENT | STRING
//
// The left-hand side of a comparison is always looked up in ctx (a missing
// key resolves to "", not an error — matches a step referencing an output
// field that simply wasn't produced). The right-hand side is a literal:
// either a quoted string ('...' or "...") or a bare word compared verbatim.
//
// On any unparseable input this returns (false, err) rather than panicking
// or silently guessing — "fail-safe-false on unparseable input", per §9.
// Callers that want the TS-matching fail-safe behavior (never bubble the
// parse error up as a hard failure) should treat a non-nil error as "false"
// while still surfacing it for observability — see
// adapter/stepexecutors.ConditionExecutor.
func EvaluateCondition(expr string, ctx map[string]string) (bool, error) {
	toks, err := tokenizeCondition(expr)
	if err != nil {
		return false, err
	}
	p := &conditionParser{tokens: toks, ctx: ctx}
	result, err := p.parseOr()
	if err != nil {
		return false, err
	}
	if p.pos != len(p.tokens) {
		return false, fmt.Errorf("%w: unexpected trailing input at token %d", ErrConditionSyntax, p.pos)
	}
	return result, nil
}

// ErrConditionSyntax is the sentinel wrapped by every parse/eval error
// EvaluateCondition returns — callers can errors.Is(err, ErrConditionSyntax)
// to distinguish "the expression itself is malformed" from other failure
// modes.
var ErrConditionSyntax = errors.New("domain: malformed condition expression")

type conditionTokenKind int

const (
	condTokIdent conditionTokenKind = iota
	condTokString
	condTokEq
	condTokNeq
	condTokAnd
	condTokOr
)

type conditionToken struct {
	kind  conditionTokenKind
	value string
}

// tokenizeCondition scans expr into a flat token list. This is the entire
// surface the grammar accepts — any other character is a hard error, which
// is the point: nothing resembling a general expression language sneaks
// through (§9).
func tokenizeCondition(expr string) ([]conditionToken, error) {
	var toks []conditionToken
	i := 0
	n := len(expr)
	for i < n {
		c := expr[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == '\'' || c == '"':
			quote := c
			j := i + 1
			for j < n && expr[j] != quote {
				j++
			}
			if j >= n {
				return nil, fmt.Errorf("%w: unterminated string starting at %d", ErrConditionSyntax, i)
			}
			toks = append(toks, conditionToken{kind: condTokString, value: expr[i+1 : j]})
			i = j + 1
		case c == '=' && i+1 < n && expr[i+1] == '=':
			toks = append(toks, conditionToken{kind: condTokEq})
			i += 2
		case c == '!' && i+1 < n && expr[i+1] == '=':
			toks = append(toks, conditionToken{kind: condTokNeq})
			i += 2
		case c == '&' && i+1 < n && expr[i+1] == '&':
			toks = append(toks, conditionToken{kind: condTokAnd})
			i += 2
		case c == '|' && i+1 < n && expr[i+1] == '|':
			toks = append(toks, conditionToken{kind: condTokOr})
			i += 2
		case isIdentChar(c):
			j := i
			for j < n && isIdentChar(expr[j]) {
				j++
			}
			toks = append(toks, conditionToken{kind: condTokIdent, value: expr[i:j]})
			i = j
		default:
			return nil, fmt.Errorf("%w: unexpected character %q at %d", ErrConditionSyntax, c, i)
		}
	}
	return toks, nil
}

func isIdentChar(c byte) bool {
	return c == '_' || c == '.' || c == '-' ||
		(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

type conditionParser struct {
	tokens []conditionToken
	pos    int
	ctx    map[string]string
}

func (p *conditionParser) peek() (conditionToken, bool) {
	if p.pos >= len(p.tokens) {
		return conditionToken{}, false
	}
	return p.tokens[p.pos], true
}

func (p *conditionParser) parseOr() (bool, error) {
	left, err := p.parseAnd()
	if err != nil {
		return false, err
	}
	for {
		tok, ok := p.peek()
		if !ok || tok.kind != condTokOr {
			return left, nil
		}
		p.pos++
		right, err := p.parseAnd()
		if err != nil {
			return false, err
		}
		left = left || right
	}
}

func (p *conditionParser) parseAnd() (bool, error) {
	left, err := p.parseComparison()
	if err != nil {
		return false, err
	}
	for {
		tok, ok := p.peek()
		if !ok || tok.kind != condTokAnd {
			return left, nil
		}
		p.pos++
		right, err := p.parseComparison()
		if err != nil {
			return false, err
		}
		left = left && right
	}
}

func (p *conditionParser) parseComparison() (bool, error) {
	keyTok, ok := p.peek()
	if !ok || keyTok.kind != condTokIdent {
		return false, fmt.Errorf("%w: expected an identifier at token %d", ErrConditionSyntax, p.pos)
	}
	p.pos++

	opTok, ok := p.peek()
	if !ok || (opTok.kind != condTokEq && opTok.kind != condTokNeq) {
		return false, fmt.Errorf("%w: expected == or != at token %d", ErrConditionSyntax, p.pos)
	}
	p.pos++

	valTok, ok := p.peek()
	if !ok || (valTok.kind != condTokIdent && valTok.kind != condTokString) {
		return false, fmt.Errorf("%w: expected a value at token %d", ErrConditionSyntax, p.pos)
	}
	p.pos++

	actual := p.ctx[keyTok.value] // missing key resolves to "", not an error
	switch opTok.kind {
	case condTokEq:
		return actual == valTok.value, nil
	default: // condTokNeq
		return actual != valTok.value, nil
	}
}
