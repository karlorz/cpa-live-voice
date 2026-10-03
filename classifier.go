package main

import (
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"
)

// IsLiveRequest reports whether a scheduler request came from CPA's Codex Live path.
func IsLiveRequest(req pluginapi.SchedulerPickRequest) bool {
	if !hasProvider(req, LiveProvider) {
		return false
	}

	model := strings.ToLower(strings.TrimSpace(req.Model))
	if model == LiveModel {
		return true
	}
	if model != "" || len(req.Candidates) == 0 {
		return false
	}

	// CPA v8.0.12 deliberately omits the model when its Live handler calls
	// SelectAuthByKind. That filter uses AuthKind(), which treats Codex PAT
	// files with access_token metadata as oauth even when auth_kind=pat.
	// Only an explicit API-key marker means this empty-model call is ordinary.
	for _, candidate := range req.Candidates {
		kind := strings.ToLower(strings.TrimSpace(candidate.Attributes["auth_kind"]))
		if isExplicitAPIKeyKind(kind) {
			return false
		}
	}
	return true
}

func isExplicitAPIKeyKind(kind string) bool {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "apikey", "api_key", "api-key":
		return true
	default:
		return false
	}
}

func hasProvider(req pluginapi.SchedulerPickRequest, provider string) bool {
	if strings.EqualFold(strings.TrimSpace(req.Provider), provider) {
		return true
	}
	for _, candidate := range req.Providers {
		if strings.EqualFold(strings.TrimSpace(candidate), provider) {
			return true
		}
	}
	return false
}
