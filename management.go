package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"
	"gopkg.in/yaml.v3"
)

const (
	RouteStatus        = "/plugins/cpa-live-voice/status"
	RouteValidate      = "/plugins/cpa-live-voice/validate"
	RouteConfigExample = "/plugins/cpa-live-voice/config-example"

	ResourcePathStatus = "/status"
)

// ManagementHandler handles management routes and UI resource routes.
type ManagementHandler struct {
	scheduler *SchedulerCore
	history   *DecisionTracker
}

func NewManagementHandler(scheduler *SchedulerCore, history *DecisionTracker) *ManagementHandler {
	return &ManagementHandler{
		scheduler: scheduler,
		history:   history,
	}
}

// Routes returns the ManagementRoute definitions to be registered with CPA.
func (m *ManagementHandler) Routes() []pluginapi.ManagementRoute {
	return []pluginapi.ManagementRoute{
		{
			Method:      http.MethodGet,
			Path:        RouteStatus,
			Description: "Returns status and recent routing decisions for cpa-live-voice plugin",
		},
		{
			Method:      http.MethodPost,
			Path:        RouteValidate,
			Description: "Validates configured live_auth_ids against candidate inventory",
		},
		{
			Method:      http.MethodGet,
			Path:        RouteConfigExample,
			Description: "Returns example configuration for cpa-live-voice plugin",
		},
	}
}

// Resources returns browser-navigable resource routes.
func (m *ManagementHandler) Resources() []pluginapi.ResourceRoute {
	return []pluginapi.ResourceRoute{
		{
			Path:        ResourcePathStatus,
			Menu:        "Live Voice Scheduler",
			Description: "Status, pool health, and recent decisions for CPA Live Voice",
		},
	}
}

// Handle processes incoming ManagementRequest.
func (m *ManagementHandler) Handle(req pluginapi.ManagementRequest) (pluginapi.ManagementResponse, error) {
	method := strings.ToUpper(strings.TrimSpace(req.Method))
	path := strings.TrimSpace(req.Path)

	// Normalize path by stripping management base prefix if present
	normalizedPath := path
	if strings.HasPrefix(normalizedPath, "/v0/management") {
		normalizedPath = strings.TrimPrefix(normalizedPath, "/v0/management")
	}

	// 1. Browser Resource Page: /v0/resource/plugins/cpa-live-voice/status or /status
	if method == http.MethodGet && (normalizedPath == ResourcePathStatus || strings.HasSuffix(normalizedPath, "/plugins/cpa-live-voice/status") && strings.HasPrefix(path, "/v0/resource/plugins/")) {
		return m.handleResourcePage()
	}

	// 2. Management API endpoints
	switch {
	case method == http.MethodGet && normalizedPath == RouteStatus:
		return m.handleStatus()
	case method == http.MethodPost && normalizedPath == RouteValidate:
		return m.handleValidate(req)
	case method == http.MethodGet && normalizedPath == RouteConfigExample:
		return m.handleConfigExample()
	default:
		return pluginapi.ManagementResponse{
			StatusCode: http.StatusNotFound,
			Headers:    http.Header{"Content-Type": []string{"application/json; charset=utf-8"}},
			Body:       []byte(`{"error":"not_found","message":"route not found"}`),
		}, nil
	}
}

func (m *ManagementHandler) handleStatus() (pluginapi.ManagementResponse, error) {
	cfg := m.scheduler.CurrentConfig()
	records, total, live, normal, selected, delegated, rejected := m.history.Snapshot()

	resp := StatusResponse{
		PluginID:        PluginID,
		Version:         PluginVersion,
		Config:          cfg,
		PoolSize:        len(cfg.LiveAuthIDs),
		TotalDecisions:  total,
		LiveDecisions:   live,
		NormalDecisions: normal,
		SelectedCount:   selected,
		DelegatedCount:  delegated,
		RejectedCount:   rejected,
		RecentDecisions: records,
		SystemTime:      time.Now().UTC().Format(time.RFC3339),
	}

	raw, errMarshal := json.Marshal(resp)
	if errMarshal != nil {
		return pluginapi.ManagementResponse{
			StatusCode: http.StatusInternalServerError,
			Headers:    http.Header{"Content-Type": []string{"application/json; charset=utf-8"}},
			Body:       []byte(fmt.Sprintf(`{"error":"marshal_error","message":%q}`, errMarshal.Error())),
		}, nil
	}

	return pluginapi.ManagementResponse{
		StatusCode: http.StatusOK,
		Headers:    http.Header{"Content-Type": []string{"application/json; charset=utf-8"}},
		Body:       raw,
	}, nil
}

func (m *ManagementHandler) handleValidate(req pluginapi.ManagementRequest) (pluginapi.ManagementResponse, error) {
	if len(req.Body) > MaxValidateReqBody {
		return pluginapi.ManagementResponse{
			StatusCode: http.StatusRequestEntityTooLarge,
			Headers:    http.Header{"Content-Type": []string{"application/json; charset=utf-8"}},
			Body:       []byte(`{"error":"payload_too_large","message":"request body exceeds 64KB limit"}`),
		}, nil
	}

	var valReq ValidateRequest
	if len(req.Body) > 0 {
		if errUnmarshal := json.Unmarshal(req.Body, &valReq); errUnmarshal != nil {
			return pluginapi.ManagementResponse{
				StatusCode: http.StatusBadRequest,
				Headers:    http.Header{"Content-Type": []string{"application/json; charset=utf-8"}},
				Body:       []byte(fmt.Sprintf(`{"error":"invalid_json","message":%q}`, errUnmarshal.Error())),
			}, nil
		}
	}

	cfg := m.scheduler.CurrentConfig()
	configuredSet := make(map[string]struct{}, len(cfg.LiveAuthIDs))
	for _, id := range cfg.LiveAuthIDs {
		configuredSet[id] = struct{}{}
	}

	candidateSet := make(map[string]struct{}, len(valReq.Candidates)*2)
	var candidateIDs []string
	for _, c := range valReq.Candidates {
		trimmed := strings.TrimSpace(c.ID)
		if trimmed == "" {
			continue
		}
		if _, exists := candidateSet[trimmed]; !exists {
			candidateIDs = append(candidateIDs, trimmed)
		}
		for _, alias := range uniqueIDAliases(trimmed) {
			candidateSet[alias] = struct{}{}
		}
	}

	var matchingIDs []string
	var missingIDs []string
	for _, id := range cfg.LiveAuthIDs {
		if configuredIDMatchesInventory(id, candidateSet) {
			matchingIDs = append(matchingIDs, id)
		} else {
			missingIDs = append(missingIDs, id)
		}
	}

	var extraIDs []string
	for _, id := range candidateIDs {
		if !configuredIDMatchesInventory(id, configuredSet) {
			extraIDs = append(extraIDs, id)
		}
	}

	poolHealth := "healthy"
	msg := "All configured live auth IDs are available in candidate inventory"
	if len(matchingIDs) == 0 {
		poolHealth = "critical"
		msg = "No configured live auth IDs match the candidate inventory. Live requests will fail closed if fail_closed=true."
	} else if len(missingIDs) > 0 {
		poolHealth = "degraded"
		msg = fmt.Sprintf("%d of %d configured live auth IDs are missing from candidate inventory", len(missingIDs), len(cfg.LiveAuthIDs))
	}

	resp := ValidateResponse{
		ConfiguredCount: len(cfg.LiveAuthIDs),
		MatchingCount:   len(matchingIDs),
		MissingCount:    len(missingIDs),
		CandidateCount:  len(candidateIDs),
		ConfiguredIDs:   cfg.LiveAuthIDs,
		MatchingIDs:     matchingIDs,
		MissingIDs:      missingIDs,
		ExtraIDs:        extraIDs,
		PoolHealth:      poolHealth,
		Message:         msg,
	}

	raw, _ := json.Marshal(resp)
	return pluginapi.ManagementResponse{
		StatusCode: http.StatusOK,
		Headers:    http.Header{"Content-Type": []string{"application/json; charset=utf-8"}},
		Body:       raw,
	}, nil
}

func (m *ManagementHandler) handleConfigExample() (pluginapi.ManagementResponse, error) {
	exampleYAML := `# Configuration for cpa-live-voice plugin
# Place under plugins.configs.cpa-live-voice in your CPA config.yaml
plugins:
  enabled: true
  configs:
    cpa-live-voice:
      enabled: true
      priority: 100 # Sole active scheduler: priority above other scheduler plugins
      live_auth_ids:
        - "codex-account-primary"
        - "codex-account-secondary"
      strategy: "fill-first" # or "round-robin"
      fail_closed: true     # reject with live_oauth_unavailable when no match
`

	resp := ExampleConfigResponse{
		YAML: exampleYAML,
		Config: Config{
			LiveAuthIDs: []string{"codex-account-primary", "codex-account-secondary"},
			Strategy:    StrategyFillFirst,
			FailClosed:  true,
		},
	}

	raw, _ := json.Marshal(resp)
	return pluginapi.ManagementResponse{
		StatusCode: http.StatusOK,
		Headers:    http.Header{"Content-Type": []string{"application/json; charset=utf-8"}},
		Body:       raw,
	}, nil
}

func (m *ManagementHandler) handleResourcePage() (pluginapi.ManagementResponse, error) {
	return pluginapi.ManagementResponse{
		StatusCode: http.StatusOK,
		Headers:    http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
		Body:       []byte(ResourcePageHTML),
	}, nil
}

// GenerateExampleYAML produces sample YAML for documentation or config templates.
func GenerateExampleYAML() string {
	raw := RawConfig{
		LiveAuthIDs: []string{"codex-account-1", "codex-account-2"},
		Strategy:    StrategyFillFirst,
	}
	f := true
	raw.FailClosed = &f
	out, _ := yaml.Marshal(map[string]any{
		"plugins": map[string]any{
			"enabled": true,
			"configs": map[string]any{
				PluginID: map[string]any{
					"enabled":       true,
					"priority":      100,
					"live_auth_ids": raw.LiveAuthIDs,
					"strategy":      raw.Strategy,
					"fail_closed":   *raw.FailClosed,
				},
			},
		},
	})
	return string(out)
}
