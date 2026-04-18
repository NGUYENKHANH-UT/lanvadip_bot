package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"lanvadip-bot/internal/model"
	"lanvadip-bot/internal/store"

	"github.com/google/generative-ai-go/genai"
	"github.com/payOSHQ/payos-lib-golang/v2"
	"go.uber.org/zap"
)

type AIService interface {
	AnalyzeAndRespond(ctx context.Context, userID int64, message string) (string, error)
	ClearSession(userID int64)
	Close()
	CancelPendingOrder(ctx context.Context, userID int64)
}

type aiService struct {
	client      *genai.Client
	model       *genai.GenerativeModel
	menuStore   store.MenuStore
	logger      *zap.SugaredLogger
	sessions    sync.Map
	payosClient *payos.PayOS
	fsmService  FSMService
	orderStore  store.OrderStore
}

func NewAIService(client *genai.Client, menuStore store.MenuStore, logger *zap.SugaredLogger, payosClient *payos.PayOS, fsmService FSMService, orderStore store.OrderStore) AIService {
	model := client.GenerativeModel("gemini-2.5-flash-lite")

	temp := float32(0.2)
	model.Temperature = &temp

	model.SystemInstruction = &genai.Content{Parts: []genai.Part{genai.Text(
		`Bạn là nhân viên nhận order siêu dễ thương của quán "Trà sữa Lan và Địp".
Xưng hô: Gọi mình là "Quán" hoặc "Dạ", gọi khách là "Cục dàng".
Nhiệm vụ: Chào hỏi, tư vấn menu, nhận order và CHỐT ĐƠN.
LƯU Ý QUAN TRỌNG: 
- Khi có kết quả từ get_menu, BẮT BUỘC PHẢI in y hệt đoạn HTML hệ thống đưa.
- BẤT CỨ KHI NÀO khách báo chọn món, thêm món, bớt món, bạn PHẢI gọi hàm calculate_total và truyền vào TOÀN BỘ danh sách món. SAU ĐÓ IN CHI TIẾT HÓA ĐƠN ĐÓ RA.
- KHI KHÁCH ĐỒNG Ý CHỐT ĐƠN (ví dụ: "chốt", "ok lấy đi", "tính tiền"), bạn BẮT BUỘC gọi hàm checkout để tạo mã QR thanh toán.
- Sau khi gọi checkout, hãy gửi Link thanh toán cho khách và nói rõ: "Cục dàng nhấn vào link để quét mã QR thanh toán nha!".`,
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
		client:      client,
		model:       model,
		menuStore:   menuStore,
		logger:      logger,
		payosClient: payosClient,
		fsmService:  fsmService,
		orderStore:  orderStore,
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

	req := payos.CreatePaymentLinkRequest{
		OrderCode:   orderCode,
		Amount:      total,
		Description: "Thanh toan tra sua",
		CancelUrl:   "https://t.me/lanvadip_bot?start=cancel",
		ReturnUrl:   "https://t.me/lanvadip_bot", // URL khi thanh toán thành công
	}

	res, err := s.payosClient.PaymentRequests.Create(ctx, req)
	if err != nil {
		return "", fmt.Errorf("Error call API PayOS: %w", err)
	}

	s.fsmService.SetState(ctx, userID, model.StateAwaitingPayment)

	return fmt.Sprintf("%s\n\n[HỆ THỐNG]: Đã tạo link thanh toán thành công. Yêu cầu báo khách click vào link này để quét mã QR: %s", billText, res.CheckoutUrl), nil
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
			case "checkout":
				result, errCall = s.processCheckout(ctx, fnCall.Args, userID)
			default:
				result = "system error: invalid function name"
			}

			if errCall != nil {
				result = fmt.Sprintf("system error: %v", errCall)
				s.logger.Error(result)
			}

			resp, err = session.SendMessage(ctx, genai.FunctionResponse{
				Name:     fnCall.Name,
				Response: map[string]interface{}{"result": result},
			})
			if err != nil {
				return "", err
			}
		}
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("received empty response from Gemini API")
	}

	return fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0]), nil
}

func (s *aiService) CancelPendingOrder(ctx context.Context, userID int64) {
	// Lấy mã đơn đang nợ
	orderCode, err := s.fsmService.GetUserPendingOrder(ctx, userID)
	if err != nil || orderCode == 0 {
		return
	}

	// 1. CẬP NHẬT DATABASE THÀNH CANCELLED
	err = s.orderStore.UpdateOrderStatus(ctx, orderCode, "CANCELLED")
	if err != nil {
		s.logger.Errorw("Lỗi hủy DB", "err", err)
	}

	// 2. CHỦ ĐỘNG HỦY LINK TRÊN PAYOS
	// (Kệ lỗi nếu link đã bị hủy trước đó trên web)
	cancelMessage := "Khách hủy đơn"
	_, _ = s.payosClient.PaymentRequests.Cancel(ctx, int(orderCode), &cancelMessage)

	// 3. XÓA VẾT NỢ TRONG REDIS
	s.fsmService.SetUserPendingOrder(ctx, userID, 0)
}
