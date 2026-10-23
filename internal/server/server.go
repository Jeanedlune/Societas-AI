package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// NewHandler returns the HTTP handlers available during application bootstrap.
func NewHandler(options ...APIOptions) http.Handler {
	var opts APIOptions
	if len(options) > 0 {
		opts = options[0]
	}
	return NewAPIHandler(opts)
}

// Run serves HTTP until ctx is cancelled, then gracefully shuts down.
func Run(ctx context.Context, addr string, ready io.Writer) error {
	return RunWithOptions(ctx, addr, ready, APIOptions{})
}

func RunWithOptions(ctx context.Context, addr string, ready io.Writer, options APIOptions) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}
	if _, err := fmt.Fprintf(ready, "listening on %s\n", listener.Addr()); err != nil {
		_ = listener.Close()
		return fmt.Errorf("write ready message: %w", err)
	}

	httpServer := &http.Server{
		Handler:           NewHandler(options),
		ReadHeaderTimeout: 5 * time.Second,
	}
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- httpServer.Serve(listener)
	}()

	select {
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP: %w", err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			_ = httpServer.Close()
			return fmt.Errorf("shutdown HTTP server: %w", err)
		}
		err := <-serveErr
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", err)
		}
		return nil
	}
}
