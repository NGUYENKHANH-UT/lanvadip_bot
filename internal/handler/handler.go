package handler

import (
	"lanvadip-bot/internal/service"

	"github.com/payOSHQ/payos-lib-golang/v2"
	"go.uber.org/zap"
)

type Handler struct {
	Health  HealthHandler
	Webhook *WebhookHandler
}

func NewHandler(version string, env string, logger *zap.SugaredLogger, payosClient *payos.PayOS, paymentWorker service.PaymentWorker) *Handler {
	return &Handler{
		Health:  NewHealthHandler(version, env, logger),
		Webhook: NewWebhookHandler(logger, payosClient, paymentWorker),
	}
}
