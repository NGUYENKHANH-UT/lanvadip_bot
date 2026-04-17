package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"lanvadip-bot/internal/platform/env"

	"go.uber.org/zap"
)

func main() {
	cfg := config{
		addr:    env.GetString("PORT", ":8080"),
		env:     env.GetString("ENV", "development"),
		version: env.GetString("VERSION", "0.0.1"),
	}

	// logger
	logger := zap.Must(zap.NewProduction()).Sugar()
	defer logger.Sync()

	app := &application{
		config: cfg,
		logger: logger,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	token := env.GetString("TELEGRAM_BOT_TOKEN", "")
	if token == "" {
		logger.Fatal("TELEGRAM_BOT_TOKEN is not set")
	}

	b, err := setupBot(token, logger)
	if err != nil {
		logger.Fatalw("Failed to initialize bot", "error", err)
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("Bot successfully started and listening for messages...")
		b.Start(ctx)
		logger.Info("Bot has stopped polling")
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := app.run(app.mount()); err != nil && err != http.ErrServerClosed {
			logger.Errorw("Web server error", "error", err)
		}
	}()

	<-ctx.Done()
	logger.Info("Interrupt signal received. Initiating graceful shutdown...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if app.server != nil {
		if err := app.server.Shutdown(shutdownCtx); err != nil {
			logger.Errorw("Web server forced to shutdown", "error", err)
		} else {
			logger.Info("Web server gracefully stopped")
		}
	}

	wg.Wait()
	logger.Info("All services stopped. Goodbye!")
}
