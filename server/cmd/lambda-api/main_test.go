package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"
	"github.com/google/uuid"

	"github.com/aaugoustis/waitlist/server/internal/config"
)

type nopEnqueuer struct{}

func (nopEnqueuer) EnqueueClassify(context.Context, uuid.UUID) error { return nil }

// Feeds API Gateway v2 events through the same adapter the Lambda uses. No
// database is reachable, so only routes that don't query it are checked.
func TestAPIGatewayEvents(t *testing.T) {
	cfg := config.Config{
		DatabaseURL:  "postgres://nobody@127.0.0.1:1/none",
		PublicURL:    "https://abc.execute-api.us-east-1.amazonaws.com",
		DevJWTSecret: "test-secret-test-secret-test-secret",
		CORSOrigins:  []string{"https://web.example"},
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h, err := newHandler(context.Background(), cfg, log, nopEnqueuer{})
	if err != nil {
		t.Fatal(err)
	}
	proxy := httpadapter.NewV2(h).ProxyWithContext

	req := func(method, path string, headers map[string]string) events.APIGatewayV2HTTPResponse {
		t.Helper()
		ev := events.APIGatewayV2HTTPRequest{
			Version: "2.0", RouteKey: "$default", RawPath: path, Headers: headers,
			RequestContext: events.APIGatewayV2HTTPRequestContext{
				DomainName: "abc.execute-api.us-east-1.amazonaws.com",
				HTTP:       events.APIGatewayV2HTTPRequestContextHTTPDescription{Method: method, Path: path},
			},
		}
		res, err := proxy(context.Background(), ev)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}

	if res := req(http.MethodGet, "/v1/items", nil); res.StatusCode != http.StatusUnauthorized {
		t.Errorf("no token: status %d, body %s", res.StatusCode, res.Body)
	}
	res := req(http.MethodGet, "/.well-known/oauth-protected-resource/mcp", nil)
	if res.StatusCode != http.StatusOK || !strings.Contains(res.Body, `"resource":"https://abc.execute-api.us-east-1.amazonaws.com/mcp"`) {
		t.Errorf("MCP metadata: status %d, body %s", res.StatusCode, res.Body)
	}
	res = req(http.MethodOptions, "/v1/items", map[string]string{
		"origin": "https://web.example", "access-control-request-method": "POST",
	})
	if res.Headers["Access-Control-Allow-Origin"] != "https://web.example" {
		t.Errorf("CORS preflight: status %d, headers %v", res.StatusCode, res.Headers)
	}
}
