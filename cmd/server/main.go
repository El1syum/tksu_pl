package main

import (
	"github.com/El1syum/tksu_pl/internal/app"
	"log/slog"
	"os"
)

func main() {
	if err := app.Run(); err != nil {
		slog.Error("application stopped", "error", err)
		os.Exit(1)
	}
}
