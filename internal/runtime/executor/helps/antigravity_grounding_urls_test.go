package helps

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/shinmentakezo07/shinway/v7/internal/config"
	"github.com/tidwall/gjson"
)

type groundingURLRoundTripper func(*http.Request) (*http.Response, error)

func (f groundingURLRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestResolveAntigravityGroundingURLsResolvesVertexRedirects(t *testing.T) {
	t.Parallel()

	const redirectURL = "https://vertexaisearch.cloud.google.com/grounding-api-redirect/example-token"
	const resolvedURL = "https://example.com/weather"

	var sawRedirectRequest bool
	ctx := context.WithValue(context.Background(), "shinway.roundtripper", groundingURLRoundTripper(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodHead {
			t.Fatalf("method = %s, want HEAD", req.Method)
		}
		if req.URL.String() != redirectURL {
			t.Fatalf("url = %s, want %s", req.URL.String(), redirectURL)
		}
		sawRedirectRequest = true
		return &http.Response{
			StatusCode: http.StatusFound,
			Header: http.Header{
				"Location": []string{resolvedURL},
			},
			Body: io.NopCloser(strings.NewReader("")),
		}, nil
	}))

	input := []byte(`{
		"response": {
			"candidates": [{
				"groundingMetadata": {
					"groundingChunks": [
						{"web": {"uri": "` + redirectURL + `", "title": "Weather"}},
						{"web": {"uri": "https://already.example/source", "title": "Existing"}}
					]
				}
			}]
		}
	}`)

	output := ResolveAntigravityGroundingURLs(ctx, nil, nil, input)
	if !sawRedirectRequest {
		t.Fatal("expected resolver to request the vertex redirect")
	}
	if got := gjson.GetBytes(output, "response.candidates.0.groundingMetadata.groundingChunks.0.web.uri").String(); got != resolvedURL {
		t.Fatalf("resolved uri = %q, want %q; output=%s", got, resolvedURL, output)
	}
	if got := gjson.GetBytes(output, "response.candidates.0.groundingMetadata.groundingChunks.1.web.uri").String(); got != "https://already.example/source" {
		t.Fatalf("non-vertex uri = %q", got)
	}
}

func TestResolveAntigravityGroundingURLs_CachesResolvedURIs(t *testing.T) {
	payload := []byte(`{"response":{"candidates":[{"groundingMetadata":{"groundingChunks":[
		{"web":{"uri":"https://vertexaisearch.cloud.google.com/grounding-api-redirect/abc"}},
		{"web":{"uri":"https://vertexaisearch.cloud.google.com/grounding-api-redirect/abc"}},
		{"web":{"uri":"https://plain.example.com/result"}}
	]}}]}}`)
	var headCount atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headCount.Add(1)
		w.Header().Set("Location", "https://resolved.example.com/abc")
		w.WriteHeader(http.StatusFound)
	}))
	defer ts.Close()

	tsURL, errParse := url.Parse(ts.URL)
	if errParse != nil {
		t.Fatalf("parse test server url: %v", errParse)
	}

	// isAntigravityVertexSearchRedirect matches the vertex host exactly, so the
	// payload keeps the real host and the roundtripper forwards the HEAD to the
	// test server instead of rewriting the host in the payload.
	ctx := context.WithValue(context.Background(), "shinway.roundtripper", groundingURLRoundTripper(func(req *http.Request) (*http.Response, error) {
		req.URL.Scheme = tsURL.Scheme
		req.URL.Host = tsURL.Host
		return ts.Client().Transport.RoundTrip(req)
	}))

	cfg := &config.Config{}
	// The same redirect URI appears twice within this payload and again in a
	// second call; the package-level 60s cache must collapse all occurrences
	// (within and across calls) into a single HEAD.
	out := ResolveAntigravityGroundingURLs(ctx, cfg, nil, payload)
	if got := gjson.GetBytes(out, "response.candidates.0.groundingMetadata.groundingChunks.1.web.uri").String(); got != "https://resolved.example.com/abc" {
		t.Fatalf("resolved uri = %q, want https://resolved.example.com/abc", got)
	}
	out = ResolveAntigravityGroundingURLs(ctx, cfg, nil, payload)
	if got := gjson.GetBytes(out, "response.candidates.0.groundingMetadata.groundingChunks.1.web.uri").String(); got != "https://resolved.example.com/abc" {
		t.Fatalf("resolved uri = %q, want https://resolved.example.com/abc", got)
	}
	if headCount.Load() != 1 {
		t.Fatalf("HEAD requests = %d, want 1 (deduped)", headCount.Load())
	}
}
