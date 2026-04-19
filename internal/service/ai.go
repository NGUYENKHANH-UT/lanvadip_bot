package service

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"lanvadip-bot/internal/model"
	"lanvadip-bot/internal/store"

	"github.com/payOSHQ/payos-lib-golang/v2"
	"github.com/sashabaranov/go-openai"
	"go.uber.org/zap"
)

const groqModel = "llama-3.3-70b-versatile"

// Match tất cả HTML tags, sau đó giữ lại các tag hợp lệ của Telegram
var anyTagRegex = regexp.MustCompile(`<[^>]+>`)
var validTags = map[string]bool{
	"b": true, "/b": true,
	"i": true, "/i": true,
	"code": true, "/code": true,
	"pre": true, "/pre": true,
	"a": true, "/a": true,
}

func sanitizeTelegramHTML(text string) string {
	return anyTagRegex.ReplaceAllStringFunc(text, func(tag string) string {
		// Lấy tên tag (bỏ < > và attributes)
		inner := strings.TrimPrefix(tag, "<")
		inner = strings.TrimSuffix(inner, ">")
		tagName := strings.ToLower(strings.Fields(inner)[0])
		if validTags[tagName] {
			return tag
		}
		return ""
	})
}

type AIService interface {
	AnalyzeAndRespond(ctx context.Context, userID int64, message string) (string, error)
	ClearSession(userID int64)
	Close()
	CancelPendingOrder(ctx context.Context, userID int64)
}

type aiService struct {
	client      *openai.Client
	menuStore   store.MenuStore
	logger      *zap.SugaredLogger
	sessions    sync.Map // map[int64][]openai.ChatCompletionMessage
	menuCache   sync.Map
	payosClient *payos.PayOS
	fsmService  FSMService
	orderStore  store.OrderStore
}

var systemPrompt = `Bạn là nhân viên nhận order siêu dễ thương của quán "Trà sữa Lan và Địp".
Xưng hô: Gọi mình là "Quán" hoặc "Dạ", gọi khách là "Cục vàng".
Nhiệm vụ: Chào hỏi, tư vấn menu, nhận order, LẤY THÔNG TIN GIAO HÀNG và CHỐT ĐƠN.
LƯU Ý QUAN TRỌNG:
1. VỀ MENU VÀ ORDER:
- Khi gọi hàm get_menu, kết quả trả về có chứa token [[MENU_CONTENT]]. Hãy giữ nguyên token đó trong câu trả lời, hệ thống sẽ tự thay thế bằng nội dung menu thật.
- BẤT CỨ KHI NÀO khách báo chọn món, thêm món, bớt món: PHẢI gọi hàm calculate_total (truyền vào toàn bộ danh sách món đang chọn) và IN CHI TIẾT HÓA ĐƠN TẠM TÍNH RA.

2. QUY TRÌNH CHỐT ĐƠN (Phải thực hiện theo đúng thứ tự):
- Bước 1 (Lấy thông tin): Khi khách muốn chốt đơn (ví dụ: "chốt", "tính tiền", "ok lấy đi"), BẮT BUỘC PHẢI kiểm tra xem đã có đủ 3 thông tin chưa: Tên người nhận, Số điện thoại và Địa chỉ giao. Nếu thiếu bất kỳ thông tin nào, phải hỏi khách cho đủ. (Thời gian giao mặc định là "Càng sớm càng tốt" nếu khách không dặn).
- Bước 2 (Gọi hàm thanh toán): TUYỆT ĐỐI KHÔNG gọi hàm checkout khi chưa đủ thông tin. CHỈ GỌI hàm checkout KHI VÀ CHỈ KHI đã có đầy đủ danh sách món VÀ 3 thông tin giao hàng.
- Bước 3 (Gửi link): Sau khi hàm checkout thực thi thành công, hãy gửi Link thanh toán cho khách và nói rõ: "Dạ Cục dàng nhấn vào link để quét mã QR thanh toán nha!".

3. VỀ FORMAT TRẢ LỜI:
- TUYỆT ĐỐI KHÔNG được viết <function=...>, <tool>, hay bất kỳ XML/function tag nào trong câu trả lời.
- Câu trả lời chỉ được chứa text thuần và các HTML tag hợp lệ của Telegram: <b>, <i>, <code>, <a>, <pre>.`

var tools = []openai.Tool{
	{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "get_menu",
			Description: "Lấy danh sách thực đơn trà sữa và cà phê hiện có của quán. Bao gồm mô tả món và giá size M, L.",
			Parameters:  json.RawMessage(`{"type": "object", "properties": {}}`),
		},
	},
	{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "calculate_total",
			Description: "Tính tổng tiền hóa đơn. BẮT BUỘC gọi hàm này khi khách thay đổi order (thêm/bớt/đổi) hoặc muốn chốt đơn. Truyền vào TOÀN BỘ danh sách món ĐÃ CHỐT cuối cùng.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"orders": {
						"type": "array",
						"description": "Danh sách toàn bộ món khách đặt",
						"items": {
							"type": "object",
							"properties": {
								"item_code": {"type": "string", "description": "Mã món (VD: TS01, CF02)"},
								"size":      {"type": "string", "description": "Size món (M hoặc L)"},
								"quantity":  {"type": "integer", "description": "Số lượng"}
							},
							"required": ["item_code", "size", "quantity"]
						}
					}
				},
				"required": ["orders"]
			}`),
		},
	},
	{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "checkout",
			Description: "Gọi hàm này ĐỂ CHỐT ĐƠN VÀ LÊN MÃ QR. Chỉ gọi khi đã xin đủ Tên, SĐT, Địa chỉ của khách.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"orders": {
						"type": "array",
						"description": "Danh sách toàn bộ món khách đặt",
						"items": {
							"type": "object",
							"properties": {
								"item_code": {"type": "string", "description": "Mã món (VD: TS01, CF02)"},
								"size":      {"type": "string", "description": "Size món (M hoặc L)"},
								"quantity":  {"type": "integer", "description": "Số lượng"}
							},
							"required": ["item_code", "size", "quantity"]
						}
					},
					"customer_name": {"type": "string", "description": "Tên người nhận"},
					"phone":         {"type": "string", "description": "Số điện thoại liên hệ"},
					"address":       {"type": "string", "description": "Địa chỉ giao hàng"},
					"delivery_time": {"type": "string", "description": "Thời gian giao (VD: 15:00, ASAP)"}
				},
				"required": ["orders", "customer_name", "phone", "address"]
			}`),
		},
	},
}

func NewAIService(client *openai.Client, menuStore store.MenuStore, logger *zap.SugaredLogger, payosClient *payos.PayOS, fsmService FSMService, orderStore store.OrderStore) AIService {
	return &aiService{
		client:      client,
		menuStore:   menuStore,
		logger:      logger,
		payosClient: payosClient,
		fsmService:  fsmService,
		orderStore:  orderStore,
	}
}

func (s *aiService) Close() {}

func (s *aiService) ClearSession(userID int64) {
	s.sessions.Delete(userID)
}

func (s *aiService) getHistory(userID int64) []openai.ChatCompletionMessage {
	if val, ok := s.sessions.Load(userID); ok {
		return val.([]openai.ChatCompletionMessage)
	}
	return []openai.ChatCompletionMessage{}
}

func (s *aiService) saveHistory(userID int64, history []openai.ChatCompletionMessage) {
	// Giới hạn 40 messages để tránh vượt context limit của Groq
	if len(history) > 40 {
		history = history[len(history)-40:]
	}
	s.sessions.Store(userID, history)
}

// --- Function handlers ---

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

	menuStr := ""
	for _, item := range items {
		menuStr += fmt.Sprintf("🥤 <b>[%s] %s</b> - <i>%s</i>\n📝 %s\n💰 Giá: <b>%dk</b> (M) | <b>%dk</b> (L)\n\n",
			item.CategoryName, item.ItemCode, item.Name, item.Description, item.PriceM/1000, item.PriceL/1000)
	}

	s.menuCache.Store("latest", menuStr)

	return "Đã lấy menu thành công. Hãy gửi nội dung [[MENU_CONTENT]] cho khách rồi hỏi khách muốn chọn món gì.", nil
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

func (s *aiService) processCheckout(ctx context.Context, args map[string]interface{}, userID int64) (string, error) {
	billText, _ := s.processCalculateTotal(ctx, args)

	ordersRaw, _ := args["orders"].([]interface{})
	items, _ := s.menuStore.GetAvailableMenu(ctx)
	menuMap := make(map[string]model.MenuItem)
	for _, item := range items {
		menuMap[item.ItemCode] = item
	}

	var total int
	for _, o := range ordersRaw {
		orderData, _ := o.(map[string]interface{})
		code, _ := orderData["item_code"].(string)
		size, _ := orderData["size"].(string)
		qty := int(orderData["quantity"].(float64))
		price := menuMap[code].PriceM
		if size == "L" {
			price = menuMap[code].PriceL
		}
		total += price * qty
	}

	if total == 0 {
		return "Giỏ hàng trống, không thể chốt đơn.", nil
	}

	orderCode := time.Now().Unix()
	s.fsmService.SetOrderUser(ctx, orderCode, userID)
	s.fsmService.SetUserPendingOrder(ctx, userID, orderCode)

	var dbItems []model.OrderItem
	for _, o := range ordersRaw {
		orderData, _ := o.(map[string]interface{})
		code, _ := orderData["item_code"].(string)
		size, _ := orderData["size"].(string)
		qty := int(orderData["quantity"].(float64))
		price := menuMap[code].PriceM
		if size == "L" {
			price = menuMap[code].PriceL
		}
		dbItems = append(dbItems, model.OrderItem{
			ItemCode: code, Size: size, Quantity: qty, Price: price,
		})
	}

	err := s.orderStore.CreateOrder(ctx, orderCode, userID, total, dbItems)
	if err != nil {
		s.logger.Errorw("Failed to save order to DB", "error", err)
		return "Hệ thống lỗi khi lưu đơn hàng, cục dàng chờ xíu thử lại nha!", nil
	}

	customerName, _ := args["customer_name"].(string)
	phone, _ := args["phone"].(string)
	address, _ := args["address"].(string)
	deliveryTime, _ := args["delivery_time"].(string)
	if deliveryTime == "" {
		deliveryTime = "ASAP"
	}

	deliveryInfo := fmt.Sprintf("Tên: %s | SĐT: %s | ĐC: %s | Giờ: %s", customerName, phone, address, deliveryTime)
	s.fsmService.SetOrderDeliveryInfo(ctx, orderCode, deliveryInfo)

	req := payos.CreatePaymentLinkRequest{
		OrderCode:   orderCode,
		Amount:      total,
		Description: "Thanh toan tra sua",
		CancelUrl:   "https://t.me/lanvadip_bot?start=cancel",
		ReturnUrl:   "https://t.me/lanvadip_bot",
	}

	res, err := s.payosClient.PaymentRequests.Create(ctx, req)
	if err != nil {
		return "", fmt.Errorf("error call API PayOS: %w", err)
	}

	s.fsmService.SetState(ctx, userID, model.StateAwaitingPayment)

	return fmt.Sprintf("%s\n\n[HỆ THỐNG]: Đã tạo link thanh toán thành công. Yêu cầu báo khách click vào link này để quét mã QR: %s", billText, res.CheckoutUrl), nil
}

// --- Core loop ---

func (s *aiService) AnalyzeAndRespond(ctx context.Context, userID int64, message string) (string, error) {
	history := s.getHistory(userID)

	loc := time.FixedZone("ICT", 7*3600)
	now := time.Now().In(loc).Format("15:04 02/01/2006")
	userMsg := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: fmt.Sprintf("[Thông tin hệ thống: Khách nhắn lúc %s]\nKhách nói: %s", now, message),
	}
	history = append(history, userMsg)

	for i := 0; i < 5; i++ {
		messages := append([]openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
		}, history...)

		resp, err := s.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
			Model:    groqModel,
			Messages: messages,
			Tools:    tools,
		})
		if err != nil {
			// Nếu lỗi 400, thử lại với history rút gọn để tránh context bị corrupt
			if strings.Contains(err.Error(), "400") && len(history) > 2 {
				s.logger.Warnw("Groq 400 error, retrying with truncated history", "userID", userID, "attempt", i)
				history = history[len(history)-2:]
				continue
			}
			return "", fmt.Errorf("failed to call Groq API: %w", err)
		}

		choice := resp.Choices[0]

		if len(choice.Message.ToolCalls) == 0 {
			cleanHistory := make([]openai.ChatCompletionMessage, 0, len(history)+1)
			for _, msg := range history {
				if msg.Role == openai.ChatMessageRoleTool || len(msg.ToolCalls) > 0 {
					continue
				}
				cleanHistory = append(cleanHistory, msg)
			}
			cleanHistory = append(cleanHistory, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleAssistant,
				Content: choice.Message.Content,
			})
			s.saveHistory(userID, cleanHistory)

			finalText := choice.Message.Content
			if strings.Contains(finalText, "[[MENU_CONTENT]]") {
				if cached, ok := s.menuCache.Load("latest"); ok {
					finalText = strings.ReplaceAll(finalText, "[[MENU_CONTENT]]", cached.(string))
				}
			}

			// Sanitize để loại bỏ các tag không hợp lệ với Telegram HTML
			return sanitizeTelegramHTML(finalText), nil
		}

		history = append(history, choice.Message)

		for _, toolCall := range choice.Message.ToolCalls {
			var args map[string]interface{}
			_ = json.Unmarshal([]byte(toolCall.Function.Arguments), &args)

			var result string
			var errCall error

			switch toolCall.Function.Name {
			case "get_menu":
				result, errCall = s.processGetMenu(ctx)
			case "calculate_total":
				result, errCall = s.processCalculateTotal(ctx, args)
			case "checkout":
				result, errCall = s.processCheckout(ctx, args, userID)
			default:
				result = "system error: invalid function name"
			}

			if errCall != nil {
				result = fmt.Sprintf("system error: %v", errCall)
				s.logger.Error(result)
			}

			history = append(history, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				ToolCallID: toolCall.ID,
				Content:    result,
			})
		}
	}

	return "", fmt.Errorf("exceeded maximum tool call iterations")
}

func (s *aiService) CancelPendingOrder(ctx context.Context, userID int64) {
	orderCode, err := s.fsmService.GetUserPendingOrder(ctx, userID)
	if err != nil || orderCode == 0 {
		return
	}

	err = s.orderStore.UpdateOrderStatus(ctx, orderCode, "CANCELLED")
	if err != nil {
		s.logger.Errorw("Lỗi hủy DB", "err", err)
	}

	cancelMessage := "Khách hủy đơn"
	_, _ = s.payosClient.PaymentRequests.Cancel(ctx, int(orderCode), &cancelMessage)

	s.fsmService.SetUserPendingOrder(ctx, userID, 0)
}
