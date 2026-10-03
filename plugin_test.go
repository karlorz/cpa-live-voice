package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginabi"
	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"
)

// 1. Config normalization and validation tests
func TestConfigNormalization(t *testing.T) {
	tests := []struct {
		name        string
		rawYAML     string
		wantIDs     []string
		wantStrat   string
		wantFailCl  bool
		wantErr     bool
		errContains string
	}{
		{
			name: "valid fill-first with duplicates and whitespace",
			rawYAML: `
live_auth_ids:
  - "  codex-1  "
  - "codex-2"
  - "codex-1"
  - ""
  - "   "
strategy: "Fill-First"
fail_closed: true
`,
			wantIDs:    []string{"codex-1", "codex-2"},
			wantStrat:  "fill-first",
			wantFailCl: true,
			wantErr:    false,
		},
		{
			name: "valid round-robin with defaults",
			rawYAML: `
live_auth_ids:
  - "codex-a"
`,
			wantIDs:    []string{"codex-a"},
			wantStrat:  "fill-first",
			wantFailCl: true,
			wantErr:    false,
		},
		{
			name: "fail_closed false explicitly set",
			rawYAML: `
live_auth_ids:
  - "codex-a"
strategy: "round-robin"
fail_closed: false
`,
			wantIDs:    []string{"codex-a"},
			wantStrat:  "round-robin",
			wantFailCl: false,
			wantErr:    false,
		},
		{
			name: "ignores host-owned fields safely",
			rawYAML: `
enabled: true
priority: 50
live_auth_ids:
  - "auth-1"
`,
			wantIDs:    []string{"auth-1"},
			wantStrat:  "fill-first",
			wantFailCl: true,
			wantErr:    false,
		},
		{
			name: "rejects empty final pool",
			rawYAML: `
live_auth_ids:
  - "   "
  - ""
`,
			wantErr:     true,
			errContains: "live_auth_ids cannot be empty",
		},
		{
			name: "rejects invalid strategy",
			rawYAML: `
live_auth_ids:
  - "auth-1"
strategy: "random"
`,
			wantErr:     true,
			errContains: "invalid strategy",
		},
		{
			name:        "rejects empty config",
			rawYAML:     "",
			wantErr:     true,
			errContains: "required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := ParseConfig([]byte(tt.rawYAML))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errContains)
				}
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(cfg.LiveAuthIDs) != len(tt.wantIDs) {
				t.Fatalf("got live_auth_ids = %v, want %v", cfg.LiveAuthIDs, tt.wantIDs)
			}
			for i, id := range cfg.LiveAuthIDs {
				if id != tt.wantIDs[i] {
					t.Errorf("live_auth_ids[%d] = %q, want %q", i, id, tt.wantIDs[i])
				}
			}
			if cfg.Strategy != tt.wantStrat {
				t.Errorf("strategy = %q, want %q", cfg.Strategy, tt.wantStrat)
			}
			if cfg.FailClosed != tt.wantFailCl {
				t.Errorf("fail_closed = %v, want %v", cfg.FailClosed, tt.wantFailCl)
			}
		})
	}
}

// 2. Request classification tests
func TestClassifier(t *testing.T) {
	tests := []struct {
		name       string
		provider   string
		providers  []string
		model      string
		candidates []pluginapi.SchedulerAuthCandidate
		wantLive   bool
	}{
		{
			name:     "exact match primary provider and model",
			provider: "codex",
			model:    "gpt-live-1-codex",
			wantLive: true,
		},
		{
			name:     "case-insensitive match",
			provider: "CODEX",
			model:    "GPT-LIVE-1-CODEX",
			wantLive: true,
		},
		{
			name:      "match in providers array",
			provider:  "",
			providers: []string{"openai", "codex"},
			model:     "gpt-live-1-codex",
			wantLive:  true,
		},
		{
			name:     "CPA Live OAuth-only call shape with omitted model",
			provider: "codex",
			candidates: []pluginapi.SchedulerAuthCandidate{
				{ID: "oauth-a", Attributes: map[string]string{"auth_kind": "oauth"}},
			},
			wantLive: true,
		},
		{
			name:     "ordinary empty-model API key call",
			provider: "codex",
			candidates: []pluginapi.SchedulerAuthCandidate{
				{ID: "api-a", Attributes: map[string]string{"auth_kind": "apikey"}},
			},
			wantLive: false,
		},
		{
			name:     "empty-model Codex PAT plus OAuth still classified live",
			provider: "codex",
			candidates: []pluginapi.SchedulerAuthCandidate{
				{ID: "codex-pat-team.json", Attributes: map[string]string{"auth_kind": "pat"}},
				{ID: "codex-plus.json", Attributes: map[string]string{"auth_kind": "oauth"}},
			},
			wantLive: true,
		},
		{
			name:     "different model for codex",
			provider: "codex",
			model:    "gpt-4o",
			wantLive: false,
		},
		{
			name:     "different provider for gpt-live-1-codex",
			provider: "openai",
			model:    "gpt-live-1-codex",
			wantLive: false,
		},
		{
			name:     "empty fields",
			provider: "",
			model:    "",
			wantLive: false,
		},
		{
			name:     "empty-model call without candidates",
			provider: "codex",
			model:    "",
			wantLive: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := pluginapi.SchedulerPickRequest{
				Provider:   tt.provider,
				Providers:  tt.providers,
				Model:      tt.model,
				Candidates: tt.candidates,
			}
			got := IsLiveRequest(req)
			if got != tt.wantLive {
				t.Errorf("IsLiveRequest() = %v, want %v", got, tt.wantLive)
			}
		})
	}
}

// 3. Scheduler routing tests
func TestSchedulerRouting(t *testing.T) {
	history := NewDecisionTracker(16)
	cfg := Config{
		LiveAuthIDs: []string{"auth-live-1", "auth-live-2"},
		Strategy:    StrategyFillFirst,
		FailClosed:  true,
	}
	scheduler := NewSchedulerCore(cfg, history)

	t.Run("Normal traffic delegates preserving built-in strategy and candidates", func(t *testing.T) {
		req := pluginapi.SchedulerPickRequest{
			Provider: "codex",
			Model:    "gpt-5",
			Candidates: []pluginapi.SchedulerAuthCandidate{
				{ID: "auth-pat-1"},
				{ID: "auth-pat-2"},
			},
		}
		resp, err := scheduler.PickAuth(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !resp.Handled {
			t.Fatal("normal traffic must return handled=true to delegate")
		}
		if resp.DelegateBuiltin != StrategyFillFirst {
			t.Errorf("delegate_builtin = %q, want %q", resp.DelegateBuiltin, StrategyFillFirst)
		}
		if resp.AuthID != "" {
			t.Errorf("normal traffic must not select auth_id directly, got %q", resp.AuthID)
		}
	})

	t.Run("Live traffic selects first matching candidate (fill-first)", func(t *testing.T) {
		req := pluginapi.SchedulerPickRequest{
			Provider: "codex",
			Model:    "gpt-live-1-codex",
			Candidates: []pluginapi.SchedulerAuthCandidate{
				{ID: "auth-other"},
				{ID: "auth-live-2"},
				{ID: "auth-live-1"},
			},
		}
		resp, err := scheduler.PickAuth(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !resp.Handled {
			t.Fatal("live traffic must return handled=true")
		}
		if resp.AuthID != "auth-live-2" {
			t.Errorf("auth_id = %q, want first match in host candidate order: auth-live-2", resp.AuthID)
		}
	})

	t.Run("Live traffic round-robin rotation across matches", func(t *testing.T) {
		rrScheduler := NewSchedulerCore(Config{
			LiveAuthIDs: []string{"live-a", "live-b", "live-c"},
			Strategy:    StrategyRoundRobin,
			FailClosed:  true,
		}, NewDecisionTracker(16))

		req := pluginapi.SchedulerPickRequest{
			Provider: "codex",
			Model:    "gpt-live-1-codex",
			Candidates: []pluginapi.SchedulerAuthCandidate{
				{ID: "live-b"},
				{ID: "live-c"},
			},
		}

		picks := make([]string, 4)
		for i := 0; i < 4; i++ {
			resp, err := rrScheduler.PickAuth(req)
			if err != nil {
				t.Fatalf("pick %d error: %v", i, err)
			}
			picks[i] = resp.AuthID
		}

		// Candidates present are live-b, live-c.
		// Round-robin should alternate: live-b, live-c, live-b, live-c
		expected := []string{"live-b", "live-c", "live-b", "live-c"}
		for i := 0; i < 4; i++ {
			if picks[i] != expected[i] {
				t.Errorf("pick %d = %q, want %q", i, picks[i], expected[i])
			}
		}
	})

	t.Run("Live traffic matches basename of host candidate ID", func(t *testing.T) {
		req := pluginapi.SchedulerPickRequest{
			Provider: "codex",
			Model:    "gpt-live-1-codex",
			Candidates: []pluginapi.SchedulerAuthCandidate{
				{ID: "codex-pat-team.json"},
				{
					ID: "/root/.cli-proxy-api/auth-live-2",
					Attributes: map[string]string{
						"path": "/root/.cli-proxy-api/auth-live-2",
					},
				},
			},
		}
		resp, err := scheduler.PickAuth(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.AuthID != "/root/.cli-proxy-api/auth-live-2" {
			t.Errorf("auth_id = %q, want host candidate ID whose basename is allowlisted", resp.AuthID)
		}
	})

	t.Run("Live traffic selects allowlisted OAuth among higher-priority PAT candidates", func(t *testing.T) {
		req := pluginapi.SchedulerPickRequest{
			Provider: "codex",
			Model:    "",
			Candidates: []pluginapi.SchedulerAuthCandidate{
				{ID: "codex-pat-team.json", Attributes: map[string]string{"auth_kind": "pat"}, Priority: 100},
				{ID: "auth-live-1", Attributes: map[string]string{"auth_kind": "oauth"}, Priority: 99},
			},
		}
		resp, err := scheduler.PickAuth(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.AuthID != "auth-live-1" {
			t.Errorf("auth_id = %q, want allowlisted oauth candidate auth-live-1", resp.AuthID)
		}
	})

	t.Run("Live traffic no match fail-closed", func(t *testing.T) {
		req := pluginapi.SchedulerPickRequest{
			Provider: "codex",
			Model:    "gpt-live-1-codex",
			Candidates: []pluginapi.SchedulerAuthCandidate{
				{ID: "unrelated-1"},
				{ID: "unrelated-2"},
			},
		}
		resp, err := scheduler.PickAuth(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !resp.Handled {
			t.Fatal("handled must be true")
		}
		if !resp.Reject {
			t.Fatal("reject must be true when fail_closed=true and no match")
		}
		if resp.RejectCode != RejectCodeLiveUnavailable {
			t.Errorf("reject_code = %q, want %q", resp.RejectCode, RejectCodeLiveUnavailable)
		}
		if resp.RejectReason != SanitizedReasonLiveNoAuth {
			t.Errorf("reject_reason = %q, want %q", resp.RejectReason, SanitizedReasonLiveNoAuth)
		}
	})

	t.Run("Live traffic no match fail-open", func(t *testing.T) {
		failOpenScheduler := NewSchedulerCore(Config{
			LiveAuthIDs: []string{"auth-live-1"},
			Strategy:    StrategyRoundRobin,
			FailClosed:  false,
		}, NewDecisionTracker(16))

		req := pluginapi.SchedulerPickRequest{
			Provider: "codex",
			Model:    "gpt-live-1-codex",
			Candidates: []pluginapi.SchedulerAuthCandidate{
				{ID: "unrelated-1"},
			},
		}
		resp, err := failOpenScheduler.PickAuth(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !resp.Handled {
			t.Fatal("handled must be true")
		}
		if resp.Reject {
			t.Fatal("reject must be false when fail_closed=false")
		}
		if resp.DelegateBuiltin != StrategyRoundRobin {
			t.Errorf("delegate_builtin = %q, want %q", resp.DelegateBuiltin, StrategyRoundRobin)
		}
	})
}

// 4. Decision history and redaction tests
func TestDecisionHistoryBoundedAndRedacted(t *testing.T) {
	tracker := NewDecisionTracker(4) // small bounded size

	for i := 1; i <= 6; i++ {
		tracker.Record("live", OutcomeSelected, 10, 1, "test reason")
	}

	records, total, live, normal, selected, delegated, rejected := tracker.Snapshot()

	if total != 6 {
		t.Errorf("total = %d, want 6", total)
	}
	if live != 6 || normal != 0 || selected != 6 || delegated != 0 || rejected != 0 {
		t.Errorf("unexpected counters: live=%d normal=%d selected=%d", live, normal, selected)
	}
	if len(records) != 4 {
		t.Fatalf("records count = %d, want bounded max 4", len(records))
	}

	// Newest first order.
	if records[0].ID != 6 {
		t.Errorf("newest record ID = %d, want 6", records[0].ID)
	}
	if records[3].ID != 3 {
		t.Errorf("oldest record ID = %d, want 3", records[3].ID)
	}

	// Security: Verify JSON representation never contains prohibited fields
	raw, err := json.Marshal(records)
	if err != nil {
		t.Fatalf("json marshal error: %v", err)
	}
	rawStr := string(raw)
	for _, forbidden := range []string{"token", "authorization", "secret", "sdp", "password", "headers", "cookie"} {
		if strings.Contains(strings.ToLower(rawStr), forbidden) {
			t.Errorf("decision history leaked forbidden key %q in: %s", forbidden, rawStr)
		}
	}
}

// 5. Concurrency & race safety test
func TestConcurrencySafety(t *testing.T) {
	history := NewDecisionTracker(MaxHistorySize)
	cfg := Config{
		LiveAuthIDs: []string{"c-1", "c-2", "c-3"},
		Strategy:    StrategyRoundRobin,
		FailClosed:  true,
	}
	scheduler := NewSchedulerCore(cfg, history)

	var wg sync.WaitGroup
	workers := 20
	iterations := 50

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				// Reconfigure dynamically in one worker
				if workerID == 0 && i%10 == 0 {
					scheduler.UpdateConfig(Config{
						LiveAuthIDs: []string{"c-1", "c-2", "c-3"},
						Strategy:    StrategyRoundRobin,
						FailClosed:  true,
					})
				}

				req := pluginapi.SchedulerPickRequest{
					Provider: "codex",
					Model:    "gpt-live-1-codex",
					Candidates: []pluginapi.SchedulerAuthCandidate{
						{ID: "c-1"},
						{ID: "c-2"},
					},
				}
				resp, err := scheduler.PickAuth(req)
				if err != nil || !resp.Handled {
					t.Errorf("worker %d iteration %d pick failed: %v", workerID, i, err)
				}
				if resp.AuthID != "c-1" && resp.AuthID != "c-2" {
					t.Errorf("unexpected pick: %q", resp.AuthID)
				}

				// Concurrent read from history
				history.Snapshot()
			}
		}(w)
	}

	wg.Wait()
}

// 6. Management routes and UI security tests
func TestManagementEndpoints(t *testing.T) {
	history := NewDecisionTracker(16)
	cfg := Config{
		LiveAuthIDs: []string{"alpha", "beta"},
		Strategy:    StrategyFillFirst,
		FailClosed:  true,
	}
	scheduler := NewSchedulerCore(cfg, history)
	mgmt := NewManagementHandler(scheduler, history)

	t.Run("GET status returns clean JSON", func(t *testing.T) {
		req := pluginapi.ManagementRequest{
			Method: http.MethodGet,
			Path:   RouteStatus,
		}
		resp, err := mgmt.Handle(req)
		if err != nil {
			t.Fatalf("Handle() error: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		var st StatusResponse
		if errUnmarshal := json.Unmarshal(resp.Body, &st); errUnmarshal != nil {
			t.Fatalf("unmarshal error: %v; body: %s", errUnmarshal, string(resp.Body))
		}
		if st.PoolSize != 2 || st.Config.Strategy != StrategyFillFirst {
			t.Errorf("unexpected status response: %+v", st)
		}
	})

	t.Run("POST validate reports inventory health accurately", func(t *testing.T) {
		body := `{"candidates": [{"id": "alpha"}, {"id": "gamma"}]}`
		req := pluginapi.ManagementRequest{
			Method: http.MethodPost,
			Path:   RouteValidate,
			Body:   []byte(body),
		}
		resp, err := mgmt.Handle(req)
		if err != nil {
			t.Fatalf("Handle() error: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		var valResp ValidateResponse
		if errUnmarshal := json.Unmarshal(resp.Body, &valResp); errUnmarshal != nil {
			t.Fatalf("unmarshal error: %v", errUnmarshal)
		}
		if valResp.ConfiguredCount != 2 || valResp.MatchingCount != 1 || valResp.MissingCount != 1 {
			t.Errorf("unexpected validate response: %+v", valResp)
		}
		if valResp.PoolHealth != "degraded" {
			t.Errorf("pool_health = %q, want degraded", valResp.PoolHealth)
		}
		if len(valResp.MissingIDs) != 1 || valResp.MissingIDs[0] != "beta" {
			t.Errorf("missing_ids = %v, want ['beta']", valResp.MissingIDs)
		}
		if len(valResp.ExtraIDs) != 1 || valResp.ExtraIDs[0] != "gamma" {
			t.Errorf("extra_ids = %v, want ['gamma']", valResp.ExtraIDs)
		}
	})

	t.Run("POST validate limits body size", func(t *testing.T) {
		largeBody := make([]byte, MaxValidateReqBody+100)
		req := pluginapi.ManagementRequest{
			Method: http.MethodPost,
			Path:   RouteValidate,
			Body:   largeBody,
		}
		resp, err := mgmt.Handle(req)
		if err != nil {
			t.Fatalf("Handle() error: %v", err)
		}
		if resp.StatusCode != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusRequestEntityTooLarge)
		}
	})

	t.Run("GET config-example returns clean example", func(t *testing.T) {
		req := pluginapi.ManagementRequest{
			Method: http.MethodGet,
			Path:   RouteConfigExample,
		}
		resp, err := mgmt.Handle(req)
		if err != nil {
			t.Fatalf("Handle() error: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		var ex ExampleConfigResponse
		if errUnmarshal := json.Unmarshal(resp.Body, &ex); errUnmarshal != nil {
			t.Fatalf("unmarshal error: %v", errUnmarshal)
		}
		if !strings.Contains(ex.YAML, "cpa-live-voice") {
			t.Errorf("example yaml missing plugin id: %s", ex.YAML)
		}
	})

	t.Run("GET resource status page HTML security invariants", func(t *testing.T) {
		req := pluginapi.ManagementRequest{
			Method: http.MethodGet,
			Path:   ResourcePathStatus,
		}
		resp, err := mgmt.Handle(req)
		if err != nil {
			t.Fatalf("Handle() error: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		htmlContent := string(resp.Body)

		// Must not load external CDN scripts or fonts
		if strings.Contains(htmlContent, "http://") || strings.Contains(htmlContent, "https://cdn") || strings.Contains(htmlContent, "unpkg.com") || strings.Contains(htmlContent, "cdnjs") {
			t.Errorf("resource page contains external CDN links")
		}

		// Must store key in sessionStorage only
		if !strings.Contains(htmlContent, "sessionStorage") {
			t.Errorf("resource page missing sessionStorage")
		}
		if strings.Contains(htmlContent, "localStorage") {
			t.Errorf("resource page must not use localStorage for management key")
		}

		// Must include warning regarding scheduler ownership
		if !strings.Contains(htmlContent, "Scheduler Ownership") {
			t.Errorf("resource page missing scheduler ownership warning")
		}
	})
}

// 7. ABI call dispatch and lifecycle tests
func TestPluginABICallDispatch(t *testing.T) {
	// 1. Test register method
	rawYAML := []byte("live_auth_ids:\n  - codex-test\nstrategy: fill-first\nfail_closed: true\n")
	regReq := lifecycleRequest{
		ConfigYAML: rawYAML,
	}
	regPayload, _ := json.Marshal(regReq)
	rawReg, err := handlePluginMethod(pluginabi.MethodPluginRegister, regPayload)
	if err != nil {
		t.Fatalf("handlePluginMethod register error: %v", err)
	}

	var env envelope
	if errUnmarshal := json.Unmarshal(rawReg, &env); errUnmarshal != nil {
		t.Fatalf("unmarshal envelope error: %v", errUnmarshal)
	}
	if !env.OK {
		t.Fatalf("envelope error: %+v", env.Error)
	}

	var reg registration
	if errUnmarshal := json.Unmarshal(env.Result, &reg); errUnmarshal != nil {
		t.Fatalf("unmarshal registration error: %v", errUnmarshal)
	}
	if reg.Metadata.Name != PluginName || reg.Metadata.Version != PluginVersion {
		t.Errorf("registration metadata mismatch: %+v", reg.Metadata)
	}
	if !reg.Capabilities.Scheduler || !reg.Capabilities.SchedulerAcrossPriorities || !reg.Capabilities.ManagementAPI {
		t.Errorf("registration capabilities mismatch: %+v", reg.Capabilities)
	}

	// 2. Test scheduler pick via ABI
	pickReq := pluginapi.SchedulerPickRequest{
		Provider: "codex",
		Model:    "gpt-live-1-codex",
		Candidates: []pluginapi.SchedulerAuthCandidate{
			{ID: "codex-test"},
		},
	}
	rawPickReq, _ := json.Marshal(pickReq)
	rawPickResp, err := handlePluginMethod(pluginabi.MethodSchedulerPick, rawPickReq)
	if err != nil {
		t.Fatalf("handlePluginMethod scheduler.pick error: %v", err)
	}

	var pickEnv envelope
	if errUnmarshal := json.Unmarshal(rawPickResp, &pickEnv); errUnmarshal != nil {
		t.Fatalf("unmarshal pick envelope error: %v", errUnmarshal)
	}
	if !pickEnv.OK {
		t.Fatalf("pick envelope not ok: %+v", pickEnv.Error)
	}
	var pickResult pluginapi.SchedulerPickResponse
	if errUnmarshal := json.Unmarshal(pickEnv.Result, &pickResult); errUnmarshal != nil {
		t.Fatalf("unmarshal pick result error: %v", errUnmarshal)
	}
	if pickResult.AuthID != "codex-test" {
		t.Errorf("pick auth_id = %q, want codex-test", pickResult.AuthID)
	}
}
