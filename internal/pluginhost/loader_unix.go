//go:build cgo && (linux || darwin || freebsd)

package pluginhost

/*
#cgo linux LDFLAGS: -ldl
#cgo freebsd LDFLAGS: -ldl
#include <dlfcn.h>
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

typedef int (*shinway_plugin_call_fn)(const char*, const uint8_t*, size_t, shinway_buffer*);
typedef void (*shinway_plugin_free_fn)(void*, size_t);
typedef void (*shinway_plugin_shutdown_fn)(void);

typedef struct {
	uint32_t abi_version;
	shinway_plugin_call_fn call;
	shinway_plugin_free_fn free_buffer;
	shinway_plugin_shutdown_fn shutdown;
} shinway_plugin_api;

typedef int (*shinway_plugin_init_fn)(const shinway_host_api*, shinway_plugin_api*);

extern int shinwayHostCall(void*, const char*, const uint8_t*, size_t, shinway_buffer*);
extern void shinwayHostFree(void*, size_t);

static void* shinway_dlopen(const char* path) {
	return dlopen(path, RTLD_NOW | RTLD_LOCAL);
}

static void* shinway_dlsym(void* handle, const char* name) {
	return dlsym(handle, name);
}

static const char* shinway_dlerror(void) {
	return dlerror();
}

static int shinway_dlclose(void* handle) {
	return dlclose(handle);
}

static int shinway_call_init(void* fn, const shinway_host_api* host, shinway_plugin_api* plugin) {
	return ((shinway_plugin_init_fn)fn)(host, plugin);
}

static int shinway_call_plugin(shinway_plugin_call_fn fn, const char* method, const uint8_t* request, size_t request_len, shinway_buffer* response) {
	return fn(method, request, request_len, response);
}

static void shinway_free_plugin_buffer(shinway_plugin_free_fn fn, void* ptr, size_t len) {
	fn(ptr, len);
}

static void shinway_shutdown_plugin(shinway_plugin_shutdown_fn fn) {
	fn();
}

static void shinway_set_host_api(shinway_host_api* api, uint32_t abi_version, void* host_ctx) {
	api->abi_version = abi_version;
	api->host_ctx = host_ctx;
	api->call = shinwayHostCall;
	api->free_buffer = shinwayHostFree;
}

*/
import "C"

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"unsafe"
)

var (
	hostCallbackID      atomic.Uintptr
	hostCallbackEntries sync.Map
)

type dynamicLibraryLoader struct{}

type dynamicLibraryClient struct {
	handle  unsafe.Pointer
	hostAPI *C.shinway_host_api
	hostCtx unsafe.Pointer
	api     C.shinway_plugin_api
}

func defaultPluginLoader() pluginLoader {
	return dynamicLibraryLoader{}
}

func (dynamicLibraryLoader) Open(file pluginFile, host *Host) (pluginClient, error) {
	cPath := C.CString(file.Path)
	defer C.free(unsafe.Pointer(cPath))

	handle := C.shinway_dlopen(cPath)
	if handle == nil {
		return nil, fmt.Errorf("dlopen %s: %s", file.Path, dlerrorString())
	}

	cSymbol := C.CString("shinway_plugin_init")
	initSymbol := C.shinway_dlsym(handle, cSymbol)
	C.free(unsafe.Pointer(cSymbol))
	if initSymbol == nil {
		C.shinway_dlclose(handle)
		return nil, fmt.Errorf("missing shinway_plugin_init: %s", dlerrorString())
	}

	hostAPI := (*C.shinway_host_api)(C.malloc(C.size_t(unsafe.Sizeof(C.shinway_host_api{}))))
	if hostAPI == nil {
		C.shinway_dlclose(handle)
		return nil, fmt.Errorf("allocate host api")
	}
	hostCtx := C.malloc(C.size_t(unsafe.Sizeof(C.uintptr_t(0))))
	if hostCtx == nil {
		C.free(unsafe.Pointer(hostAPI))
		C.shinway_dlclose(handle)
		return nil, fmt.Errorf("allocate host context")
	}
	id := hostCallbackID.Add(1)
	*(*C.uintptr_t)(hostCtx) = C.uintptr_t(id)
	hostCallbackEntries.Store(id, dynamicHostCallbackEntry{host: host, pluginID: file.ID})
	C.shinway_set_host_api(hostAPI, C.uint32_t(pluginHostABIVersion), hostCtx)

	client := &dynamicLibraryClient{
		handle:  handle,
		hostAPI: hostAPI,
		hostCtx: hostCtx,
	}
	rc := C.shinway_call_init(initSymbol, hostAPI, &client.api)
	if rc != 0 {
		client.Shutdown()
		return nil, fmt.Errorf("shinway_plugin_init returned %d", int(rc))
	}
	if uint32(client.api.abi_version) != pluginHostABIVersion {
		client.Shutdown()
		return nil, fmt.Errorf("plugin ABI version %d is not supported", uint32(client.api.abi_version))
	}
	if client.api.call == nil || client.api.free_buffer == nil {
		client.Shutdown()
		return nil, fmt.Errorf("plugin function table is incomplete")
	}
	return client, nil
}

func (c *dynamicLibraryClient) Call(ctx context.Context, method string, request []byte) ([]byte, error) {
	if c == nil || c.api.call == nil {
		return nil, fmt.Errorf("plugin client is closed")
	}
	if ctx != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
	}

	cMethod := C.CString(method)
	defer C.free(unsafe.Pointer(cMethod))
	var cRequest unsafe.Pointer
	if len(request) > 0 {
		cRequest = C.CBytes(request)
		defer C.free(cRequest)
	}
	var response C.shinway_buffer
	rc := C.shinway_call_plugin(c.api.call, cMethod, (*C.uint8_t)(cRequest), C.size_t(len(request)), &response)
	var out []byte
	if response.ptr != nil && response.len > 0 {
		out = C.GoBytes(response.ptr, C.int(response.len))
	}
	if response.ptr != nil {
		C.shinway_free_plugin_buffer(c.api.free_buffer, response.ptr, response.len)
	}
	if rc != 0 {
		if isPluginErrorEnvelope(out) {
			return out, nil
		}
		return nil, fmt.Errorf("plugin call %s returned %d: %s", method, int(rc), string(out))
	}
	return out, nil
}

func (c *dynamicLibraryClient) Shutdown() {
	if c == nil {
		return
	}
	if c.api.shutdown != nil {
		C.shinway_shutdown_plugin(c.api.shutdown)
		c.api.shutdown = nil
	}
	if c.hostCtx != nil {
		id := uintptr(*(*C.uintptr_t)(c.hostCtx))
		hostCallbackEntries.Delete(id)
		C.free(c.hostCtx)
		c.hostCtx = nil
	}
	if c.hostAPI != nil {
		C.free(unsafe.Pointer(c.hostAPI))
		c.hostAPI = nil
	}
	if c.handle != nil {
		C.shinway_dlclose(c.handle)
		c.handle = nil
	}
}

func dlerrorString() string {
	errText := C.shinway_dlerror()
	if errText == nil {
		return ""
	}
	return C.GoString(errText)
}
