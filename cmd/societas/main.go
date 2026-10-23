package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Jeanedlune/Societas-AI/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	args := os.Args[1:]
	if os.Getenv("SOCIETAS_PROCESS_SMOKE") == "1" {
		args = []string{"serve", "--config", os.Getenv("SOCIETAS_SMOKE_CONFIG"), "--db", os.Getenv("SOCIETAS_SMOKE_DB")}
	} else if len(args) == 0 {
		args = []string{"serve"}
	}
	if err := cli.Execute(ctx, args, os.Stdout, os.Stderr); err != nil {
		slog.Error("societas command failed", "error", err)
		os.Exit(1)
	}
}
