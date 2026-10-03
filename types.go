package main

import (
	"encoding/json"

	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"
)

// Plugin metadata constants.
const (
	PluginID               = "cpa-live-voice"
	PluginName             = "cpa-live-voice"
	PluginVersion          = "0.1.0"
	PluginAuthor           = "karlorz"
	PluginGitHubRepository = "https://github.com/karlorz/cpa-live-voice"
	PluginLogo             = "https://github.com/karlorz/cpa-live-voice"

	StrategyFillFirst  = pluginapi.SchedulerBuiltinFillFirst
	StrategyRoundRobin = pluginapi.SchedulerBuiltinRoundRobin

	DefaultStrategy   = StrategyFillFirst
	DefaultFailClosed = true

	LiveProvider = "codex"
	LiveModel    = "gpt-live-1-codex"

	MaxHistorySize     = 32
	MaxValidateReqBody = 64 * 1024 // 64 KiB
)

// RawConfig represents raw plugin configuration parsed from YAML/JSON.
type RawConfig struct {
	LiveAuthIDs []string `yaml:"live_auth_ids" json:"live_auth_ids"`
	Strategy    string   `yaml:"strategy" json:"strategy"`
	FailClosed  *bool    `yaml:"fail_closed" json:"fail_closed"`
	// Host-owned fields are safely ignored by plugin configuration decoding.
	Enabled  *bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	Priority *int  `yaml:"priority,omitempty" json:"priority,omitempty"`
}

// Config represents validated, normalized plugin configuration.
type Config struct {
	LiveAuthIDs []string `json:"live_auth_ids"`
	Strategy    string   `json:"strategy"`
	FailClosed  bool     `json:"fail_closed"`
}

// DecisionOutcome classifies the scheduler routing decision.
type DecisionOutcome string

const (
	OutcomeSelected  DecisionOutcome = "selected"
	OutcomeDelegated DecisionOutcome = "delegated"
	OutcomeRejected  DecisionOutcome = "rejected"
)

// DecisionRecord holds one sanitized, bounded decision history entry.
type DecisionRecord struct {
	ID             int64           `json:"id"`
	Timestamp      string          `json:"timestamp"`
	Classification string          `json:"classification"`
	Outcome        DecisionOutcome `json:"outcome"`
	CandidateCount int             `json:"candidate_count"`
	MatchingCount  int             `json:"matching_count"`
	Reason         string          `json:"reason"`
}

// StatusResponse is returned by GET /plugins/cpa-live-voice/status.
type StatusResponse struct {
	PluginID        string           `json:"plugin_id"`
	Version         string           `json:"version"`
	Config          Config           `json:"config"`
	PoolSize        int              `json:"pool_size"`
	TotalDecisions  int64            `json:"total_decisions"`
	LiveDecisions   int64            `json:"live_decisions"`
	NormalDecisions int64            `json:"normal_decisions"`
	SelectedCount   int64            `json:"selected_count"`
	DelegatedCount  int64            `json:"delegated_count"`
	RejectedCount   int64            `json:"rejected_count"`
	RecentDecisions []DecisionRecord `json:"recent_decisions"`
	SystemTime      string           `json:"system_time"`
}

// ValidateRequest represents an operator candidate inventory verification payload.
type ValidateRequest struct {
	Candidates []ValidateCandidate `json:"candidates"`
}

// ValidateCandidate describes one candidate identifier and status for verification.
type ValidateCandidate struct {
	ID     string `json:"id"`
	Status string `json:"status,omitempty"`
}

// ValidateResponse reports matching, missing, and extra candidate IDs.
type ValidateResponse struct {
	ConfiguredCount int      `json:"configured_count"`
	MatchingCount   int      `json:"matching_count"`
	MissingCount    int      `json:"missing_count"`
	CandidateCount  int      `json:"candidate_count"`
	ConfiguredIDs   []string `json:"configured_ids"`
	MatchingIDs     []string `json:"matching_ids"`
	MissingIDs      []string `json:"missing_ids"`
	ExtraIDs        []string `json:"extra_ids"`
	PoolHealth      string   `json:"pool_health"`
	Message         string   `json:"message"`
}

// ExampleConfigResponse is returned by GET /plugins/cpa-live-voice/config-example.
type ExampleConfigResponse struct {
	YAML   string `json:"yaml"`
	Config Config `json:"config"`
}

// envelope represents the generic JSON ABI envelope exchanged with CPA host.
type envelope struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *envelopeError  `json:"error,omitempty"`
}

type envelopeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type lifecycleRequest struct {
	ConfigYAML []byte `json:"config_yaml"`
}

type registration struct {
	SchemaVersion uint32                 `json:"schema_version"`
	Metadata      pluginapi.Metadata     `json:"metadata"`
	Capabilities  registrationCapability `json:"capabilities"`
}

type registrationCapability struct {
	Scheduler     bool `json:"scheduler"`
	ManagementAPI bool `json:"management_api"`
}

type managementRegistrationResponse struct {
	Routes    []pluginapi.ManagementRoute `json:"routes,omitempty"`
	Resources []pluginapi.ResourceRoute   `json:"resources,omitempty"`
}
