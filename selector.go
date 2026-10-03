package main

import (
	"path/filepath"
	"strings"
	"sync"

	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"
)

const (
	RejectCodeLiveUnavailable = "live_oauth_unavailable"
	SanitizedReasonLiveNoAuth = "no eligible live voice OAuth credential available"
	SanitizedReasonNormal     = "delegating ordinary request to built-in scheduler"
	SanitizedReasonSelected   = "selected configured live voice credential"
	SanitizedReasonFailOpen   = "no configured live match; fail_closed=false delegated to built-in"
)

// SchedulerCore handles request classification, credential filtering, selection, and history tracking.
type SchedulerCore struct {
	mu      sync.RWMutex
	config  Config
	history *DecisionTracker
	rrIndex uint64 // atomic/mutex-protected round-robin index
}

// NewSchedulerCore initializes a scheduler core with given config.
func NewSchedulerCore(cfg Config, history *DecisionTracker) *SchedulerCore {
	if history == nil {
		history = NewDecisionTracker(MaxHistorySize)
	}
	return &SchedulerCore{
		config:  cfg,
		history: history,
	}
}

// UpdateConfig safely updates active configuration.
func (s *SchedulerCore) UpdateConfig(cfg Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = cfg
}

// CurrentConfig safely gets current configuration copy.
func (s *SchedulerCore) CurrentConfig() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// PickAuth executes routing logic according to the specification.
func (s *SchedulerCore) PickAuth(req pluginapi.SchedulerPickRequest) (pluginapi.SchedulerPickResponse, error) {
	s.mu.RLock()
	cfg := s.config
	s.mu.RUnlock()

	// 1. Check classification
	if !IsLiveRequest(req) {
		// Non-Live request: delegate to built-in strategy configured for host (fill-first or round-robin).
		// Returning a handled delegation preserves normal candidate pool and PAT eligibility.
		s.history.Record("normal", OutcomeDelegated, len(req.Candidates), 0, SanitizedReasonNormal)
		return pluginapi.SchedulerPickResponse{
			Handled:         true,
			DelegateBuiltin: cfg.Strategy,
		}, nil
	}

	// 2. Live request: intersect host-provided candidates with configured live_auth_ids
	allowlist := make(map[string]struct{}, len(cfg.LiveAuthIDs))
	for _, id := range cfg.LiveAuthIDs {
		allowlist[id] = struct{}{}
	}

	var matchingCandidates []pluginapi.SchedulerAuthCandidate
	for _, candidate := range req.Candidates {
		if candidateMatchesAllowlist(candidate, allowlist) {
			matchingCandidates = append(matchingCandidates, candidate)
		}
	}

	// 3. Handle no matching candidates
	if len(matchingCandidates) == 0 {
		if cfg.FailClosed {
			s.history.Record("live", OutcomeRejected, len(req.Candidates), 0, SanitizedReasonLiveNoAuth)
			return pluginapi.SchedulerPickResponse{
				Handled:      true,
				Reject:       true,
				RejectCode:   RejectCodeLiveUnavailable,
				RejectReason: SanitizedReasonLiveNoAuth,
			}, nil
		}

		// fail_closed = false: delegate to configured built-in strategy
		s.history.Record("live", OutcomeDelegated, len(req.Candidates), 0, SanitizedReasonFailOpen)
		return pluginapi.SchedulerPickResponse{
			Handled:         true,
			DelegateBuiltin: cfg.Strategy,
		}, nil
	}

	// 4. Select candidate according to strategy
	var selectedID string
	if cfg.Strategy == StrategyRoundRobin {
		s.mu.Lock()
		idx := int(s.rrIndex % uint64(len(matchingCandidates)))
		s.rrIndex++
		selectedID = strings.TrimSpace(matchingCandidates[idx].ID)
		s.mu.Unlock()
	} else {
		// fill-first: pick first matching candidate in host-provided order
		selectedID = strings.TrimSpace(matchingCandidates[0].ID)
	}

	s.history.Record("live", OutcomeSelected, len(req.Candidates), len(matchingCandidates), SanitizedReasonSelected)

	return pluginapi.SchedulerPickResponse{
		Handled: true,
		AuthID:  selectedID,
	}, nil
}

func candidateMatchesAllowlist(candidate pluginapi.SchedulerAuthCandidate, allowlist map[string]struct{}) bool {
	for _, id := range candidateAliasIDs(candidate) {
		if _, ok := allowlist[id]; ok {
			return true
		}
	}
	return false
}

func candidateAliasIDs(candidate pluginapi.SchedulerAuthCandidate) []string {
	values := []string{candidate.ID}
	if candidate.Attributes != nil {
		values = append(values, candidate.Attributes["path"], candidate.Attributes["source"])
	}
	return uniqueIDAliases(values...)
}

func uniqueIDAliases(ids ...string) []string {
	seen := make(map[string]struct{}, len(ids)*2)
	out := make([]string, 0, len(ids)*2)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, exists := seen[value]; exists {
			return
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	for _, id := range ids {
		add(id)
		base := filepath.Base(strings.TrimSpace(id))
		if base != "" && base != "." && base != string(filepath.Separator) {
			add(base)
		}
	}
	return out
}

func configuredIDMatchesInventory(configuredID string, candidateAliases map[string]struct{}) bool {
	for _, alias := range uniqueIDAliases(configuredID) {
		if _, ok := candidateAliases[alias]; ok {
			return true
		}
	}
	return false
}
