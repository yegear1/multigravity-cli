package server

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// apiAliasExceptions are /api/v1 routes that do not have an /api twin.
// SSE stays on /events. The gateway stays on /v1 plus the /api/v1 mirror.
var apiAliasExceptions = map[string]string{
	"GET /api/v1/events":            "GET /events",
	"POST /api/v1/chat/completions": "POST /v1/chat/completions",
	"GET /api/v1/models":            "GET /v1/models",
	"POST /api/v1/messages":         "POST /v1/messages",
	"GET /api/v1/router/status":     "GET /v1/router/status",
	"POST /api/v1/router/reset":     "POST /v1/router/reset",
	"POST /api/v1/router/strategy":  "POST /v1/router/strategy",
}

func TestAPIRoutesHaveAliasTwin(t *testing.T) {
	srv := NewServer(Config{})
	registered := make(map[string]struct{}, len(srv.patterns))
	for _, pattern := range srv.patterns {
		registered[pattern] = struct{}{}
	}

	seen := make(map[string]struct{}, len(apiAliasExceptions))
	for _, pattern := range srv.patterns {
		method, path, ok := strings.Cut(pattern, " ")
		if !ok || !strings.HasPrefix(path, "/api/v1/") && path != "/api/v1" {
			continue
		}
		twinPath := "/api" + strings.TrimPrefix(path, "/api/v1")
		twin := method + " " + twinPath
		if canonical, excepted := apiAliasExceptions[pattern]; excepted {
			seen[pattern] = struct{}{}
			if _, ok := registered[twin]; ok {
				t.Errorf("%s gained an /api twin; drop it from apiAliasExceptions or stop registering it", pattern)
			}
			if _, ok := registered[canonical]; !ok {
				t.Errorf("%s missing canonical %s", pattern, canonical)
			}
			assertMuxPattern(t, srv, pattern)
			assertMuxPattern(t, srv, canonical)
			continue
		}
		if _, ok := registered[twin]; !ok {
			t.Errorf("%s has no /api twin %s", pattern, twin)
			continue
		}
		assertMuxPattern(t, srv, pattern)
		assertMuxPattern(t, srv, twin)
	}

	for pattern := range apiAliasExceptions {
		if _, ok := seen[pattern]; !ok {
			t.Errorf("exception %s is not registered", pattern)
		}
	}
}

func assertMuxPattern(t *testing.T, srv *Server, pattern string) {
	t.Helper()
	method, path, ok := strings.Cut(pattern, " ")
	if !ok {
		t.Fatalf("bad pattern %q", pattern)
	}
	req := httptest.NewRequest(method, fillWildcards(path), nil)
	_, got := srv.mux.Handler(req)
	if got != pattern {
		t.Errorf("mux.Handler(%s %s) = %q, want %q", method, req.URL.Path, got, pattern)
	}
}

func fillWildcards(path string) string {
	var b strings.Builder
	for i := 0; i < len(path); {
		if path[i] != '{' {
			b.WriteByte(path[i])
			i++
			continue
		}
		end := strings.IndexByte(path[i:], '}')
		if end < 0 {
			b.WriteString(path[i:])
			break
		}
		b.WriteString("sample")
		i += end + 1
	}
	return b.String()
}
