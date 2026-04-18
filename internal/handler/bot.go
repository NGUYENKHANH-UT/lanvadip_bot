package handler

import (
	"context"
	"lanvadip-bot/internal/model"
	"lanvadip-bot/internal/service"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"go.uber.org/zap"
)

type botHandler struct {
	logger     *zap.SugaredLogger
	fsmService service.FSMService
	aiService  service.AIService
}

type BotHandler interface {
	HandleMessage(ctx context.Context, b *bot.Bot, update *models.Update)
}

func NewBotHandler(logger *zap.SugaredLogger, fsmService service.FSMService, aiService service.AIService) BotHandler {
	return &botHandler{
		logger:     logger,
		fsmService: fsmService,
		aiService:  aiService,
	}
}

func (h *botHandler) HandleMessage(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	userID := update.Message.From.ID
	text := update.Message.Text

	currentState, err := h.fsmService.GetState(ctx, userID)
	if err != nil {
		h.logger.Errorw("failed to get user state from FSM", "error", err, "userID", userID)
		currentState = model.StateStart
	}

	h.logger.Infow("received new message", "userID", userID, "state", currentState, "text_length", len(text))

	if currentState == model.StateStart {
		err = h.fsmService.SetState(ctx, userID, model.StateOrdering)
		if err != nil {
			h.logger.Errorw("failed to update user state to ORDERING", "error", err, "userID", userID)
		}
	}

	if currentState == model.StateOrdering || currentState == model.StateStart {
		aiResponse, err := h.aiService.AnalyzeAndRespond(ctx, userID, text)

		if err != nil {
			h.logger.Errorw("failed to get AI response", "error", err, "userID", userID)

			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   "Dạ hệ thống quán đang bảo trì chút xíu, cục dàng chờ xíu rồi nhắn lại giúp quán nha! 🧋",
			})
			return
		}

		_, err = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   aiResponse,
		})

		if err != nil {
			h.logger.Errorw("failed to send message via Telegram API", "error", err, "userID", userID)
		}
	}

}
