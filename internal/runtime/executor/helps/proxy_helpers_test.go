package helps

import (
	"context"
	"net/http"
	"testing"

	"github.com/shinmentakezo07/shinway/v7/internal/config"
	sdkconfig "github.com/shinmentakezo07/shinway/v7/sdk/config"
	shinwayauth "github.com/shinmentakezo07/shinway/v7/sdk/shinway/auth"
)

func TestNewProxyAwareHTTPClientDirectBypassesGlobalProxy(t *testing.T) {
	t.Parallel()

	client := NewProxyAwareHTTPClient(
		context.Background(),
		&config.Config{SDKConfig: sdkconfig.SDKConfig{ProxyURL: "http://global-proxy.example.com:8080"}},
		&shinwayauth.Auth{ProxyURL: "direct"},
		0,
	)

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T, want *http.Transport", client.Transport)
	}
	if transport.Proxy != nil {
		t.Fatal("expected direct transport to disable proxy function")
	}
}
