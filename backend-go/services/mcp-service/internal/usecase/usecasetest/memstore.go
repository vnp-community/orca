// Package usecasetest provides an in-memory implementation of the governance
// repository ports with the same conditional-update semantics as the SQL
// adapter, so usecase and red-team tests exercise real flows without a database.
package usecasetest

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"
)

// Clock is a manually advanced clock.
type Clock struct {
	mu sync.Mutex
	t  time.Time
}

func NewClock() *Clock { return &Clock{t: time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)} }
func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}
func (c *Clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

type Outbox struct {
	TenantID string
	Rec      domain.OutboxRecord
}

// MemStore implements PolicyRepository, ApprovalRepository, ToolCallRepository
// and KillSwitchRepository.
type MemStore struct {
	mu        sync.Mutex
	Settings  map[string]domain.TenantSettings
	Epoch     map[string]int64
	Policies  map[string][]domain.ToolPolicy
	Approvals map[string]domain.Approval
	Calls     map[string]domain.ToolCall
	Taint     map[string]time.Time
	Kills     map[string]domain.KillSwitchEntry
	Grants    map[string]bool
	Events    []Outbox
	Revisions []string
	ClockRef  *Clock

	pendingCleanup map[string]bool
}

func NewMemStore(clock *Clock) *MemStore {
	return &MemStore{
		Settings: map[string]domain.TenantSettings{}, Epoch: map[string]int64{}, Policies: map[string][]domain.ToolPolicy{},
		Approvals: map[string]domain.Approval{}, Calls: map[string]domain.ToolCall{}, Taint: map[string]time.Time{},
		Kills: map[string]domain.KillSwitchEntry{}, Grants: map[string]bool{}, ClockRef: clock, pendingCleanup: map[string]bool{},
	}
}

func (m *MemStore) emit(tenantID string, evs []domain.OutboxRecord) {
	for _, e := range evs {
		m.Events = append(m.Events, Outbox{TenantID: tenantID, Rec: e})
	}
}

// EventsOf returns the recorded outbox records for a subject.
func (m *MemStore) EventsOf(subject string) []domain.OutboxRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.OutboxRecord
	for _, e := range m.Events {
		if e.Rec.Subject == subject {
			out = append(out, e.Rec)
		}
	}
	return out
}

func (m *MemStore) ensureSettings(d domain.TenantSettings) domain.TenantSettings {
	s, ok := m.Settings[d.TenantID]
	if !ok {
		s = d
		m.Settings[d.TenantID] = s
	}
	return s
}

// --- PolicyRepository ---

func (m *MemStore) LoadPolicySnapshot(_ context.Context, d domain.TenantSettings) (usecase.PolicySnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.ensureSettings(d)
	ps := append([]domain.ToolPolicy(nil), m.Policies[d.TenantID]...)
	return usecase.PolicySnapshot{Settings: s, Policies: ps, Epoch: m.Epoch[d.TenantID]}, nil
}

func (m *MemStore) ListToolPolicies(_ context.Context, t string) ([]domain.ToolPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]domain.ToolPolicy{}, m.Policies[t]...), nil
}

func (m *MemStore) CreateToolPolicy(_ context.Context, t string, p domain.ToolPolicy, evs []domain.OutboxRecord) (domain.ToolPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p.Version = 1
	m.Policies[t] = append(m.Policies[t], p)
	m.Epoch[t]++
	m.Revisions = append(m.Revisions, "create:"+p.ID)
	m.emit(t, evs)
	return p, nil
}

func (m *MemStore) UpdateToolPolicy(_ context.Context, t string, p domain.ToolPolicy, evs []domain.OutboxRecord) (domain.ToolPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, cur := range m.Policies[t] {
		if cur.ID != p.ID {
			continue
		}
		if cur.Version != p.Version {
			return domain.ToolPolicy{}, domain.ErrPolicyVersionConflict(cur.Version)
		}
		p.Version, p.CreatedBy = cur.Version+1, cur.CreatedBy
		m.Policies[t][i] = p
		m.Epoch[t]++
		m.Revisions = append(m.Revisions, "update:"+p.ID)
		m.emit(t, evs)
		return p, nil
	}
	return domain.ToolPolicy{}, domain.ErrNotFound()
}

func (m *MemStore) DeleteToolPolicy(_ context.Context, t, id, _ string, evs []domain.OutboxRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, cur := range m.Policies[t] {
		if cur.ID == id {
			m.Policies[t] = append(m.Policies[t][:i], m.Policies[t][i+1:]...)
			m.Epoch[t]++
			m.Revisions = append(m.Revisions, "delete:"+id)
			m.emit(t, evs)
			return nil
		}
	}
	return domain.ErrNotFound()
}

func (m *MemStore) PatchTenantSettings(_ context.Context, d domain.TenantSettings, p usecase.SettingsPatch, actor string, evs []domain.OutboxRecord) (domain.TenantSettings, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.ensureSettings(d)
	if p.Enabled != nil {
		s.Enabled = *p.Enabled
	}
	if p.DCREnabled != nil {
		s.DCREnabled = *p.DCREnabled
	}
	if p.MaxTokenDays != nil {
		s.MaxTokenDays = *p.MaxTokenDays
	}
	if p.ApprovalTTLSeconds != nil {
		s.ApprovalTTLSeconds = *p.ApprovalTTLSeconds
	}
	s.UpdatedBy = actor
	m.Settings[d.TenantID] = s
	m.Epoch[d.TenantID]++
	m.emit(d.TenantID, evs)
	return s, nil
}

// --- ApprovalRepository ---

func (m *MemStore) FindOrCreatePendingApproval(_ context.Context, a domain.Approval, l usecase.ApprovalLimits, now time.Time) (domain.Approval, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	pending, recent := 0, 0
	for id, cur := range m.Approvals {
		if cur.TenantID != a.TenantID || cur.UserID != a.UserID || cur.ClientID != a.ClientID {
			continue
		}
		open := (cur.Status == domain.ApprovalPending || cur.Status == domain.ApprovalApproved) && cur.ConsumedAt == nil
		if open && cur.ParamsHash == a.ParamsHash {
			if cur.ExpiresAt.After(now) {
				return cur, false, nil
			}
			cur.Status = domain.ApprovalExpired
			m.Approvals[id] = cur
			continue
		}
		if cur.Status == domain.ApprovalPending && cur.ExpiresAt.After(now) {
			pending++
		}
		if cur.CreatedAt.After(now.Add(-time.Hour)) {
			recent++
		}
	}
	if (l.MaxPendingPerClient > 0 && pending >= l.MaxPendingPerClient) || (l.MaxCreatedPerHour > 0 && recent >= l.MaxCreatedPerHour) {
		return domain.Approval{}, false, usecase.ErrApprovalFlood
	}
	a.Status = domain.ApprovalPending
	m.Approvals[a.ID] = a
	ev, _ := domain.NewApprovalRequestedEvent(uuid.NewString(), a, now)
	m.emit(a.TenantID, []domain.OutboxRecord{ev})
	return a, true, nil
}

func (m *MemStore) DecideApproval(_ context.Context, in usecase.DecideApprovalRepoInput) (domain.Approval, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.Approvals[in.ApprovalID]
	if !ok || a.TenantID != in.TenantID || a.UserID != in.UserID {
		return domain.Approval{}, domain.DecideDiagnosis(domain.Approval{}, false, in.Now)
	}
	if a.Status != domain.ApprovalPending || !a.ExpiresAt.After(in.Now) || (in.Approve && !domain.ParamsHashEqual(a.ParamsHash, in.ParamsHash)) {
		return domain.Approval{}, domain.DecideDiagnosis(a, true, in.Now)
	}
	a.Status = domain.ApprovalDenied
	if in.Approve {
		a.Status = domain.ApprovalApproved
	}
	t := in.Now
	a.DecidedAt, a.DecidedVia, a.DecisionNote = &t, in.Via, in.Note
	m.Approvals[a.ID] = a
	ev, _ := domain.NewApprovalResolvedEvent(uuid.NewString(), a, in.Now)
	m.emit(a.TenantID, []domain.OutboxRecord{ev})
	if in.Approve {
		aud, _ := domain.NewAdminAuditEvent(uuid.NewString(), uuid.NewString(), a.TenantID, in.UserID, domain.AuditActionApprovalDecide,
			"mcp_approval", a.ID, "allowed", in.Now, map[string]any{"approval_id": a.ID, "tool": a.ToolName, "risk": a.Risk, "decided_via": in.Via})
		m.emit(a.TenantID, []domain.OutboxRecord{aud})
	} else {
		c := domain.ToolCall{ID: uuid.NewString(), TenantID: a.TenantID, UserID: a.UserID, ClientID: a.ClientID, ClientName: a.ClientName,
			ToolName: a.ToolName, Channel: a.Channel, Risk: a.Risk, ParamsHash: a.ParamsHash,
			ArgsSummary: domain.SummarizeArgs(a.ArgsPreview, domain.SecretRedactor{}), Decision: domain.CallDenied, ApprovalID: a.ID,
			ApprovedBy: in.UserID, State: domain.CallStateDone, StartedAt: in.Now}
		m.Calls[c.ID] = c
		cev, _ := domain.NewToolCallAuditEvent(uuid.NewString(), c, in.Now, 0)
		m.emit(a.TenantID, []domain.OutboxRecord{cev})
	}
	return a, nil
}

func (m *MemStore) GetApproval(_ context.Context, t, id string) (domain.Approval, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.Approvals[id]
	if !ok || a.TenantID != t {
		return domain.Approval{}, domain.ErrNotFound()
	}
	return a, nil
}

func (m *MemStore) ListApprovals(_ context.Context, q usecase.ApprovalQuery) ([]domain.Approval, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.Approval
	for _, a := range m.Approvals {
		if a.TenantID != q.TenantID || a.UserID != q.UserID {
			continue
		}
		if q.PendingOnly && !(a.Status == domain.ApprovalPending && a.ExpiresAt.After(q.Now)) {
			continue
		}
		if !q.CursorAt.IsZero() && !(a.CreatedAt.Before(q.CursorAt) || (a.CreatedAt.Equal(q.CursorAt) && a.ID < q.CursorID)) {
			continue
		}
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.After(out[j].CreatedAt)
		}
		return out[i].ID > out[j].ID
	})
	if q.Limit > 0 && len(out) > q.Limit {
		out = out[:q.Limit]
	}
	return out, nil
}

func (m *MemStore) ListExpiredApprovalRefs(_ context.Context, now time.Time, limit int) ([]usecase.Ref, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []usecase.Ref
	for _, a := range m.Approvals {
		if a.Status == domain.ApprovalPending && !a.ExpiresAt.After(now) && len(out) < limit {
			out = append(out, usecase.Ref{TenantID: a.TenantID, ID: a.ID})
		}
	}
	return out, nil
}

func (m *MemStore) ExpireApproval(_ context.Context, t, id string, now time.Time) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.Approvals[id]
	if !ok || a.TenantID != t || a.Status != domain.ApprovalPending || a.ExpiresAt.After(now) {
		return false, nil
	}
	a.Status = domain.ApprovalExpired
	m.Approvals[id] = a
	ev, _ := domain.NewApprovalResolvedEvent(uuid.NewString(), a, now)
	m.emit(t, []domain.OutboxRecord{ev})
	c := domain.ToolCall{ID: uuid.NewString(), TenantID: t, UserID: a.UserID, ClientID: a.ClientID, ClientName: a.ClientName, ToolName: a.ToolName,
		Channel: a.Channel, Risk: a.Risk, ParamsHash: a.ParamsHash, ArgsSummary: domain.SummarizeArgs(a.ArgsPreview, domain.SecretRedactor{}),
		Decision: domain.CallExpired, ReasonCode: domain.ReasonApprovalExpired, ApprovalID: id, State: domain.CallStateDone, StartedAt: now}
	m.Calls[c.ID] = c
	aev, _ := domain.NewToolCallAuditEvent(uuid.NewString(), c, now, 0)
	m.emit(t, []domain.OutboxRecord{aev})
	return true, nil
}

func inScope(scope, target string, clientID, session, root string, m *MemStore, tenantID, userID string) bool {
	switch scope {
	case domain.KillScopeTenant:
		return true
	case domain.KillScopeClient:
		return clientID == target
	case domain.KillScopeSession:
		return session == target || root == target
	}
	return false
}

func (m *MemStore) CancelApprovals(_ context.Context, t, scope, target string, now time.Time) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for id, a := range m.Approvals {
		if a.TenantID != t || a.ConsumedAt != nil || (a.Status != domain.ApprovalPending && a.Status != domain.ApprovalApproved) {
			continue
		}
		if !inScope(scope, target, a.ClientID, a.SessionID, "", m, t, a.UserID) {
			continue
		}
		a.Status = domain.ApprovalCancelled
		m.Approvals[id] = a
		ev, _ := domain.NewApprovalResolvedEvent(uuid.NewString(), a, now)
		m.emit(t, []domain.OutboxRecord{ev})
		n++
	}
	return n, nil
}

// --- ToolCallRepository ---

func (m *MemStore) AdmitToolCall(_ context.Context, req usecase.AdmitRequest) (usecase.AdmitResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := req.Call
	var res usecase.AdmitResult
	var approval *domain.Approval
	if req.ConsumeApprovalHash != "" {
		for id, a := range m.Approvals {
			if a.TenantID == c.TenantID && a.UserID == c.UserID && a.ClientID == c.ClientID && a.ParamsHash == req.ConsumeApprovalHash &&
				a.Status == domain.ApprovalApproved && a.ConsumedAt == nil && a.ExpiresAt.After(req.Now) {
				a := a
				approval = &a
				_ = id
				break
			}
		}
		if approval == nil {
			res.ApprovalMissing = true
			return res, nil
		}
	}
	l := req.Limits
	var total, inClass, daily, loop1, loop5 int
	for _, x := range m.Calls {
		if x.TenantID != c.TenantID || x.UserID != c.UserID || (x.Decision != domain.CallAllow && x.Decision != domain.CallApproved) || !x.StartedAt.After(req.Now.Add(-24*time.Hour)) {
			continue
		}
		daily++
		if x.ClientID != c.ClientID {
			continue
		}
		if x.StartedAt.After(req.Now.Add(-time.Minute)) {
			total++
			if x.RiskClass == c.RiskClass {
				inClass++
			}
			if x.ParamsHash == c.ParamsHash {
				loop1++
			}
		}
		if x.ParamsHash == c.ParamsHash && x.StartedAt.After(req.Now.Add(-5*time.Minute)) {
			loop5++
		}
	}
	switch {
	case l.LoopBlock > 0 && loop5 >= l.LoopBlock:
		res.DenyReason = domain.ReasonLoopBlocked
	case l.LoopSlowDown > 0 && loop1 >= l.LoopSlowDown:
		res.DenyReason = domain.ReasonLoopSlowDown
	case l.TotalPerMinute > 0 && total >= l.TotalPerMinute, l.PerMinute[c.RiskClass] > 0 && inClass >= l.PerMinute[c.RiskClass], l.DailyPerUser > 0 && daily >= l.DailyPerUser:
		res.DenyReason = domain.ReasonRateLimited
	}
	if res.DenyReason != "" {
		return res, nil
	}
	if approval != nil {
		t := req.Now
		approval.ConsumedAt = &t
		m.Approvals[approval.ID] = *approval
		c.ApprovalID, c.ApprovedBy = approval.ID, c.UserID
		res.ApprovalID, res.ApproverID = approval.ID, c.UserID
	}
	c.StartedAt, c.State = req.Now, domain.CallStateStarted
	m.Calls[c.ID] = c
	res.Admitted = true
	return res, nil
}

func (m *MemStore) RecordFinalCall(_ context.Context, c domain.ToolCall, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c.State = domain.CallStateDone
	m.Calls[c.ID] = c
	ev, _ := domain.NewToolCallAuditEvent(uuid.NewString(), c, now, c.SuppressedCount)
	m.emit(c.TenantID, []domain.OutboxRecord{ev})
	return nil
}

func (m *MemStore) FinalizeToolCall(_ context.Context, t, u, id, result, reason string, dur int64, now time.Time, ttl time.Duration) (domain.ToolCall, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.Calls[id]
	if !ok || c.TenantID != t || c.UserID != u {
		return domain.ToolCall{}, domain.ErrNotFound()
	}
	if c.State != domain.CallStateStarted {
		return c, nil
	}
	c.State, c.Result, c.DurationMs, c.FinishedAt = domain.CallStateDone, result, dur, &now
	if reason != "" {
		c.ReasonCode = reason
	}
	m.Calls[id] = c
	if c.ReadUntrusted && result == domain.ResultOK {
		m.Taint[t+"|"+u+"|"+c.ClientID] = now.Add(ttl)
	}
	ev, _ := domain.NewToolCallAuditEvent(uuid.NewString(), c, now, 0)
	m.emit(t, []domain.OutboxRecord{ev})
	return c, nil
}

func (m *MemStore) IsTainted(_ context.Context, t, u, client string, now time.Time) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Taint[t+"|"+u+"|"+client].After(now), nil
}

func (m *MemStore) ListStaleCalls(_ context.Context, before time.Time, limit int) ([]usecase.Ref, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []usecase.Ref
	for _, c := range m.Calls {
		if c.State == domain.CallStateStarted && c.StartedAt.Before(before) && len(out) < limit {
			out = append(out, usecase.Ref{TenantID: c.TenantID, ID: c.ID})
		}
	}
	return out, nil
}

func (m *MemStore) interrupt(c domain.ToolCall, reason string, now time.Time) {
	c.State, c.Result, c.ReasonCode, c.FinishedAt = domain.CallStateDone, domain.ResultError, reason, &now
	m.Calls[c.ID] = c
	ev, _ := domain.NewToolCallAuditEvent(uuid.NewString(), c, now, 0)
	m.emit(c.TenantID, []domain.OutboxRecord{ev})
}

func (m *MemStore) InterruptCall(_ context.Context, t, id, reason string, now time.Time) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.Calls[id]
	if !ok || c.TenantID != t || c.State != domain.CallStateStarted {
		return false, nil
	}
	m.interrupt(c, reason, now)
	return true, nil
}

func (m *MemStore) InterruptCallsInScope(_ context.Context, t, scope, target, reason string, now time.Time) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var ids []string
	for _, c := range m.Calls {
		if c.TenantID == t && c.State == domain.CallStateStarted && inScope(scope, target, c.ClientID, c.SessionID, c.RootSessionID, m, t, c.UserID) {
			m.interrupt(c, reason, now)
			ids = append(ids, c.ID)
		}
	}
	return ids, nil
}

func (m *MemStore) PurgeFinishedCalls(_ context.Context, before time.Time, limit int) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for id, c := range m.Calls {
		if c.State == domain.CallStateDone && c.StartedAt.Before(before) && n < limit {
			delete(m.Calls, id)
			n++
		}
	}
	return n, nil
}

// --- KillSwitchRepository ---

func (m *MemStore) UpsertKillSwitch(_ context.Context, e domain.KillSwitchEntry, evs []domain.OutboxRecord) (domain.KillSwitchEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := e.TenantID + "|" + e.Scope + "|" + e.TargetID
	if cur, ok := m.Kills[key]; ok {
		e.ID = cur.ID
	}
	m.Kills[key] = e
	if e.Scope == domain.KillScopeTenant {
		if s, ok := m.Settings[e.TenantID]; ok {
			s.KillSwitch = domain.KillSwitch{Active: e.Active, Reason: e.Reason}
			m.Settings[e.TenantID] = s
		}
	}
	m.pendingCleanup[key] = e.Active
	m.Epoch[e.TenantID]++
	m.emit(e.TenantID, evs)
	return e, nil
}

func (m *MemStore) ListKillSwitches(_ context.Context, t string, activeOnly bool) ([]domain.KillSwitchEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.KillSwitchEntry
	for _, e := range m.Kills {
		if e.TenantID == t && (!activeOnly || e.Active) {
			out = append(out, e)
		}
	}
	return out, nil
}

func (m *MemStore) PendingKillCleanups(_ context.Context, limit int) ([]domain.KillSwitchEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.KillSwitchEntry
	for k, pending := range m.pendingCleanup {
		if pending && len(out) < limit {
			out = append(out, m.Kills[k])
		}
	}
	return out, nil
}

func (m *MemStore) ClearKillCleanup(_ context.Context, t, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, e := range m.Kills {
		if e.TenantID == t && e.ID == id {
			m.pendingCleanup[k] = false
		}
	}
	return nil
}

func (m *MemStore) GrantIDsInScope(_ context.Context, t, scope, target string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []string
	for id := range m.Grants {
		if scope == domain.KillScopeTenant || (scope == domain.KillScopeGrant && id == target) {
			out = append(out, id)
		}
	}
	return out, nil
}

func (m *MemStore) GrantExists(_ context.Context, _, id string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Grants[id], nil
}

var (
	_ usecase.PolicyRepository     = (*MemStore)(nil)
	_ usecase.ApprovalRepository   = (*MemStore)(nil)
	_ usecase.ToolCallRepository   = (*MemStore)(nil)
	_ usecase.KillSwitchRepository = (*MemStore)(nil)
)
