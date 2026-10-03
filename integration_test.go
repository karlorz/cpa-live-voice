package main

import (
	"context"
	"net/http"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v8/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v8/sdk/cliproxy/executor"
	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"
	cpapluginhost "github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginhost"
	"gopkg.in/yaml.v3"
)

// pluginSchedulerAdapter adapts the in-memory or ABI-based plugin to auth.PluginScheduler
type testPluginSchedulerAdapter struct {
	scheduler *SchedulerCore
}

func (a *testPluginSchedulerAdapter) PickAuth(ctx context.Context, req pluginapi.SchedulerPickRequest) (pluginapi.SchedulerPickResponse, bool, error) {
	resp, err := a.scheduler.PickAuth(req)
	if err != nil {
		return resp, true, err
	}
	return resp, true, nil
}

type dummyCodexExecutor struct{}

func (dummyCodexExecutor) Identifier() string {
	return "codex"
}

func (dummyCodexExecutor) Execute(ctx context.Context, a *auth.Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	return cliproxyexecutor.Response{}, nil
}

func (dummyCodexExecutor) ExecuteStream(ctx context.Context, a *auth.Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (*cliproxyexecutor.StreamResult, error) {
	return nil, nil
}

func (dummyCodexExecutor) Refresh(ctx context.Context, a *auth.Auth) (*auth.Auth, error) {
	return a, nil
}

func (dummyCodexExecutor) CountTokens(ctx context.Context, a *auth.Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	return cliproxyexecutor.Response{}, nil
}

func (dummyCodexExecutor) HttpRequest(ctx context.Context, a *auth.Auth, req *http.Request) (*http.Response, error) {
	return nil, nil
}

// TestCPAIntegration verifies end-to-end integration against real CPA SDK data structures
// (auth.Manager, candidate sets, built-in delegation, management handlers) in a temp directory without Docker.
func TestCPAIntegration(t *testing.T) {
	// 1. Initialize plugin core
	cfg := Config{
		LiveAuthIDs: []string{"live-account-1", "live-account-2"},
		Strategy:    StrategyFillFirst,
		FailClosed:  true,
	}
	history := NewDecisionTracker(32)
	schedulerCore := NewSchedulerCore(cfg, history)
	mgmtHandler := NewManagementHandler(schedulerCore, history)

	// 2. Initialize CPA auth.Manager with RoundRobinSelector
	authMgr := auth.NewManager(nil, &auth.RoundRobinSelector{}, nil)
	authMgr.RegisterExecutor(dummyCodexExecutor{})

	// Register dummy auth credentials in CPA Manager:
	// A live OAuth account and an ordinary PAT account
	liveAuth := &auth.Auth{
		ID:       "live-account-1",
		Provider: "codex",
		Attributes: map[string]string{
			auth.AttributeAuthKind: auth.AuthKindOAuth,
		},
		Metadata: map[string]any{"access_token": "fixture-oauth-value"},
	}
	patAuth := &auth.Auth{
		ID:       "pat-account-work",
		Provider: "codex",
		Attributes: map[string]string{
			auth.AttributeAPIKey:   "fixture-pat-value",
			auth.AttributeAuthKind: auth.AuthKindAPIKey,
		},
	}
	ctx := context.Background()
	_, _ = authMgr.Register(ctx, liveAuth)
	_, _ = authMgr.Register(ctx, patAuth)

	authMgr.SetPluginScheduler(&testPluginSchedulerAdapter{scheduler: schedulerCore})

	// Test 3: Normal text traffic delegates to CPA's built-in selector with PAT eligibility intact.
	t.Run("Normal request delegates to built-in and keeps PAT eligible", func(t *testing.T) {
		selected, err := authMgr.SelectAuth(context.Background(), "codex", "", cliproxyexecutor.Options{})
		if err != nil {
			t.Fatalf("SelectAuth normal request failed: %v", err)
		}
		if selected == nil {
			t.Fatal("selected auth is nil")
		}
		if selected.ID != "live-account-1" && selected.ID != "pat-account-work" {
			t.Fatalf("selected auth = %q, want a registered Codex candidate", selected.ID)
		}
	})

	// Test 4: CPA's local Live path filters to OAuth, then calls SelectAuthByKind with model="".
	t.Run("Live request selects allowlisted OAuth through CPA manager", func(t *testing.T) {
		selected, err := authMgr.SelectAuthByKind(context.Background(), "codex", "", auth.AuthKindOAuth, cliproxyexecutor.Options{})
		if err != nil {
			t.Fatalf("SelectAuthByKind live request failed: %v", err)
		}
		if selected == nil || selected.ID != "live-account-1" {
			t.Fatalf("selected auth = %#v, want live-account-1", selected)
		}
	})

	// Test 5: Live request fail_closed when configured live auths unavailable
	t.Run("Live request fails closed when pool mismatch", func(t *testing.T) {
		schedulerCore.UpdateConfig(Config{
			LiveAuthIDs: []string{"nonexistent-live-auth"},
			Strategy:    StrategyFillFirst,
			FailClosed:  true,
		})

		req := pluginapi.SchedulerPickRequest{
			Provider: "codex",
			Model:    "gpt-live-1-codex",
			Candidates: []pluginapi.SchedulerAuthCandidate{
				{ID: "live-account-1"},
				{ID: "pat-account-work"},
			},
		}
		resp, err := schedulerCore.PickAuth(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !resp.Reject || resp.RejectCode != RejectCodeLiveUnavailable {
			t.Fatalf("expected rejection with %q, got: %+v", RejectCodeLiveUnavailable, resp)
		}
	})

	// Test 6: Verify Management endpoints through httptest
	t.Run("Management status and validation through HTTP server", func(t *testing.T) {
		// Test GET /plugins/cpa-live-voice/status
		statusReq := pluginapi.ManagementRequest{
			Method: http.MethodGet,
			Path:   RouteStatus,
		}
		resp, err := mgmtHandler.Handle(statusReq)
		if err != nil {
			t.Fatalf("status handle failed: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		// Test POST /plugins/cpa-live-voice/validate
		valReq := pluginapi.ManagementRequest{
			Method: http.MethodPost,
			Path:   RouteValidate,
			Body:   []byte(`{"candidates": [{"id": "live-account-1"}]}`),
		}
		valResp, err := mgmtHandler.Handle(valReq)
		if err != nil {
			t.Fatalf("validate handle failed: %v", err)
		}
		if valResp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", valResp.StatusCode)
		}
	})
}

// TestDynamicLibraryHostLoad builds the plugin, loads it through CPA's native
// plugin host, and exercises registration plus scheduler dispatch through the ABI.
func TestDynamicLibraryHostLoad(t *testing.T) {
	tempDir := t.TempDir()
	ext := ".so"
	if runtime.GOOS == "darwin" {
		ext = ".dylib"
	} else if runtime.GOOS == "windows" {
		ext = ".dll"
	}

	libPath := filepath.Join(tempDir, PluginID+ext)
	cmd := exec.Command("go", "build", "-buildmode=c-shared", "-o", libPath, ".")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build -buildmode=c-shared failed: %v, output: %s", err, string(output))
	}

	var raw yaml.Node
	if errUnmarshal := yaml.Unmarshal([]byte("live_auth_ids:\n  - live-account-1\nstrategy: fill-first\nfail_closed: true\n"), &raw); errUnmarshal != nil {
		t.Fatalf("decode plugin config: %v", errUnmarshal)
	}
	if len(raw.Content) != 1 {
		t.Fatalf("plugin config document has %d nodes, want 1", len(raw.Content))
	}
	enabled := true
	host := cpapluginhost.New()
	host.ApplyConfig(context.Background(), cpapluginhost.RuntimeConfig{
		Enabled: true,
		Dir:     tempDir,
		Configs: map[string]cpapluginhost.PluginInstanceConfig{
			PluginID: {
				Enabled:  &enabled,
				Priority: 100,
				Raw:      *raw.Content[0],
			},
		},
	})
	// The library stays loaded for the process lifetime: dlclose of a Go
	// c-shared library leaves orphaned runtime threads and deadlocks this
	// process, so host.ShutdownAll must not run inside `go test`.
	if !host.HasScheduler() {
		t.Fatal("CPA host did not activate the plugin scheduler")
	}
	plugins := host.RegisteredPlugins()
	if len(plugins) != 1 || plugins[0].ID != PluginID || plugins[0].Metadata.Version != PluginVersion {
		t.Fatalf("registered plugins = %#v", plugins)
	}

	resp, handled, errPick := host.PickAuth(context.Background(), pluginapi.SchedulerPickRequest{
		Provider: "codex",
		Candidates: []pluginapi.SchedulerAuthCandidate{
			{ID: "live-account-1", Provider: "codex", Attributes: map[string]string{"auth_kind": "oauth"}},
		},
	})
	if errPick != nil {
		t.Fatalf("CPA host scheduler pick failed: %v", errPick)
	}
	if !handled || !resp.Handled || resp.AuthID != "live-account-1" {
		t.Fatalf("CPA host scheduler response = handled:%t %#v", handled, resp)
	}
}
