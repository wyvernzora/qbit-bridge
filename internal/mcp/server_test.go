package mcp

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	qbt "github.com/autobrr/go-qbittorrent"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// startTestSession spins up an in-memory MCP server backed by an autobrr
// qBittorrent client pointed at an unreachable host (no tools call it yet)
// and returns a connected client session.
func startTestSession(t *testing.T) (*mcpsdk.ClientSession, func()) { //nolint:gocritic
	t.Helper()
	client := qbt.NewClient(qbt.Config{
		Host:    "http://127.0.0.1:1", // unreachable; no tools call it yet
		Timeout: 1,                    // seconds; autobrr-config field is int
	})
	server := New(client, discardLogger(), "test")

	t1, t2 := mcpsdk.NewInMemoryTransports()
	ctx, cancel := context.WithCancel(context.Background())
	if _, err := server.Connect(ctx, t1, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test-client", Version: "0.0.0"}, nil).Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	cleanup := func() {
		_ = cs.Close()
		cancel()
	}
	return cs, cleanup
}

func TestListTools_EmptyScaffold(t *testing.T) {
	cs, cleanup := startTestSession(t)
	defer cleanup()
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(res.Tools) != 0 {
		names := make([]string, 0, len(res.Tools))
		for _, tool := range res.Tools {
			names = append(names, tool.Name)
		}
		t.Errorf("expected zero tools in bootstrap scaffold, got %v", names)
	}
}

func TestHTTPTransport_Healthz(t *testing.T) {
	client := qbt.NewClient(qbt.Config{Host: "http://127.0.0.1:1", Timeout: 1})
	server := New(client, discardLogger(), "test")

	mux := http.NewServeMux()
	mcpHandler := mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server { return server }, nil)
	mux.Handle("/mcp", mcpHandler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	hs := httptest.NewServer(mux)
	defer hs.Close()

	resp, err := http.Get(hs.URL + "/healthz")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status = %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	if string(b) != "ok" {
		t.Errorf("body = %q", string(b))
	}
}
