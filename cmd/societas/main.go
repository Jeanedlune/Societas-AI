package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Jeanedlune/Societas-AI/internal/server"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	addr := os.Getenv("SOCIETAS_ADDR")
	if addr == "" {
		addr = "127.0.0.1:7443"
	}

	if err := server.Run(ctx, addr, os.Stdout); err != nil {
		slog.Error("server stopped with an error", "error", err)
		os.Exit(1)
	}
}
