#include <stdint.h>
#include <stdlib.h>
#include <string.h>

#if defined(_WIN32)
#define SHINWAY_EXPORT __declspec(dllexport)
#else
#define SHINWAY_EXPORT __attribute__((visibility("default")))
#endif

#define ABI_VERSION 1

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

static const shinway_host_api* stored_host = NULL;

static void write_response(shinway_buffer* response, const char* text) {
	if (response == NULL || text == NULL) {
		return;
	}
	size_t len = strlen(text);
	void* ptr = malloc(len);
	if (ptr == NULL) {
		response->ptr = NULL;
		response->len = 0;
		return;
	}
	memcpy(ptr, text, len);
	response->ptr = ptr;
	response->len = len;
}

static void call_host(const char* method, const char* payload) {
	if (stored_host == NULL || stored_host->call == NULL || method == NULL) {
		return;
	}
	shinway_buffer response = {0};
	const uint8_t* request = (const uint8_t*)payload;
	size_t request_len = payload == NULL ? 0 : strlen(payload);
	if (stored_host->call(stored_host->host_ctx, method, request, request_len, &response) == 0 && response.ptr != NULL && stored_host->free_buffer != NULL) {
		stored_host->free_buffer(response.ptr, response.len);
	}
}

static int plugin_call(const char* method, const uint8_t* request, size_t request_len, shinway_buffer* response) {
	if (response != NULL) {
		response->ptr = NULL;
		response->len = 0;
	}
	if (method == NULL) {
		write_response(response, "{\"ok\":false,\"error\":{\"code\":\"invalid_method\",\"message\":\"method is required\"}}");
		return 1;
	}
	if (strcmp(method, "plugin.register") == 0) {
		write_response(response, "{\"ok\":true,\"result\":{\"schema_version\":1,\"metadata\":{\"Name\":\"example-response-normalizer-c\",\"Version\":\"0.1.0\",\"Author\":\"shinmentakezo07\",\"GitHubRepository\":\"https://github.com/shinmentakezo07/shinway\",\"Logo\":\"https://example.invalid/example-response-normalizer-c.png\",\"ConfigFields\":[]},\"capabilities\":{\"response_before_translator\":true,\"response_after_translator\":true}}}");
		return 0;
	}
	if (strcmp(method, "plugin.reconfigure") == 0) {
		write_response(response, "{\"ok\":true,\"result\":{\"schema_version\":1,\"metadata\":{\"Name\":\"example-response-normalizer-c\",\"Version\":\"0.1.0\",\"Author\":\"shinmentakezo07\",\"GitHubRepository\":\"https://github.com/shinmentakezo07/shinway\",\"Logo\":\"https://example.invalid/example-response-normalizer-c.png\",\"ConfigFields\":[]},\"capabilities\":{\"response_before_translator\":true,\"response_after_translator\":true}}}");
		return 0;
	}
	if (strcmp(method, "response.normalize_before") == 0) {
		write_response(response, "{\"ok\":true,\"result\":{\"Body\":\"eyJyZXNwb25zZV9ub3JtYWxpemVkX2JlZm9yZV9ieSI6ImV4YW1wbGUtcmVzcG9uc2Utbm9ybWFsaXplci1jIn0=\"}}");
		return 0;
	}
	if (strcmp(method, "response.normalize_after") == 0) {
		write_response(response, "{\"ok\":true,\"result\":{\"Body\":\"eyJyZXNwb25zZV9ub3JtYWxpemVkX2FmdGVyX2J5IjoiZXhhbXBsZS1yZXNwb25zZS1ub3JtYWxpemVyLWMifQ==\"}}");
		return 0;
	}
	write_response(response, "{\"ok\":false,\"error\":{\"code\":\"unknown_method\",\"message\":\"unknown method\"}}");
	(void)request;
	(void)request_len;
	return 0;
}

static void plugin_free(void* ptr, size_t len) {
	(void)len;
	free(ptr);
}

static void plugin_shutdown(void) {}

SHINWAY_EXPORT int shinway_plugin_init(const shinway_host_api* host, shinway_plugin_api* plugin) {
	if (plugin == NULL) {
		return 1;
	}
	stored_host = host;
	plugin->abi_version = ABI_VERSION;
	plugin->call = plugin_call;
	plugin->free_buffer = plugin_free;
	plugin->shutdown = plugin_shutdown;
	return 0;
}
