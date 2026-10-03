package main

/*
#include <stdint.h>
#include <stdlib.h>

typedef struct {
	void* ptr;
	size_t len;
} cliproxy_buffer;

typedef struct {
	uint32_t abi_version;
	void* host_ctx;
	void* call;
	void* free_buffer;
} cliproxy_host_api;

typedef int (*cliproxy_plugin_call_fn)(char*, uint8_t*, size_t, cliproxy_buffer*);
typedef void (*cliproxy_plugin_free_fn)(void*, size_t);
typedef void (*cliproxy_plugin_shutdown_fn)(void);

typedef struct {
	uint32_t abi_version;
	cliproxy_plugin_call_fn call;
	cliproxy_plugin_free_fn free_buffer;
	cliproxy_plugin_shutdown_fn shutdown;
} cliproxy_plugin_api;

extern int cliproxyPluginCall(char*, uint8_t*, size_t, cliproxy_buffer*);
extern void cliproxyPluginFree(void*, size_t);
extern void cliproxyPluginShutdown(void);
*/
import "C"

import (
	"encoding/json"
	"sync"
	"unsafe"

	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginabi"
	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"
)

var (
	pluginMu          sync.RWMutex
	globalScheduler   *SchedulerCore
	globalHistory     *DecisionTracker
	globalMgmtHandler *ManagementHandler
)

func init() {
	globalHistory = NewDecisionTracker(MaxHistorySize)
	defaultCfg := Config{
		LiveAuthIDs: []string{"default"},
		Strategy:    DefaultStrategy,
		FailClosed:  DefaultFailClosed,
	}
	globalScheduler = NewSchedulerCore(defaultCfg, globalHistory)
	globalMgmtHandler = NewManagementHandler(globalScheduler, globalHistory)
}

func main() {}

//export cliproxy_plugin_init
func cliproxy_plugin_init(_ *C.cliproxy_host_api, plugin *C.cliproxy_plugin_api) C.int {
	if plugin == nil {
		return 1
	}
	plugin.abi_version = C.uint32_t(pluginabi.ABIVersion)
	plugin.call = C.cliproxy_plugin_call_fn(C.cliproxyPluginCall)
	plugin.free_buffer = C.cliproxy_plugin_free_fn(C.cliproxyPluginFree)
	plugin.shutdown = C.cliproxy_plugin_shutdown_fn(C.cliproxyPluginShutdown)
	return 0
}

//export cliproxyPluginCall
func cliproxyPluginCall(method *C.char, request *C.uint8_t, requestLen C.size_t, response *C.cliproxy_buffer) C.int {
	if response != nil {
		response.ptr = nil
		response.len = 0
	}
	if method == nil {
		writeResponse(response, errorEnvelope("invalid_method", "method is required"))
		return 1
	}
	var requestBytes []byte
	if request != nil && requestLen > 0 {
		requestBytes = C.GoBytes(unsafe.Pointer(request), C.int(requestLen))
	}
	raw, errHandle := handlePluginMethod(C.GoString(method), requestBytes)
	if errHandle != nil {
		writeResponse(response, errorEnvelope("plugin_error", errHandle.Error()))
		return 1
	}
	writeResponse(response, raw)
	return 0
}

//export cliproxyPluginFree
func cliproxyPluginFree(ptr unsafe.Pointer, _ C.size_t) {
	if ptr != nil {
		C.free(ptr)
	}
}

//export cliproxyPluginShutdown
func cliproxyPluginShutdown() {}

func handlePluginMethod(method string, request []byte) ([]byte, error) {
	switch method {
	case pluginabi.MethodPluginRegister, pluginabi.MethodPluginReconfigure:
		if errConfigure := configurePlugin(request); errConfigure != nil {
			return nil, errConfigure
		}
		return okEnvelope(buildPluginRegistration())
	case pluginabi.MethodSchedulerPick:
		return handleSchedulerPick(request)
	case pluginabi.MethodManagementRegister:
		return handleManagementRegister()
	case pluginabi.MethodManagementHandle:
		return handleManagementHandle(request)
	default:
		return errorEnvelope("unknown_method", "unknown method: "+method), nil
	}
}

func configurePlugin(raw []byte) error {
	var req lifecycleRequest
	if len(raw) > 0 {
		if errUnmarshal := json.Unmarshal(raw, &req); errUnmarshal != nil {
			return errUnmarshal
		}
	}

	if len(req.ConfigYAML) > 0 {
		cfg, errParse := ParseConfig(req.ConfigYAML)
		if errParse != nil {
			return errParse
		}
		pluginMu.Lock()
		globalScheduler.UpdateConfig(cfg)
		pluginMu.Unlock()
	}
	return nil
}

func handleSchedulerPick(raw []byte) ([]byte, error) {
	var req pluginapi.SchedulerPickRequest
	if len(raw) > 0 {
		if errUnmarshal := json.Unmarshal(raw, &req); errUnmarshal != nil {
			return nil, errUnmarshal
		}
	}

	pluginMu.RLock()
	scheduler := globalScheduler
	pluginMu.RUnlock()

	resp, errPick := scheduler.PickAuth(req)
	if errPick != nil {
		return errorEnvelope("scheduler_error", errPick.Error()), nil
	}
	return okEnvelope(resp)
}

func handleManagementRegister() ([]byte, error) {
	pluginMu.RLock()
	mgmt := globalMgmtHandler
	pluginMu.RUnlock()

	return okEnvelope(managementRegistrationResponse{
		Routes:    mgmt.Routes(),
		Resources: mgmt.Resources(),
	})
}

func handleManagementHandle(raw []byte) ([]byte, error) {
	var req pluginapi.ManagementRequest
	if len(raw) > 0 {
		if errUnmarshal := json.Unmarshal(raw, &req); errUnmarshal != nil {
			return nil, errUnmarshal
		}
	}

	pluginMu.RLock()
	mgmt := globalMgmtHandler
	pluginMu.RUnlock()

	resp, errHandle := mgmt.Handle(req)
	if errHandle != nil {
		return errorEnvelope("management_error", errHandle.Error()), nil
	}
	return okEnvelope(resp)
}

func buildPluginRegistration() registration {
	return registration{
		SchemaVersion: pluginabi.SchemaVersion,
		Metadata: pluginapi.Metadata{
			Name:             PluginName,
			Version:          PluginVersion,
			Author:           PluginAuthor,
			GitHubRepository: PluginGitHubRepository,
			Logo:             PluginLogo,
			ConfigFields: []pluginapi.ConfigField{
				{
					Name:        "live_auth_ids",
					Type:        pluginapi.ConfigFieldTypeArray,
					Description: "Allowed OAuth auth IDs for Codex live voice (gpt-live-1-codex).",
				},
				{
					Name:        "strategy",
					Type:        pluginapi.ConfigFieldTypeEnum,
					EnumValues:  []string{StrategyFillFirst, StrategyRoundRobin},
					Description: "Selection strategy across matching live candidates: fill-first or round-robin.",
				},
				{
					Name:        "fail_closed",
					Type:        pluginapi.ConfigFieldTypeBoolean,
					Description: "Reject live requests when no matching candidate is available instead of delegating to built-in scheduler.",
				},
			},
		},
		Capabilities: registrationCapability{
			Scheduler:                 true,
			SchedulerAcrossPriorities: true,
			ManagementAPI:             true,
		},
	}
}

func okEnvelope(v any) ([]byte, error) {
	raw, errMarshal := json.Marshal(v)
	if errMarshal != nil {
		return nil, errMarshal
	}
	return json.Marshal(envelope{OK: true, Result: raw})
}

func errorEnvelope(code, message string) []byte {
	raw, _ := json.Marshal(envelope{OK: false, Error: &envelopeError{Code: code, Message: message}})
	return raw
}

func writeResponse(response *C.cliproxy_buffer, raw []byte) {
	if response == nil || len(raw) == 0 {
		return
	}
	ptr := C.CBytes(raw)
	if ptr == nil {
		return
	}
	response.ptr = ptr
	response.len = C.size_t(len(raw))
}
