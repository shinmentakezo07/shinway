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
	"github.com/tidwall/sjson"
	"gopkg.in/yaml.v3"
)

var fastEnabled atomic.Bool

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
	Fast bool `yaml:"fast"`
}

type registration struct {
	SchemaVersion uint32                 `json:"schema_version"`
	Metadata      pluginapi.Metadata     `json:"metadata"`
	Capabilities  registrationCapability `json:"capabilities"`
}

type registrationCapability struct {
	RequestNormalizer bool `json:"request_normalizer"`
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
	case pluginabi.MethodRequestNormalize:
		return normalizeRequest(request)
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
		fast, errDecodeFast := decodeFastConfig(req.ConfigYAML)
		if errDecodeFast != nil {
			return errDecodeFast
		}
		cfg.Fast = fast
	}
	fastEnabled.Store(cfg.Fast)
	return nil
}

func pluginRegistration() registration {
	return registration{
		SchemaVersion: pluginabi.SchemaVersion,
		Metadata: pluginapi.Metadata{
			Name:             "codex-service-tier",
			Version:          "0.1.0",
			Author:           "shinmentakezo07",
			GitHubRepository: "https://github.com/shinmentakezo07/shinway",
			Logo:             "https://raw.githubusercontent.com/shinmentakezo07/shinway/main/docs/logo.png",
			ConfigFields: []pluginapi.ConfigField{{
				Name:        "fast",
				Type:        pluginapi.ConfigFieldTypeBoolean,
				Description: "Sets Codex gpt-5.5 Responses requests to the priority service tier.",
			}},
		},
		Capabilities: registrationCapability{
			RequestNormalizer: true,
		},
	}
}

func normalizeRequest(raw []byte) ([]byte, error) {
	var req pluginapi.RequestTransformRequest
	if errUnmarshal := json.Unmarshal(raw, &req); errUnmarshal != nil {
		return nil, errUnmarshal
	}
	body := req.Body
	if !shouldSetPriorityServiceTier(req) {
		return okEnvelope(pluginapi.PayloadResponse{Body: body})
	}
	updated, okSet := setPriorityServiceTier(body)
	if !okSet {
		return okEnvelope(pluginapi.PayloadResponse{Body: body})
	}
	return okEnvelope(pluginapi.PayloadResponse{Body: updated})
}

func shouldSetPriorityServiceTier(req pluginapi.RequestTransformRequest) bool {
	if !fastEnabled.Load() {
		return false
	}
	if !strings.EqualFold(req.ToFormat, "codex") {
		return false
	}
	return req.Model == "gpt-5.5"
}

func decodeFastConfig(configYAML []byte) (bool, error) {
	var cfg pluginConfig
	if errUnmarshal := yaml.Unmarshal(configYAML, &cfg); errUnmarshal != nil {
		return false, errUnmarshal
	}
	return cfg.Fast, nil
}

func setPriorityServiceTier(body []byte) ([]byte, bool) {
	updated, errSet := sjson.SetBytes(body, "service_tier", "priority")
	if errSet != nil {
		return nil, false
	}
	return updated, true
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
