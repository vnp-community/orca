package prompts

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
)

// Argument is one declared argument of a custom prompt.
type Argument struct {
	Name, Description string
	Required          bool
}

// Custom is a tenant prompt as stored by mcp-service.
type Custom struct {
	ID, Name, Description string
	Arguments             []Argument
	Template              string
	Version               int
}

// Store lists the caller tenant's custom prompts (mcp-service).
type Store interface {
	ListCustom(ctx context.Context, p mcpserver.Principal) ([]Custom, error)
}

const (
	cacheTTL          = 60 * time.Second
	maxCustomArgLen   = 1000
	maxCachedTenants  = 1024
	embeddedReadLimit = 5 * time.Second
)

// Provider implements mcpserver.PromptProvider.
type Provider struct {
	store     Store
	resources mcpserver.ResourceProvider
	log       *slog.Logger
	now       func() time.Time

	mu    sync.Mutex
	cache map[string]cacheEntry
}

type cacheEntry struct {
	prompts []Custom
	at      time.Time
}

var _ mcpserver.PromptProvider = (*Provider)(nil)

// NewProvider: store nil = built-ins only; resources nil = prompts that embed
// resources are unavailable (their get fails with -32602).
func NewProvider(store Store, resources mcpserver.ResourceProvider, log *slog.Logger) *Provider {
	if log == nil {
		log = slog.Default()
	}
	return &Provider{store: store, resources: resources, log: log, now: time.Now, cache: map[string]cacheEntry{}}
}

// Invalidate drops the tenant's cached custom prompts (orca.mcp.prompt.changed).
func (pr *Provider) Invalidate(tenantID string) {
	pr.mu.Lock()
	delete(pr.cache, tenantID)
	pr.mu.Unlock()
}

func (pr *Provider) custom(ctx context.Context, p mcpserver.Principal) ([]Custom, error) {
	if pr.store == nil {
		return nil, nil
	}
	pr.mu.Lock()
	if e, ok := pr.cache[p.TenantID]; ok && pr.now().Sub(e.at) < cacheTTL {
		pr.mu.Unlock()
		return e.prompts, nil
	}
	pr.mu.Unlock()
	list, err := pr.store.ListCustom(ctx, p)
	if err != nil {
		return nil, err
	}
	pr.mu.Lock()
	if len(pr.cache) >= maxCachedTenants {
		pr.cache = map[string]cacheEntry{}
	}
	pr.cache[p.TenantID] = cacheEntry{prompts: list, at: pr.now()}
	pr.mu.Unlock()
	return list, nil
}

func localized(m map[string]string, locale string) string {
	if s := m[locale]; s != "" {
		return s
	}
	return m["en"]
}

// ListPrompts: built-ins first (definition order), then custom prompts by
// name. A failing store degrades to built-ins only.
func (pr *Provider) ListPrompts(ctx context.Context, p mcpserver.Principal, acceptLanguage string) ([]*mcp.Prompt, error) {
	locale := ResolveLocale(acceptLanguage)
	out := make([]*mcp.Prompt, 0, len(builtinDefs))
	for _, b := range builtinDefs {
		mp := &mcp.Prompt{Name: b.Name, Title: b.Name, Description: localized(b.Desc, locale)}
		for _, a := range b.Args {
			mp.Arguments = append(mp.Arguments, &mcp.PromptArgument{Name: a.Name, Description: localized(a.Desc, locale), Required: a.Required})
		}
		out = append(out, mp)
	}
	custom, err := pr.custom(ctx, p)
	if err != nil {
		pr.log.WarnContext(ctx, "mcp prompts: custom prompts unavailable; listing built-ins only", slog.Any("error", err))
		return out, nil
	}
	sort.Slice(custom, func(i, j int) bool { return custom[i].Name < custom[j].Name })
	for _, c := range custom {
		mp := &mcp.Prompt{Name: c.Name, Title: c.Name, Description: c.Description}
		for _, a := range c.Arguments {
			mp.Arguments = append(mp.Arguments, &mcp.PromptArgument{Name: a.Name, Description: a.Description, Required: a.Required})
		}
		out = append(out, mp)
	}
	return out, nil
}

// GetPrompt: built-in first, then custom; validates arguments, reads embedded
// resources AS THE CALLER, renders. Every message has role user.
func (pr *Provider) GetPrompt(ctx context.Context, p mcpserver.Principal, sessionID, name string, args map[string]string, acceptLanguage string) (*mcp.GetPromptResult, error) {
	locale := ResolveLocale(acceptLanguage)
	if def, ok := builtinByName(name); ok {
		return pr.getBuiltin(ctx, p, sessionID, def, args, locale)
	}
	custom, err := pr.custom(ctx, p)
	if err != nil {
		return nil, err
	}
	for _, c := range custom {
		if c.Name == name {
			return pr.getCustom(c, args, locale)
		}
	}
	return nil, invalid("unknown prompt: %s", safeName(name))
}

func (pr *Provider) getBuiltin(ctx context.Context, p mcpserver.Principal, sessionID string, def builtinDef, args map[string]string, locale string) (*mcp.GetPromptResult, error) {
	clean, err := validateBuiltinArgs(def, args)
	if err != nil {
		return nil, err
	}
	text, uris, err := renderBuiltin(def, locale, clean)
	if err != nil {
		return nil, err
	}
	msgs := []*mcp.PromptMessage{{Role: "user", Content: &mcp.TextContent{Text: text}}}
	for _, uri := range uris {
		if pr.resources == nil {
			return nil, invalid("resource unavailable: %s", kindOf(uri))
		}
		rctx, cancel := context.WithTimeout(ctx, embeddedReadLimit*4)
		res, err := pr.resources.ReadResource(rctx, p, sessionID, uri)
		cancel()
		switch {
		case errors.Is(err, mcpserver.ErrResourceNotFound):
			return nil, invalid("resource unavailable: %s", kindOf(uri))
		case err != nil:
			return nil, err
		}
		for _, c := range res.Contents {
			msgs = append(msgs, &mcp.PromptMessage{Role: "user", Content: &mcp.EmbeddedResource{Resource: c}})
		}
	}
	return &mcp.GetPromptResult{Description: localized(def.Desc, locale), Messages: msgs}, nil
}

func (pr *Provider) getCustom(c Custom, args map[string]string, locale string) (*mcp.GetPromptResult, error) {
	declared := map[string]Argument{}
	for _, a := range c.Arguments {
		declared[a.Name] = a
	}
	for name, v := range args {
		if _, ok := declared[name]; !ok {
			return nil, invalid("unknown argument: %s", safeName(name))
		}
		if !validCustomValue(v) {
			return nil, invalid("invalid argument: %s", name)
		}
	}
	for _, a := range c.Arguments {
		if a.Required && args[a.Name] == "" {
			return nil, invalid("missing required argument: %s", a.Name)
		}
	}
	text := renderCustom(c.Template, args, locale)
	return &mcp.GetPromptResult{Description: c.Description, Messages: []*mcp.PromptMessage{{Role: "user", Content: &mcp.TextContent{Text: text}}}}, nil
}

func validCustomValue(v string) bool {
	if len(v) > maxCustomArgLen || !utf8.ValidString(v) {
		return false
	}
	for _, r := range v {
		if (unicode.IsControl(r) && r != '\n' && r != '\t') || unicode.Is(unicode.Cf, r) {
			return false
		}
	}
	return true
}

func kindOf(uri string) string {
	rest := uri[len("orca://"):]
	for i := 0; i < len(rest); i++ {
		if rest[i] == '/' || rest[i] == '?' {
			return rest[:i]
		}
	}
	return rest
}
