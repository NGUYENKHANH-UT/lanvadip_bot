package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"lanvadip-bot/internal/model"
	"lanvadip-bot/internal/store"

	"github.com/google/generative-ai-go/genai"
	"go.uber.org/zap"
)

type AIService interface {
	AnalyzeAndRespond(ctx context.Context, userID int64, message string) (string, error)
	ClearSession(userID int64)
	Close()
}

type aiService struct {
	client    *genai.Client
	model     *genai.GenerativeModel
	menuStore store.MenuStore
	logger    *zap.SugaredLogger
	sessions  sync.Map
}

func NewAIService(client *genai.Client, menuStore store.MenuStore, logger *zap.SugaredLogger) AIService {
	model := client.GenerativeModel("gemini-2.5-flash-lite")

	temp := float32(0.2)
	model.Temperature = &temp

	model.SystemInstruction = &genai.Content{Parts: []genai.Part{genai.Text(
		`Bạn là nhân viên nhận order siêu dễ thương của quán "Trà sữa Lan và Địp".
Xưng hô: Gọi mình là "Quán" hoặc "Dạ", gọi khách là "Cục dàng".
Nhiệm vụ: Chào hỏi, tư vấn menu, nhận order và chốt đơn.
LƯU Ý QUAN TRỌNG: 
- Khi dùng get_menu, hệ thống sẽ trả về một chuỗi Menu đã được định dạng HTML sẵn. BẠN BẮT BUỘC PHẢI IN Y HỆT ĐOẠN HTML ĐÓ RA MÀN HÌNH, tuyệt đối không được tự ý sửa đổi, xóa bớt thẻ hoặc viết lại theo ý mình.
- BẤT CỨ KHI NÀO khách báo chọn món, thêm món, bớt món, hoặc đổi ý, bạn PHẢI gọi hàm calculate_total và truyền vào TOÀN BỘ danh sách món mà khách đang chốt. 
- Hãy luôn chú ý đến [Thời gian hiện tại] để giao tiếp tự nhiên.`,
	)}}

	model.Tools = []*genai.Tool{
		{
			FunctionDeclarations: []*genai.FunctionDeclaration{
				{
					Name:        "get_menu",
					Description: "Lấy danh sách thực đơn trà sữa và cà phê hiện có của quán. Bao gồm mô tả món và giá size M, L.",
				},
				{
					Name:        "calculate_total",
					Description: "Tính tổng tiền hóa đơn. BẮT BUỘC gọi hàm này khi khách thay đổi order (thêm/bớt/đổi) hoặc muốn chốt đơn. Truyền vào TOÀN BỘ danh sách món ĐÃ CHỐT cuối cùng.",
					Parameters: &genai.Schema{
						Type: genai.TypeObject,
						Properties: map[string]*genai.Schema{
							"orders": {
								Type:        genai.TypeArray,
								Description: "Danh sách toàn bộ món khách đặt",
								Items: &genai.Schema{
									Type: genai.TypeObject,
									Properties: map[string]*genai.Schema{
										"item_code": {Type: genai.TypeString, Description: "Mã món (VD: TS01, CF02)"},
										"size":      {Type: genai.TypeString, Description: "Size món (M hoặc L)"},
										"quantity":  {Type: genai.TypeInteger, Description: "Số lượng"},
									},
									Required: []string{"item_code", "size", "quantity"},
								},
							},
						},
						Required: []string{"orders"},
					},
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

func (s *aiService) ClearSession(userID int64) {
	s.sessions.Delete(userID)
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

	menuStr := "Đây là dữ liệu menu từ hệ thống (Dạng HTML). Bạn BẮT BUỘC phải copy y hệt và in ra màn hình cho khách:\n\n"
	for _, item := range items {
		menuStr += fmt.Sprintf("🥤 <b>[%s] %s</b> - <i>%s</i>\n📝 %s\n💰 Giá: <b>%dk</b> (M) | <b>%dk</b> (L)\n\n",
			item.CategoryName, item.ItemCode, item.Name, item.Description, item.PriceM/1000, item.PriceL/1000)
	}
	return menuStr, nil
}

func (s *aiService) processCalculateTotal(ctx context.Context, args map[string]interface{}) (string, error) {
	ordersRaw, ok := args["orders"].([]interface{})
	if !ok || len(ordersRaw) == 0 {
		return "Thông báo hệ thống: Giỏ hàng đang trống.", nil
	}

	items, err := s.menuStore.GetAvailableMenu(ctx)
	if err != nil {
		return "", fmt.Errorf("lỗi lấy menu: %w", err)
	}

	menuMap := make(map[string]model.MenuItem)
	for _, item := range items {
		menuMap[item.ItemCode] = item
	}

	var total int
	receipt := "--- HÓA ĐƠN TẠM TÍNH ---\n"

	for _, o := range ordersRaw {
		orderData, ok := o.(map[string]interface{})
		if !ok {
			continue
		}

		code, _ := orderData["item_code"].(string)
		size, _ := orderData["size"].(string)

		var qty int
		switch v := orderData["quantity"].(type) {
		case float64:
			qty = int(v)
		case int:
			qty = v
		default:
			qty = 1
		}

		menuItem, exists := menuMap[code]
		if !exists {
			receipt += fmt.Sprintf("- Lỗi: Món %s không tồn tại\n", code)
			continue
		}

		price := menuItem.PriceM
		if size == "L" {
			price = menuItem.PriceL
		}

		lineTotal := price * qty
		total += lineTotal
		receipt += fmt.Sprintf("- %dx %s (Size %s): %dđ\n", qty, menuItem.Name, size, lineTotal)
	}

	receipt += fmt.Sprintf("------------------------\nTỔNG CỘNG: %dđ", total)
	return receipt, nil
}

func (s *aiService) AnalyzeAndRespond(ctx context.Context, userID int64, message string) (string, error) {
	var session *genai.ChatSession

	if existingSession, ok := s.sessions.Load(userID); ok {
		session = existingSession.(*genai.ChatSession)
	} else {
		session = s.model.StartChat()
		s.sessions.Store(userID, session)
	}

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

			switch fnCall.Name {
			case "get_menu":
				result, errCall = s.processGetMenu(ctx)
			case "calculate_total":
				result, errCall = s.processCalculateTotal(ctx, fnCall.Args)
			default:
				result = "system error: invalid function name"
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
