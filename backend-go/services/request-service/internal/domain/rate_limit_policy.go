package domain

// Limit is a token bucket: PerSecond refill with Burst capacity.
type Limit struct {
	PerSecond float64
	Burst     int
}

// DefaultLimits per rate class (CR-REQ-035 2.7); unmeasured proposals, tune with real traffic.
var DefaultLimits = map[string]Limit{
	RateRead:    {PerSecond: 20, Burst: 40},
	RateWrite:   {PerSecond: 5, Burst: 10},
	RateAI:      {PerSecond: 2, Burst: 4},
	RateWebhook: {PerSecond: 10, Burst: 20},
}

// ConcurrencyCaps are enforced from the database so they hold across replicas.
type ConcurrencyCaps struct {
	RunningPerTenant      int
	RunningPerProject     int
	OpenRequestsPerTenant int
}

var DefaultConcurrencyCaps = ConcurrencyCaps{RunningPerTenant: 10, RunningPerProject: 2, OpenRequestsPerTenant: 2000}

// Rate classes shared by the catalog and the limiter.
const (
	RateRead    = "read"
	RateWrite   = "write"
	RateAI      = "ai"
	RateWebhook = "webhook"
	RateNone    = "none"
)
