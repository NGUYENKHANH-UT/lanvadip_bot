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

	if text == "/start cancel" {
		text = "/cancel"
	}

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

		h.aiService.ClearSession(userID)
	}

	if text == "/cancel" {
		if currentState == model.StateAwaitingPayment {
			h.fsmService.SetState(ctx, userID, model.StateOrdering)
			h.aiService.CancelPendingOrder(ctx, userID)

			systemPrompt := "[Lệnh hệ thống]: Khách đã yêu cầu hủy link thanh toán để sửa lại đơn hàng. Giỏ hàng cũ vẫn đang được giữ. Bạn BẮT BUỘC gọi hàm calculate_total để in lại hóa đơn tạm tính cho khách xem, và hỏi khách muốn thêm hay bớt món gì."
			aiResponse, err := h.aiService.AnalyzeAndRespond(ctx, userID, systemPrompt)

			if err != nil {
				aiResponse = "Dạ Quán đã hủy link thanh toán cũ. Giỏ hàng của Cục dàng vẫn còn y nguyên, Cục dàng muốn thêm bớt món gì cứ nhắn Quán nha! 📝"
			}
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID, Text: aiResponse, ParseMode: models.ParseModeHTML,
			})
			return
		}

		if currentState == model.StateOrdering || currentState == model.StateStart {
			h.fsmService.SetState(ctx, userID, model.StateStart)
			h.aiService.ClearSession(userID)

			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   "Dạ Quán đã hủy phiên order này rồi ạ. Khi nào thèm trà sữa thì Cục dàng gõ /start để gọi món lại nha! Hẹn gặp Cục dàng! 👋",
			})
			return
		}
	}

	if currentState == model.StateAwaitingPayment {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Dạ Cục dàng đang có đơn hàng chờ thanh toán. Vui lòng ấn vào link Quán vừa gửi để quét mã, hoặc gõ /cancel để hủy đơn nha! 💸",
		})
		return
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
			ChatID:    update.Message.Chat.ID,
			Text:      aiResponse,
			ParseMode: models.ParseModeHTML,
		})

		if err != nil {
			h.logger.Errorw("failed to send message via Telegram API", "error", err, "userID", userID)
		}
	}

}
