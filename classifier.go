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
	// SelectAuthByKind. At that point CPA has already restricted candidates to
	// OAuth credentials. Reject an explicit non-OAuth marker while accepting
	// legacy OAuth records that do not expose auth_kind to scheduler plugins.
	for _, candidate := range req.Candidates {
		kind := strings.ToLower(strings.TrimSpace(candidate.Attributes["auth_kind"]))
		if kind != "" && kind != "oauth" && kind != "oauth2" {
			return false
		}
	}
	return true
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
