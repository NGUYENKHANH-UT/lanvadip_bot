package main

import (
	"fmt"
	"lanvadip-bot/internal/handler"
	"net/http"
	"time"

	"lanvadip-bot/docs"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger/v2"
	"go.uber.org/zap"
)

type application struct {
	config config
	logger *zap.SugaredLogger
	server *http.Server
}

type config struct {
	addr     string
	env      string
	version  string
	dbPath   string
	redisURL string
}

func (app *application) mount() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	h := handler.NewHandler(app.config.version, app.config.env, app.logger)

	r.Route("/v1", func(r chi.Router) {
		r.Route("/health", h.Health.HealthRoute)

		docsURL := fmt.Sprintf("%s/swagger/doc.json", app.config.addr)
		r.Get("/swagger/*", httpSwagger.Handler(httpSwagger.URL(docsURL)))
	})

	return r
}

func (app *application) run(mux http.Handler) error {
	docs.SwaggerInfo.Version = app.config.version
	docs.SwaggerInfo.BasePath = "/v1"

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
