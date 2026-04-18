package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"lanvadip-bot/internal/service"

	"github.com/payOSHQ/payos-lib-golang/v2"
	"go.uber.org/zap"
)

type WebhookHandler struct {
	logger        *zap.SugaredLogger
	payosClient   *payos.PayOS
	paymentWorker service.PaymentWorker
}

func NewWebhookHandler(logger *zap.SugaredLogger, payosClient *payos.PayOS, worker service.PaymentWorker) *WebhookHandler {
	return &WebhookHandler{
		logger:        logger,
		payosClient:   payosClient,
		paymentWorker: worker,
	}
}

func (h *WebhookHandler) HandlePayOSWebhook(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Errorw("failed to read webhook body", "error", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var webhookMap map[string]interface{}
	if err := json.Unmarshal(body, &webhookMap); err != nil {
		h.logger.Errorw("failed to unmarshal webhook to map", "error", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	_, err = h.payosClient.Webhooks.VerifyData(r.Context(), webhookMap)
	if err != nil {
		h.logger.Errorw("invalid webhook signature detected", "error", err)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   0,
			"message": "Ok",
			"data":    nil,
		})
		return
	}

	var webhookStruct payos.Webhook
	if err := json.Unmarshal(body, &webhookStruct); err != nil {
		h.logger.Errorw("failed to unmarshal webhook to struct", "error", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if webhookStruct.Data != nil {
		h.paymentWorker.EnqueuePayload(*webhookStruct.Data)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   0,
		"message": "Ok",
		"data":    nil,
	})
}
