package handler

import (
	"lanvadip-bot/internal/platform/errs"
	"lanvadip-bot/internal/platform/transport"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type healthHandler struct {
	version string
	env     string
	logger  *zap.SugaredLogger
}

type HealthHandler interface {
	HealthRoute(chi.Router)
}

func NewHealthHandler(version, env string, logger *zap.SugaredLogger) HealthHandler {
	return &healthHandler{
		version: version,
		env:     env,
		logger:  logger,
	}
}

func (h *healthHandler) HealthRoute(r chi.Router) {
	r.Get("/", h.healthCheck)
}

func (h *healthHandler) healthCheck(w http.ResponseWriter, r *http.Request) {
	data := map[string]string{
		"status":  "OK",
		"env":     h.env,
		"version": h.version,
	}
	if err := transport.JsonResponse(w, http.StatusOK, data); err != nil {
		errs.InternalServerError(w, r, err, h.logger)
	}
}
