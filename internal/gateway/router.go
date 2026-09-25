package gateway

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrNoProfilesAvailable    = errors.New("no profiles available in router pool")
	ErrAllProfilesCoolingDown = errors.New("all profiles in pool are currently rate-limited or cooling down")
	ErrProfileNotFound        = errors.New("specified profile not found in pool")
)

// RoutingStrategy represents the algorithm used to select profiles
type RoutingStrategy string

const (
	StrategySmart      RoutingStrategy = "smart"
	StrategyRoundRobin RoutingStrategy = "round-robin"
	StrategyPriority   RoutingStrategy = "priority"
	StrategySticky     RoutingStrategy = "sticky"
)

// ParseRoutingStrategy normalizes strategy string names with aliases
func ParseRoutingStrategy(s string) RoutingStrategy {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "round-robin", "roundrobin", "rr":
		return StrategyRoundRobin
	case "priority", "fallback", "ordered":
		return StrategyPriority
	case "sticky", "affinity":
		return StrategySticky
	default:
		return StrategySmart
	}
}

// ProfileNode represents a profile in the gateway pool with health and metrics
type ProfileNode struct {
	Name              string
	RemainingFraction float64
	ResetTime         time.Time
	CooldownUntil     time.Time
	ConsecutiveErrors int
	TotalRequests     int64
	TotalSuccess      int64
	RateLimitHits     int64
	TotalFailovers    int64
	LastUsed          time.Time
	LastError         string
	mu                sync.RWMutex
}

// IsCoolingDown returns whether the profile is currently under rate-limit cooldown
func (p *ProfileNode) IsCoolingDown(now time.Time) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return now.Before(p.CooldownUntil)
}

// Status returns human-readable status for the profile node
func (p *ProfileNode) Status(now time.Time) string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if now.Before(p.CooldownUntil) {
		if p.ConsecutiveErrors > 0 {
			return "rate_limited"
		}
		return "cooldown"
	}
	return "healthy"
}

// Score computes the dynamic priority score for Smart routing. Higher is better.
func (p *ProfileNode) Score(now time.Time) float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if now.Before(p.CooldownUntil) {
		return -1_000_000.0 // cooling down nodes heavily penalized
	}

	// Base score: 0 to 1000 from remaining fraction
	fraction := p.RemainingFraction
	if fraction <= 0 {
		fraction = 1.0 // default healthy if unknown
	}
	score := fraction * 1000.0

	// Penalty for recent consecutive errors
	score -= float64(p.ConsecutiveErrors) * 100.0

	// Bonus for idle profiles (load distribution over time)
	if !p.LastUsed.IsZero() {
		idleMinutes := now.Sub(p.LastUsed).Minutes()
		if idleMinutes > 0 {
			if idleMinutes > 30 {
				idleMinutes = 30
			}
			score += idleMinutes * 5.0
		}
	} else {
		score += 50.0 // never used gets slight priority
	}

	return score
}

// Router coordinates multi-account profile pooling, distribution, and failover
type Router struct {
	mu              sync.RWMutex
	nodes           map[string]*ProfileNode
	profilesOrder   []string
	strategy        RoutingStrategy
	stickyProfile   string
	defaultCooldown time.Duration
	failoverEnabled bool
	rrIndex         uint64
	tokenResolver   TokenResolverFunc
	nowFn           func() time.Time
}

// RouterOption defines configuration options for Router
type RouterOption func(*Router)

// WithRouterStrategy sets default routing strategy
func WithRouterStrategy(s RoutingStrategy) RouterOption {
	return func(r *Router) {
		r.strategy = s
	}
}

// WithDefaultCooldown sets standard cooldown duration on 429/403
func WithDefaultCooldown(d time.Duration) RouterOption {
	return func(r *Router) {
		if d > 0 {
			r.defaultCooldown = d
		}
	}
}

// WithFailoverEnabled enables or disables auto-failover globally
func WithFailoverEnabled(enabled bool) RouterOption {
	return func(r *Router) {
		r.failoverEnabled = enabled
	}
}

// WithRouterTokenResolver configures the token resolver for the router
func WithRouterTokenResolver(fn TokenResolverFunc) RouterOption {
	return func(r *Router) {
		r.tokenResolver = fn
	}
}

// NewRouter creates a new multi-account router instance
func NewRouter(opts ...RouterOption) *Router {
	r := &Router{
		nodes:           make(map[string]*ProfileNode),
		profilesOrder:   make([]string, 0),
		strategy:        StrategySmart,
		defaultCooldown: 60 * time.Second,
		failoverEnabled: true,
		nowFn:           time.Now,
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// SetNowFunc sets custom time provider for testing
func (r *Router) SetNowFunc(fn func() time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if fn != nil {
		r.nowFn = fn
	} else {
		r.nowFn = time.Now
	}
}

func (r *Router) now() time.Time {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.nowFn != nil {
		return r.nowFn()
	}
	return time.Now()
}

// RegisterProfile adds or updates a profile in the pool
func (r *Router) RegisterProfile(name string) *ProfileNode {
	r.mu.Lock()
	defer r.mu.Unlock()

	if node, exists := r.nodes[name]; exists {
		return node
	}

	node := &ProfileNode{
		Name:              name,
		RemainingFraction: 1.0,
	}
	r.nodes[name] = node
	r.profilesOrder = append(r.profilesOrder, name)
	sort.Strings(r.profilesOrder)
	return node
}

// SyncProfiles synchronizes router pool with the given list of profile names
func (r *Router) SyncProfiles(names []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	currentSet := make(map[string]bool)
	for _, n := range names {
		if strings.TrimSpace(n) == "" {
			continue
		}
		currentSet[n] = true
		if _, exists := r.nodes[n]; !exists {
			r.nodes[n] = &ProfileNode{
				Name:              n,
				RemainingFraction: 1.0,
			}
			r.profilesOrder = append(r.profilesOrder, n)
		}
	}

	// Remove deleted profiles
	var updatedOrder []string
	for _, n := range r.profilesOrder {
		if currentSet[n] {
			updatedOrder = append(updatedOrder, n)
		} else {
			delete(r.nodes, n)
		}
	}
	sort.Strings(updatedOrder)
	r.profilesOrder = updatedOrder

	if r.stickyProfile != "" && !currentSet[r.stickyProfile] {
		r.stickyProfile = ""
	}
}

// SetStrategy updates the active distribution strategy
func (r *Router) SetStrategy(s RoutingStrategy) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.strategy = s
}

// GetStrategy returns the active distribution strategy
func (r *Router) GetStrategy() RoutingStrategy {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.strategy
}

// SetFailover enables or disables auto-failover
func (r *Router) SetFailover(enabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failoverEnabled = enabled
}

// IsFailoverEnabled checks if failover is enabled
func (r *Router) IsFailoverEnabled() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.failoverEnabled
}

// ResetCooldowns clears all rate-limit cooldowns across all profiles
func (r *Router) ResetCooldowns() {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, node := range r.nodes {
		node.mu.Lock()
		node.CooldownUntil = time.Time{}
		node.ConsecutiveErrors = 0
		node.LastError = ""
		node.mu.Unlock()
	}
}

// SelectProfile picks the best eligible profile based on request and strategy
func (r *Router) SelectProfile(
	ctx context.Context,
	model string,
	requestedProfile string,
	excluded map[string]bool,
	strategyOverride RoutingStrategy,
) (*ProfileNode, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.nodes) == 0 {
		defNode := &ProfileNode{Name: "default", RemainingFraction: 1.0}
		r.nodes["default"] = defNode
		r.profilesOrder = append(r.profilesOrder, "default")
	}

	now := time.Now()
	if r.nowFn != nil {
		now = r.nowFn()
	}

	// 1. Explicit profile requested (and not a pool wildcard)
	reqProf := strings.TrimSpace(requestedProfile)
	if reqProf != "" && reqProf != "auto" && reqProf != "pool" && reqProf != "any" {
		node, exists := r.nodes[reqProf]
		if !exists {
			// Profile may exist on disk but not in pool yet
			node = &ProfileNode{Name: reqProf, RemainingFraction: 1.0}
			r.nodes[reqProf] = node
			r.profilesOrder = append(r.profilesOrder, reqProf)
			sort.Strings(r.profilesOrder)
		}

		if excluded == nil || !excluded[reqProf] {
			if !node.IsCoolingDown(now) {
				node.mu.Lock()
				node.LastUsed = now
				node.mu.Unlock()
				r.stickyProfile = reqProf
				return node, nil
			}
			// If requested profile is in cooldown and failover is disabled, fail immediately
			if !r.failoverEnabled {
				return nil, fmt.Errorf("profile %q is currently cooling down until %s", reqProf, node.CooldownUntil.Format(time.RFC3339))
			}
			// If failover enabled, treat as excluded and fall through to pool selection
			if excluded == nil {
				excluded = make(map[string]bool)
			}
			excluded[reqProf] = true
		}
	}

	// 2. Filter available healthy candidates
	strat := r.strategy
	if strategyOverride != "" {
		strat = strategyOverride
	}

	var healthyCandidates []*ProfileNode
	var allUnexcluded []*ProfileNode

	for _, name := range r.profilesOrder {
		if excluded != nil && excluded[name] {
			continue
		}
		node := r.nodes[name]
		allUnexcluded = append(allUnexcluded, node)
		if !node.IsCoolingDown(now) {
			healthyCandidates = append(healthyCandidates, node)
		}
	}

	// 3. Fallback if all candidates are currently cooling down
	if len(healthyCandidates) == 0 {
		if len(allUnexcluded) == 0 {
			return nil, ErrNoProfilesAvailable
		}
		return nil, ErrAllProfilesCoolingDown
	}

	// 4. Select candidate based on strategy
	var chosen *ProfileNode

	switch strat {
	case StrategyRoundRobin:
		idx := atomic.AddUint64(&r.rrIndex, 1) - 1
		chosen = healthyCandidates[int(idx%uint64(len(healthyCandidates)))]

	case StrategyPriority:
		// Select first in priority/alphabetical order
		chosen = healthyCandidates[0]

	case StrategySticky:
		if r.stickyProfile != "" && (excluded == nil || !excluded[r.stickyProfile]) {
			if node, ok := r.nodes[r.stickyProfile]; ok && !node.IsCoolingDown(now) {
				chosen = node
			}
		}
		if chosen == nil {
			// Fall back to best via smart score
			chosen = pickHighestScore(healthyCandidates, now)
		}

	case StrategySmart:
		fallthrough
	default:
		chosen = pickHighestScore(healthyCandidates, now)
	}

	if chosen != nil {
		chosen.mu.Lock()
		chosen.LastUsed = now
		chosen.mu.Unlock()
		r.stickyProfile = chosen.Name
	}

	return chosen, nil
}

func pickHighestScore(candidates []*ProfileNode, now time.Time) *ProfileNode {
	if len(candidates) == 0 {
		return nil
	}
	best := candidates[0]
	bestScore := best.Score(now)

	for i := 1; i < len(candidates); i++ {
		cScore := candidates[i].Score(now)
		if cScore > bestScore {
			best = candidates[i]
			bestScore = cScore
		}
	}
	return best
}

// MarkSuccess records a successful completion on a profile
func (r *Router) MarkSuccess(name string) {
	r.mu.RLock()
	node, exists := r.nodes[name]
	r.mu.RUnlock()

	if !exists {
		return
	}

	node.mu.Lock()
	defer node.mu.Unlock()
	node.TotalRequests++
	node.TotalSuccess++
	node.ConsecutiveErrors = 0
	node.LastError = ""
}

// MarkRateLimited records a 429/403 rate limit event and places node on cooldown
func (r *Router) MarkRateLimited(name string, code int, msg string, cooldown time.Duration) {
	r.mu.RLock()
	node, exists := r.nodes[name]
	r.mu.RUnlock()

	if !exists {
		return
	}

	now := r.now()
	if cooldown <= 0 {
		cooldown = r.defaultCooldown
	}

	node.mu.Lock()
	defer node.mu.Unlock()
	node.TotalRequests++
	node.RateLimitHits++
	node.ConsecutiveErrors++
	node.CooldownUntil = now.Add(cooldown)
	node.LastError = fmt.Sprintf("HTTP %d: %s", code, msg)
}

// MarkFailover records that a failover switch occurred from a failed profile
func (r *Router) MarkFailover(name string) {
	r.mu.RLock()
	node, exists := r.nodes[name]
	r.mu.RUnlock()

	if !exists {
		return
	}

	node.mu.Lock()
	defer node.mu.Unlock()
	node.TotalFailovers++
}

// UpdateQuota updates the quota telemetry for a profile node
func (r *Router) UpdateQuota(name string, remainingFraction float64, resetTime time.Time) {
	r.mu.RLock()
	node, exists := r.nodes[name]
	r.mu.RUnlock()

	if !exists {
		return
	}

	node.mu.Lock()
	defer node.mu.Unlock()
	node.RemainingFraction = remainingFraction
	node.ResetTime = resetTime
}

// GetStatus returns a structured telemetry snapshot of the router pool
func (r *Router) GetStatus() RouterStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()

	now := r.now()
	status := RouterStatus{
		Strategy:        string(r.strategy),
		FailoverEnabled: r.failoverEnabled,
		DefaultCooldown: r.defaultCooldown.String(),
		TotalProfiles:   len(r.profilesOrder),
		Profiles:        make([]ProfileNodeStatus, 0, len(r.profilesOrder)),
	}

	for _, name := range r.profilesOrder {
		node := r.nodes[name]
		node.mu.RLock()

		nodeStatus := node.Status(now)
		if nodeStatus == "healthy" {
			status.HealthyProfiles++
		} else {
			status.CooldownProfiles++
		}

		status.TotalRequests += node.TotalRequests
		status.TotalSuccess += node.TotalSuccess
		status.TotalFailovers += node.TotalFailovers

		var cdUntilStr *string
		var cdRemaining string
		if now.Before(node.CooldownUntil) {
			s := node.CooldownUntil.UTC().Format(time.RFC3339)
			cdUntilStr = &s
			cdRemaining = node.CooldownUntil.Sub(now).Round(time.Second).String()
		}

		var lastUsedStr *string
		if !node.LastUsed.IsZero() {
			s := node.LastUsed.UTC().Format(time.RFC3339)
			lastUsedStr = &s
		}

		resetTimeStr := ""
		if !node.ResetTime.IsZero() {
			resetTimeStr = node.ResetTime.UTC().Format(time.RFC3339)
		}

		pStatus := ProfileNodeStatus{
			Name:              node.Name,
			Status:            nodeStatus,
			RemainingFraction: node.RemainingFraction,
			ResetTime:         resetTimeStr,
			CooldownUntil:     cdUntilStr,
			CooldownRemaining: cdRemaining,
			TotalRequests:     node.TotalRequests,
			TotalSuccess:      node.TotalSuccess,
			RateLimitHits:     node.RateLimitHits,
			TotalFailovers:    node.TotalFailovers,
			ConsecutiveErrors: node.ConsecutiveErrors,
			LastUsed:          lastUsedStr,
			LastError:         node.LastError,
		}
		node.mu.RUnlock()

		status.Profiles = append(status.Profiles, pStatus)
	}

	return status
}

// AvailableCount returns the number of healthy profiles in the pool
func (r *Router) AvailableCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	now := r.now()
	count := 0
	for _, node := range r.nodes {
		if !node.IsCoolingDown(now) {
			count++
		}
	}
	return count
}
