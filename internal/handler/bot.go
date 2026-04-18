package handler

import (
	"context"
	"lanvadip-bot/internal/service"
	"lanvadip-bot/internal/store"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"go.uber.org/zap"
)

type BotHandler struct {
	logger     *zap.SugaredLogger
	fsmService service.FSMService
}

func NewBotHandler(logger *zap.SugaredLogger, fsmService service.FSMService) *BotHandler {
	return &BotHandler{
		logger:     logger,
		fsmService: fsmService,
	}
}

func (h *BotHandler) HandleMessage(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	userID := update.Message.From.ID

	currentState, err := h.fsmService.GetState(ctx, userID)
	if err != nil {
		h.logger.Errorw("Get state error", "error", err, "userID", userID)
		return
	}

	h.logger.Infow("New message", "userID", userID, "state", currentState)

	switch currentState {
	case store.StateStart:
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Chào cục dàng, quán trà sữa Lan và Địp xin nghe! Cục dàng muốn xem menu chứ?",
		})
		h.fsmService.SetState(ctx, userID, store.StateOrdering)

	case store.StateOrdering:
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Cục dàng hôm nay muốn uống gì nè!",
		})
	}
}
