package service

import (
	"context"
	"fmt"
	"time"

	"lanvadip-bot/internal/model"
	"lanvadip-bot/internal/store"

	"github.com/go-telegram/bot"
	"github.com/payOSHQ/payos-lib-golang/v2"
	"go.uber.org/zap"
)

type PaymentWorker interface {
	Start(ctx context.Context, numWorkers int)
	EnqueuePayload(payload payos.WebhookData)
	SetBot(b *bot.Bot)
}

type paymentWorker struct {
	jobQueue   chan payos.WebhookData
	logger     *zap.SugaredLogger
	fsmService FSMService
	tgBot      *bot.Bot
	orderStore store.OrderStore
}

func NewPaymentWorker(logger *zap.SugaredLogger, fsmService FSMService, orderStore store.OrderStore) PaymentWorker {
	return &paymentWorker{
		jobQueue:   make(chan payos.WebhookData, 100),
		logger:     logger,
		fsmService: fsmService,
		orderStore: orderStore,
	}
}

func (w *paymentWorker) SetBot(b *bot.Bot) {
	w.tgBot = b
}

func (w *paymentWorker) Start(ctx context.Context, numWorkers int) {
	for i := 1; i <= numWorkers; i++ {
		go w.worker(ctx, i)
	}
}

func (w *paymentWorker) EnqueuePayload(payload payos.WebhookData) {
	w.jobQueue <- payload
}

func (w *paymentWorker) worker(ctx context.Context, id int) {
	w.logger.Infof("Payment Worker %d đang túc trực lắng nghe...", id)
	for {
		select {
		case <-ctx.Done():
			return
		case payload := <-w.jobQueue:
			w.processPayment(payload)
		}
	}
}

func (w *paymentWorker) processPayment(payload payos.WebhookData) {
	orderCode := payload.OrderCode
	w.logger.Infow("Đang xử lý webhook thanh toán...", "orderCode", orderCode)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	userID, err := w.fsmService.GetOrderUser(ctx, orderCode)
	if err != nil || userID == 0 {
		w.logger.Errorw("Mất dấu vết UserID cho Order này", "orderCode", orderCode)
		return
	}

	w.fsmService.SetState(ctx, userID, model.StateCompleted)

	err = w.orderStore.UpdateOrderStatus(ctx, orderCode, "PAID_SUCCESS")
	if err != nil {
		w.logger.Errorw("Lỗi cập nhật DB hóa đơn", "orderCode", orderCode, "error", err)
	} else {
		w.logger.Infof("Order %d đã lưu DB trạng thái PAID_SUCCESS!", orderCode)
	}

	w.logger.Infof("Order %d PAID_SUCCESS! (UserID: %d)", orderCode, userID)

	if w.tgBot != nil {
		w.tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    userID,
			Text:      fmt.Sprintf("🎉 DING DING! Quán đã nhận được lúa cho đơn hàng mã <b>#%d</b> rồi nha Cục dàng ơi. Quán đang bắt tay vào làm nước liền đây! 🥤", orderCode),
			ParseMode: "HTML",
		})
	}
}
