package main

/*
#include <stdint.h>
#include <stdlib.h>

typedef struct {
	void* ptr;
	size_t len;
} shinway_buffer;

typedef int (*shinway_host_call_fn)(void*, const char*, const uint8_t*, size_t, shinway_buffer*);
typedef void (*shinway_host_free_fn)(void*, size_t);

typedef struct {
	uint32_t abi_version;
	void* host_ctx;
	shinway_host_call_fn call;
	shinway_host_free_fn free_buffer;
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

static const shinway_host_api* stored_host;

static void store_host_api(const shinway_host_api* host) {
	stored_host = host;
}

static int call_host_api(const char* method, const uint8_t* request, size_t request_len, shinway_buffer* response) {
	if (stored_host == NULL || stored_host->call == NULL) {
		return 1;
	}
	return stored_host->call(stored_host->host_ctx, method, request, request_len, response);
}

static void free_host_buffer(void* ptr, size_t len) {
	if (stored_host != NULL && stored_host->free_buffer != NULL && ptr != NULL) {
		stored_host->free_buffer(ptr, len);
	}
}
*/
import "C"

import (
	"encoding/json"
	"net/http"
	"time"
	"unsafe"
)

const abiVersion uint32 = 1

type envelope struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *envelopeError  `json:"error,omitempty"`
}

type envelopeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func main() {}

//export shinway_plugin_init
func shinway_plugin_init(host *C.shinway_host_api, plugin *C.shinway_plugin_api) C.int {
	if plugin == nil {
		return 1
	}
	C.store_host_api(host)
	plugin.abi_version = C.uint32_t(abiVersion)
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
	raw, errHandle := handleMethod(C.GoString(method))
	if errHandle != nil {
		writeResponse(response, errorEnvelope("plugin_error", errHandle.Error()))
		return 1
	}
	writeResponse(response, raw)
	_ = request
	_ = requestLen
	return 0
}

//export shinwayPluginFree
func shinwayPluginFree(ptr unsafe.Pointer, len C.size_t) {
	if ptr != nil {
		C.free(ptr)
	}
	_ = len
}

//export shinwayPluginShutdown
func shinwayPluginShutdown() {}

func handleMethod(method string) ([]byte, error) {
	_ = http.StatusOK
	_ = time.Second
	switch method {
	case "plugin.register":
		return okEnvelopeJSON("{\"schema_version\":1,\"metadata\":{\"Name\":\"example-cli-go\",\"Version\":\"0.1.0\",\"Author\":\"shinmentakezo07\",\"GitHubRepository\":\"https://github.com/shinmentakezo07/shinway\",\"Logo\":\"https://example.invalid/example-cli-go.png\",\"ConfigFields\":[]},\"capabilities\":{\"command_line_plugin\":true}}")
	case "plugin.reconfigure":
		return okEnvelopeJSON("{\"schema_version\":1,\"metadata\":{\"Name\":\"example-cli-go\",\"Version\":\"0.1.0\",\"Author\":\"shinmentakezo07\",\"GitHubRepository\":\"https://github.com/shinmentakezo07/shinway\",\"Logo\":\"https://example.invalid/example-cli-go.png\",\"ConfigFields\":[]},\"capabilities\":{\"command_line_plugin\":true}}")
	case "command_line.register":
		return okEnvelopeJSON("{\"Flags\":[{\"Name\":\"example-cli-go-command\",\"Usage\":\"Run the example plugin command\",\"Type\":\"bool\"}]}")
	case "command_line.execute":
		return okEnvelopeJSON("{\"Stdout\":\"ImV4YW1wbGUtY2xpLWdvIGNvbW1hbmQgZXhlY3V0ZWRcXG4i\",\"ExitCode\":0}")
	default:
		return errorEnvelope("unknown_method", "unknown method: "+method), nil
	}
}

func okEnvelopeJSON(result string) ([]byte, error) {
	return json.Marshal(envelope{OK: true, Result: json.RawMessage(result)})
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

func callHost(method string, payload []byte) {
	cMethod := C.CString(method)
	defer C.free(unsafe.Pointer(cMethod))
	var response C.shinway_buffer
	var req *C.uint8_t
	if len(payload) > 0 {
		req = (*C.uint8_t)(C.CBytes(payload))
		defer C.free(unsafe.Pointer(req))
	}
	if C.call_host_api(cMethod, req, C.size_t(len(payload)), &response) == 0 && response.ptr != nil {
		C.free_host_buffer(response.ptr, response.len)
	}
}
