package mcp

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	qbt "github.com/autobrr/go-qbittorrent"

	"github.com/wyvernzora/qbittorrent-mcp/internal/savepath"
)

type capturedReq struct {
	Method   string
	Path     string
	Query    url.Values
	PostForm url.Values
}

type mockRoute struct {
	status int
	body   string
}

func newQbitMockStatus(t *testing.T, status int) *qbt.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return qbt.NewClient(qbt.Config{Host: srv.URL, Timeout: 2, RetryAttempts: 1, RetryDelay: 1})
}

func newQbitMockRoutes(t *testing.T, routes map[string]mockRoute) (client *qbt.Client, captured map[string]*capturedReq) {
	t.Helper()
	captured = make(map[string]*capturedReq, len(routes))
	for k := range routes {
		captured[k] = &capturedReq{}
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route, ok := routes[r.URL.Path]
		if !ok {
			t.Errorf("unrouted request to %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		cap := captured[r.URL.Path]
		cap.Method = r.Method
		cap.Path = r.URL.Path
		cap.Query = r.URL.Query()
		if r.Method == http.MethodPost {
			_ = r.ParseForm()
			cap.PostForm = r.PostForm
		}
		if route.status != 0 && route.status != http.StatusOK {
			w.WriteHeader(route.status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(route.body))
	}))
	t.Cleanup(srv.Close)
	return qbt.NewClient(qbt.Config{Host: srv.URL, Timeout: 2, RetryAttempts: 1, RetryDelay: 1}), captured
}

func mustResolver(t *testing.T, spec string) *savepath.Resolver {
	t.Helper()
	r, err := savepath.Parse(spec)
	if err != nil {
		t.Fatalf("savepath.Parse(%q): %v", spec, err)
	}
	return r
}
