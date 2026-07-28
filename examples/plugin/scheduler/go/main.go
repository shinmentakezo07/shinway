package main

/*
#include <stdint.h>
#include <stdlib.h>

typedef struct {
	void* ptr;
	size_t len;
} shinway_buffer;

typedef struct {
	uint32_t abi_version;
	void* host_ctx;
	void* call;
	void* free_buffer;
} shinway_host_api;

typedef int (*shinway_plugin_call_fn)(char*, uint8_t*, size_t, shinway_buffer*);
typedef void (*shinway_plugin_free_fn)(void*, size_t);
typedef void (*shinway_plugin_shutdown_fn)(void);

typedef struct {
	uint32_t abi_version;
	shinway_plugin_call_fn call;
	shinway_plugin_free_fn free_buffer;
	shinway_plugin_shutdown_fn shutdown;
} shinway_plugin_api;

extern int shinwayPluginCall(char*, uint8_t*, size_t, shinway_buffer*);
extern void shinwayPluginFree(void*, size_t);
extern void shinwayPluginShutdown(void);
*/
import "C"

import (
	"encoding/json"
	"strings"
	"sync/atomic"
	"unsafe"

	"github.com/shinmentakezo07/shinway/v7/sdk/pluginabi"
	"github.com/shinmentakezo07/shinway/v7/sdk/pluginapi"
	"gopkg.in/yaml.v3"
)

var currentConfig atomic.Value

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

type pluginConfig struct {
	AuthID   string `yaml:"auth_id"`
	Delegate string `yaml:"delegate"`
	Deny     bool   `yaml:"deny"`
}

type registration struct {
	SchemaVersion uint32                 `json:"schema_version"`
	Metadata      pluginapi.Metadata     `json:"metadata"`
	Capabilities  registrationCapability `json:"capabilities"`
}

type registrationCapability struct {
	Scheduler bool `json:"scheduler"`
}

func main() {}

//export shinway_plugin_init
func shinway_plugin_init(_ *C.shinway_host_api, plugin *C.shinway_plugin_api) C.int {
	if plugin == nil {
		return 1
	}
	plugin.abi_version = C.uint32_t(pluginabi.ABIVersion)
	plugin.call = C.shinway_plugin_call_fn(C.shinwayPluginCall)
	plugin.free_buffer = C.shinway_plugin_free_fn(C.shinwayPluginFree)
	plugin.shutdown = C.shinway_plugin_shutdown_fn(C.shinwayPluginShutdown)
	return 0
}

//export shinwayPluginCall
func shinwayPluginCall(method *C.char, request *C.uint8_t, requestLen C.size_t, response *C.shinway_buffer) C.int {
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
	raw, errHandle := handleMethod(C.GoString(method), requestBytes)
	if errHandle != nil {
		writeResponse(response, errorEnvelope("plugin_error", errHandle.Error()))
		return 1
	}
	writeResponse(response, raw)
	return 0
}

//export shinwayPluginFree
func shinwayPluginFree(ptr unsafe.Pointer, len C.size_t) {
	if ptr != nil {
		C.free(ptr)
	}
}

//export shinwayPluginShutdown
func shinwayPluginShutdown() {}

func handleMethod(method string, request []byte) ([]byte, error) {
	switch method {
	case pluginabi.MethodPluginRegister, pluginabi.MethodPluginReconfigure:
		if errConfigure := configure(request); errConfigure != nil {
			return nil, errConfigure
		}
		return okEnvelope(pluginRegistration())
	case pluginabi.MethodSchedulerPick:
		return pickAuth(request)
	default:
		return errorEnvelope("unknown_method", "unknown method: "+method), nil
	}
}

func configure(raw []byte) error {
	var req lifecycleRequest
	if len(raw) > 0 {
		if errUnmarshal := json.Unmarshal(raw, &req); errUnmarshal != nil {
			return errUnmarshal
		}
	}

	cfg := pluginConfig{}
	if len(req.ConfigYAML) > 0 {
		decoded, errDecode := decodeConfig(req.ConfigYAML)
		if errDecode != nil {
			return errDecode
		}
		cfg = decoded
	}
	cfg.AuthID = strings.TrimSpace(cfg.AuthID)
	cfg.Delegate = strings.TrimSpace(cfg.Delegate)
	currentConfig.Store(cfg)
	return nil
}

func decodeConfig(raw []byte) (pluginConfig, error) {
	var cfg pluginConfig
	if errUnmarshal := yaml.Unmarshal(raw, &cfg); errUnmarshal != nil {
		return pluginConfig{}, errUnmarshal
	}
	return cfg, nil
}

func pluginRegistration() registration {
	return registration{
		SchemaVersion: pluginabi.SchemaVersion,
		Metadata: pluginapi.Metadata{
			Name:             "scheduler",
			Version:          "0.1.0",
			Author:           "shinmentakezo07",
			GitHubRepository: "https://github.com/shinmentakezo07/shinway",
			Logo:             "https://raw.githubusercontent.com/shinmentakezo07/shinway/main/docs/logo.png",
			ConfigFields: []pluginapi.ConfigField{
				{
					Name:        "auth_id",
					Type:        pluginapi.ConfigFieldTypeString,
					Description: "Selects this auth ID when it is present in the scheduler candidates.",
				},
				{
					Name:        "delegate",
					Type:        pluginapi.ConfigFieldTypeEnum,
					EnumValues:  []string{"", pluginapi.SchedulerBuiltinFillFirst, pluginapi.SchedulerBuiltinRoundRobin},
					Description: "Delegates selection to a built-in scheduler when set to fill-first or round-robin.",
				},
				{
					Name:        "deny",
					Type:        pluginapi.ConfigFieldTypeBoolean,
					Description: "Rejects scheduler picks with an explicit error when enabled.",
				},
			},
		},
		Capabilities: registrationCapability{
			Scheduler: true,
		},
	}
}

func pickAuth(raw []byte) ([]byte, error) {
	var req pluginapi.SchedulerPickRequest
	if errUnmarshal := json.Unmarshal(raw, &req); errUnmarshal != nil {
		return nil, errUnmarshal
	}

	cfg := loadedConfig()
	if cfg.Deny {
		return errorEnvelope("scheduler_denied", "scheduler pick denied by plugin configuration"), nil
	}
	switch cfg.Delegate {
	case pluginapi.SchedulerBuiltinFillFirst, pluginapi.SchedulerBuiltinRoundRobin:
		return okEnvelope(pluginapi.SchedulerPickResponse{
			DelegateBuiltin: cfg.Delegate,
			Handled:         true,
		})
	case "":
	default:
		return okEnvelope(pluginapi.SchedulerPickResponse{Handled: false})
	}
	if cfg.AuthID == "" {
		return okEnvelope(pluginapi.SchedulerPickResponse{Handled: false})
	}
	for _, candidate := range req.Candidates {
		if candidate.ID == cfg.AuthID {
			return okEnvelope(pluginapi.SchedulerPickResponse{
				AuthID:  cfg.AuthID,
				Handled: true,
			})
		}
	}
	return okEnvelope(pluginapi.SchedulerPickResponse{Handled: false})
}

func loadedConfig() pluginConfig {
	raw := currentConfig.Load()
	if cfg, ok := raw.(pluginConfig); ok {
		return cfg
	}
	return pluginConfig{}
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

func writeResponse(response *C.shinway_buffer, raw []byte) {
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
