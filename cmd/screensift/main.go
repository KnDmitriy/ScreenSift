package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"screensift/internal/api"
	"screensift/internal/config"
	"screensift/internal/indexer"
	"screensift/internal/ocr"
	"screensift/internal/store"
)

func main() {
	cfg := config.FromFlags()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		logger.Error("open database", "error", err)
		os.Exit(1)
	}
	defer st.Close()

	recognizer := ocr.NewTesseract(cfg.TesseractPath, cfg.OCRLanguages, cfg.OCRTimeout)
	ix := indexer.New(cfg, st, recognizer)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := ix.Start(ctx); err != nil {
		logger.Error("start indexer", "error", err)
		os.Exit(1)
	}

	server := api.NewServer(cfg.HTTPAddr, st, ix)
	go func() {
		if err := server.Start(); err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("http server stopped", "error", err)
			cancel()
		}
	}()

	<-ctx.Done()
	logger.Info("shutdown requested")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	_ = server.Shutdown(shutdownCtx)
	ix.Stop()

	logger.Info("shutdown complete")
}
