package app

import (
	"context"
	"errors"
	"github.com/El1syum/tksu_pl/internal/config"
	"github.com/El1syum/tksu_pl/internal/handlers"
	"github.com/El1syum/tksu_pl/internal/repository"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func Run() error {
	c, err := config.Load()
	if err != nil {
		return err
	}
	store, err := repository.Open(c.DatabasePath)
	if err != nil {
		return err
	}
	defer store.DB.Close()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	h, err := handlers.New(store, c, logger)
	if err != nil {
		return err
	}
	server := &http.Server{Addr: net.JoinHostPort(c.Host, c.Port), Handler: h.Routes(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan error, 1)
	go func() {
		logger.Info("server started", "address", server.Addr, "url", c.PublicURL)
		done <- server.ListenAndServe()
	}()
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case err := <-done:
			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			return err
		case <-ticker.C:
			if err := store.Sessions.Cleanup(ctx); err != nil {
				logger.Error("session cleanup failed", "error", err)
			}
		case <-ctx.Done():
			shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			logger.Info("server shutting down")
			return server.Shutdown(shutdown)
		}
	}
}
