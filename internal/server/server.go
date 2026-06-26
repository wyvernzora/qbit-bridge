// Package server owns the HTTP mux that mounts MCP, REST, and health routes.
package server

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	qbt "github.com/autobrr/go-qbittorrent"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/wyvernzora/qbittorrent-mcp/internal/downloads"
	"github.com/wyvernzora/qbittorrent-mcp/internal/rest"
	"github.com/wyvernzora/qbittorrent-mcp/internal/savepath"
)

const (
	mcpEndpoint = "/mcp"
	healthPath  = "/healthz"

	maxRequestBytes = 1 << 20 // 1 MiB

	// httpShutdownGrace must be at least the upstream HTTP timeout so an
	// in-flight tool call has a chance to finish during graceful shutdown.
	httpShutdownGrace = 20 * time.Second
)

// RunHTTP starts the HTTP transport plus /healthz and REST endpoints. Blocks
// until ctx is cancelled, then returns after a graceful shutdown.
func RunHTTP(ctx context.Context, s *mcpsdk.Server, client *qbt.Client, resolver *savepath.Resolver, addr string, logger *slog.Logger) error {
	mux := http.NewServeMux()

	mcpHandler := mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server { return s }, nil)
	mux.Handle(mcpEndpoint, capBodySize(mcpHandler))
	rest.Register(mux, downloads.New(client, resolver, logger))

	mux.HandleFunc(healthPath, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	logger.Info("http transport listening", "addr", ln.Addr().String(), "endpoint", mcpEndpoint)

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), httpShutdownGrace)
		defer cancel()
		return srv.Shutdown(shutdownCtx) //nolint:contextcheck // parent ctx is already cancelled; shutdown needs a fresh deadline
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func capBodySize(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
		h.ServeHTTP(w, r)
	})
}
