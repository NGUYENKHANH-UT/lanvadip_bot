package main

import (
	"lanvadip-bot/internal/handler"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

type application struct {
	config config
	logger *zap.SugaredLogger
	server *http.Server // Thêm trường này để main.go có thể gọi .Shutdown()
}

type config struct {
	addr    string
	env     string
	version string
}

func (app *application) mount() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	h := handler.NewHandler(app.config.version, app.config.env, app.logger)

	r.Route("/v1", func(r chi.Router) {
		r.Route("/health", h.Health.HealthRoute)
	})

	return r
}

func (app *application) run(mux http.Handler) error {
	// Gán http.Server vào struct thay vì dùng biến cục bộ
	app.server = &http.Server{
		Addr:         app.config.addr,
		Handler:      mux,
		WriteTimeout: time.Second * 30,
		ReadTimeout:  time.Second * 10,
		IdleTimeout:  time.Minute,
	}

	app.logger.Infow("Server has started", "addr", app.config.addr, "env", app.config.env, "version", app.config.version)

	return app.server.ListenAndServe()
}
