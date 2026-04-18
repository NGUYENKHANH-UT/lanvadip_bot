package service

import (
	"context"
	"fmt"
	"time"

	"lanvadip-bot/internal/store"

	"github.com/google/generative-ai-go/genai"
	"go.uber.org/zap"
)

type AIService interface {
	AnalyzeAndRespond(ctx context.Context, userID int64, message string) (string, error)
	Close()
}

type aiService struct {
	client    *genai.Client
	model     *genai.GenerativeModel
	menuStore store.MenuStore
	logger    *zap.SugaredLogger
}

func NewAIService(client *genai.Client, menuStore store.MenuStore, logger *zap.SugaredLogger) AIService {
	model := client.GenerativeModel("gemini-2.5-flash-lite")

	temp := float32(0.2)
	model.Temperature = &temp

	model.SystemInstruction = &genai.Content{Parts: []genai.Part{genai.Text(
		`Bạn là nhân viên nhận order siêu dễ thương của quán "Trà sữa Lan và Địp".
Xưng hô: Gọi mình là "Quán" hoặc "Dạ", gọi khách là "Cục dàng".
Nhiệm vụ: Chào hỏi, mời khách xem menu và giải đáp thắc mắc về đồ uống.
LƯU Ý: 
- Nếu khách hỏi menu hoặc hỏi món này có vị gì, MỚI dùng công cụ get_menu để kiểm tra.
- Hãy luôn chú ý đến [Thời gian hiện tại] mà hệ thống cung cấp kèm theo lời khách để linh hoạt trả lời (ví dụ: chào buổi sáng/tối).`,
	)}}

	model.Tools = []*genai.Tool{
		{
			FunctionDeclarations: []*genai.FunctionDeclaration{
				{
					Name:        "get_menu",
					Description: "Lấy danh sách thực đơn trà sữa và cà phê hiện có của quán. Bao gồm mô tả món và giá size M, L.",
				},
			},
		},
	}

	return &aiService{
		client:    client,
		model:     model,
		menuStore: menuStore,
		logger:    logger,
	}
}

func (s *aiService) Close() {
	if s.client != nil {
		s.client.Close()
	}
}

func (s *aiService) processGetMenu(ctx context.Context) (string, error) {
	items, err := s.menuStore.GetAvailableMenu(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve available menu from store: %w", err)
	}

	if len(items) == 0 {
		menuEmpty := "System notification: The menu is currently empty or unavailable."
		s.logger.Error(menuEmpty)
		return menuEmpty, nil
	}

	var menuStr string
	for _, item := range items {
		menuStr += fmt.Sprintf("[%s] %s - %s. Mô tả: %s. Giá M: %dđ, Giá L: %dđ\n",
			item.CategoryName, item.ItemCode, item.Name, item.Description, item.PriceM, item.PriceL)
	}
	return menuStr, nil
}

func (s *aiService) AnalyzeAndRespond(ctx context.Context, userID int64, message string) (string, error) {
	session := s.model.StartChat()

	loc := time.FixedZone("ICT", 7*3600)
	now := time.Now().In(loc).Format("15:04 02/01/2006")
	dynamicPrompt := fmt.Sprintf("[Thông tin hệ thống: Khách nhắn lúc %s]\nKhách nói: %s", now, message)

	resp, err := session.SendMessage(ctx, genai.Text(dynamicPrompt))
	if err != nil {
		return "", fmt.Errorf("failed to send message to Gemini API: %w", err)
	}

	for _, part := range resp.Candidates[0].Content.Parts {
		if fnCall, ok := part.(genai.FunctionCall); ok {
			var result interface{}
			var errCall error

			if fnCall.Name == "get_menu" {
				result, errCall = s.processGetMenu(ctx)
			}

			if errCall != nil {
				result = fmt.Sprintf("system error occurred during function execution: %v", errCall)
				s.logger.Error(result)
			} else if result == nil {
				result = "system error: function returned nil result"
				s.logger.Error(result)
			}

			resp, err = session.SendMessage(ctx, genai.FunctionResponse{
				Name:     fnCall.Name,
				Response: map[string]interface{}{"result": result},
			})
			if err != nil {
				return "", fmt.Errorf("failed to send function response back to Gemini API: %w", err)
			}
		}
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("received empty response from Gemini API")
	}

	return fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0]), nil
}
