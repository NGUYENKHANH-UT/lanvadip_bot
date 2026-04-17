package handler

import (
	"go.uber.org/zap"
)

type Handler struct {
	Health HealthHandler
}

func NewHandler(version string, env string, logger *zap.SugaredLogger) *Handler {
	return &Handler{
		Health: NewHealthHandler(version, env, logger),
	}
}
