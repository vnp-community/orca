package stubs

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// ClassifyMarker is what a test puts in a Request title or body to steer the stub classifier:
// [e2e:type=bug size=S urgency=urgent confidence=0.9]. Missing keys fall back to a plausible default.
var classifyMarker = regexp.MustCompile(`\[e2e:([^\]]*)\]`)

// MarkerFor builds the marker text for a title.
func MarkerFor(typ, size string) string {
	return fmt.Sprintf("[e2e:type=%s size=%s]", typ, size)
}

// Call is one relayed agent call, kept so tests can assert what request-service asked for.
type Call struct {
	Method      string
	DevServerID string
	Params      map[string]any
}

// Handler answers one agent method; ok=false passes to the next handler.
type Handler func(method string, params map[string]any) (result any, ok bool)

// AgentScript is the stub agent: ai.complete classification by marker, plus handlers tests can add for the
// later stages (solution, plan, diagnosis) once those RPCs exist. Unhandled methods return an error so a test
// never passes on an answer nobody wrote.
type AgentScript struct {
	mu       sync.Mutex
	calls    []Call
	handlers []Handler
	// FailClassification makes ai.complete return text that is not a proposal (twice, since the client retries once).
	FailClassification bool
}

func NewAgentScript() *AgentScript { return &AgentScript{} }

func (s *AgentScript) AddHandler(h Handler) {
	s.mu.Lock()
	s.handlers = append(s.handlers, h)
	s.mu.Unlock()
}

func (s *AgentScript) Calls() []Call {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Call(nil), s.calls...)
}

// Answer returns the JSON result for a relayed call.
func (s *AgentScript) Answer(devServerID, method, paramsJSON string) (string, error) {
	params := map[string]any{}
	if paramsJSON != "" {
		if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
			return "", fmt.Errorf("stub agent: bad params for %s: %w", method, err)
		}
	}
	s.mu.Lock()
	s.calls = append(s.calls, Call{Method: method, DevServerID: devServerID, Params: params})
	handlers := append([]Handler(nil), s.handlers...)
	fail := s.FailClassification
	s.mu.Unlock()

	for _, h := range handlers {
		if res, ok := h(method, params); ok {
			return encode(res)
		}
	}
	if method == "ai.complete" {
		prompt, _ := params["prompt"].(string)
		if strings.Contains(prompt, "You classify a software work request") {
			if fail {
				return encode(map[string]any{"content": "I cannot decide."})
			}
			return encode(map[string]any{"content": classificationJSON(prompt)})
		}
	}
	return "", fmt.Errorf("stub agent: no answer written for %s", method)
}

func encode(v any) (string, error) {
	b, err := json.Marshal(v)
	return string(b), err
}

// classificationJSON reads the marker from the prompt's data block. Defaults: change_request, M, normal.
func classificationJSON(prompt string) string {
	typ, size, urgency, confidence := "change_request", "M", "normal", 0.8
	if m := classifyMarker.FindStringSubmatch(prompt); m != nil {
		for _, kv := range strings.Fields(m[1]) {
			k, v, ok := strings.Cut(kv, "=")
			if !ok {
				continue
			}
			switch k {
			case "type":
				typ = v
			case "size":
				size = v
			case "urgency":
				urgency = v
			case "confidence":
				if f, err := strconv.ParseFloat(v, 64); err == nil {
					confidence = f
				}
			}
		}
	}
	b, _ := json.Marshal(map[string]any{"type": typ, "size": size, "urgency": urgency, "confidence": confidence, "reason": "stub agent: from e2e marker"})
	return string(b)
}
