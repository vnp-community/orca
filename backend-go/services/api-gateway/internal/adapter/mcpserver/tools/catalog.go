package tools

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"
)

const effectiveTTL = 30 * time.Second

// Catalog is the immutable tool set (enabled packs, implemented specs) plus a
// small decision cache. It implements mcpserver.ToolCatalog.
type Catalog struct {
	cfg      Config
	gate     mcpserver.PolicyGate
	view     mcpserver.ToolPolicyView // optional capability of gate
	byName   map[string]*ToolSpec
	ordered  []*ToolSpec
	hardDeny []HardDenied
	mu       sync.Mutex
	cache    map[string]cachedDecision
	now      func() time.Time
}

type cachedDecision struct {
	d  mcpserver.EffectiveDecision
	at time.Time
}

// NewCatalog compiles every spec's schemas (fail fast on a bad spec) and keeps
// those in enabled packs. gate nil = mcpserver.FailClosedGate. channels is the
// registry inventory, used only to expand hard-denied patterns for the admin
// view.
func NewCatalog(specs []*ToolSpec, cfg Config, gate mcpserver.PolicyGate, channels []wscompat.ChannelInfo) (*Catalog, error) {
	cfg = cfg.withDefaults()
	if gate == nil {
		gate = mcpserver.FailClosedGate{}
	}
	c := &Catalog{cfg: cfg, gate: gate, byName: map[string]*ToolSpec{}, cache: map[string]cachedDecision{}, now: time.Now}
	c.view, _ = gate.(mcpserver.ToolPolicyView)
	for _, s := range specs {
		if err := s.prepare(); err != nil {
			return nil, fmt.Errorf("tool %q: %w", s.Name, err)
		}
		if !cfg.Packs[s.Pack] {
			continue
		}
		if _, dup := c.byName[s.Name]; dup {
			return nil, fmt.Errorf("duplicate tool name %q", s.Name)
		}
		c.byName[s.Name] = s
		c.ordered = append(c.ordered, s)
	}
	sort.Slice(c.ordered, func(i, j int) bool {
		if c.ordered[i].Namespace != c.ordered[j].Namespace {
			return c.ordered[i].Namespace < c.ordered[j].Namespace
		}
		return c.ordered[i].Name < c.ordered[j].Name
	})
	ex, err := LoadExclusions()
	if err != nil {
		return nil, err
	}
	c.hardDeny = ex.HardDenied(channels)
	return c, nil
}

// prepare builds schemas once; the output schema is the generic envelope.
func (s *ToolSpec) prepare() error {
	s.input = inputSchema(s.Fields)
	r, err := s.input.Resolve(nil)
	if err != nil {
		return fmt.Errorf("input schema: %w", err)
	}
	s.resolved = r
	s.output = outputEnvelopeSchema(s.List)
	return nil
}

// Lookup returns an implemented, enabled tool.
func (c *Catalog) Lookup(name string) (*ToolSpec, bool) {
	s, ok := c.byName[name]
	if !ok || s.Declared {
		return nil, false
	}
	return s, true
}

// Specs returns all catalogued specs of enabled packs (including Declared).
func (c *Catalog) Specs() []*ToolSpec { return c.ordered }

func hasScope(p mcpserver.Principal, scope string) bool {
	for _, s := range p.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// effective asks the gate's optional view (cached 30s per tenant+tool). Gates
// without a view yield ok=false: callers then rely on call-time Decide.
func (c *Catalog) effective(ctx context.Context, tenantID string, s *ToolSpec) (mcpserver.EffectiveDecision, bool) {
	if c.view == nil {
		return mcpserver.EffectiveDecision{}, false
	}
	key := tenantID + "\x00" + s.Name
	c.mu.Lock()
	if cd, ok := c.cache[key]; ok && c.now().Sub(cd.at) < effectiveTTL {
		c.mu.Unlock()
		return cd.d, true
	}
	c.mu.Unlock()
	d, err := c.view.EffectiveDecision(ctx, tenantID, s.Meta())
	if err != nil {
		// Unknown policy state hides the tool; call-time Decide stays authoritative.
		return mcpserver.EffectiveDecision{Decision: mcpserver.OutcomeDeny, Source: "default"}, true
	}
	c.mu.Lock()
	c.cache[key] = cachedDecision{d, c.now()}
	c.mu.Unlock()
	return d, true
}

// InvalidateTenant drops cached decisions (hook for tools/list_changed events).
func (c *Catalog) InvalidateTenant(tenantID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := range c.cache {
		if len(k) > len(tenantID) && k[:len(tenantID)+1] == tenantID+"\x00" {
			delete(c.cache, k)
		}
	}
}

// ListTools returns tools whose scope the token carries and whose tenant
// default is not deny. require_approval tools stay listed.
func (c *Catalog) ListTools(ctx context.Context, p mcpserver.Principal) ([]*mcp.Tool, error) {
	out := make([]*mcp.Tool, 0, len(c.ordered))
	for _, s := range c.ordered {
		if s.Declared || !hasScope(p, s.RequiredScope()) {
			continue
		}
		if d, ok := c.effective(ctx, p.TenantID, s); ok && d.Decision == mcpserver.OutcomeDeny {
			continue
		}
		out = append(out, s.mcpTool())
	}
	return out, nil
}

func (s *ToolSpec) mcpTool() *mcp.Tool {
	a := s.Annotations
	destructive, openWorld := a.Destructive, a.OpenWorld
	return &mcp.Tool{
		Name: s.Name, Title: s.Title, Description: s.Description,
		InputSchema: s.input, OutputSchema: s.output,
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: a.ReadOnly, DestructiveHint: &destructive,
			IdempotentHint: a.Idempotent, OpenWorldHint: &openWorld},
	}
}

// InputSchema / OutputSchema expose the compiled schemas to tests and tooling.
func (s *ToolSpec) InputSchema() *jsonschema.Schema  { return s.input }
func (s *ToolSpec) OutputSchema() *jsonschema.Schema { return s.output }
