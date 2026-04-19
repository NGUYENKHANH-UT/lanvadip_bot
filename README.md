# 🧋 LanVaDip Bot — AI Chatbot Nhận Order Trà Sữa Tự Động

> Bot Telegram AI đóng vai "mẹ chủ quán" để tự động tiếp nhận đơn hàng, tính tiền và xử lý thanh toán — tích hợp hệ sinh thái Casso.

---

## 🚀 Hướng Dẫn Cài Đặt & Chạy

### Yêu cầu

- Go 1.25+
- Docker & Docker Compose
- Redis (qua Docker)
- Tài khoản Telegram Bot (từ [@BotFather](https://t.me/BotFather))
- Tài khoản [payOS](https://payos.vn/) (Client ID, API Key, Checksum Key)
- API Key của [Groq](https://console.groq.com/) (LLM provider)

### Chạy bằng Docker Compose (Khuyến nghị)

```bash
# 1. Clone repository
git clone <repo-url>
cd lanvadip-bot

# 2. Tạo file .env từ mẫu
cp .env.example .env
# Điền đầy đủ các biến môi trường vào .env

# 3. Khởi động toàn bộ stack
docker compose up -d
```

### Chạy thủ công (Development)

```bash
# 1. Cài đặt dependencies
go mod download

# 2. Chạy Redis (cần Docker)
docker compose up -d redis

# 3. Chạy migrations để tạo schema SQLite
export DB_ADDR="sqlite3://./data/bot.db"
make migrate-up

# 4. Khởi động server
make run
```

### Biến Môi Trường

| Biến | Mô tả | Bắt buộc |
|------|--------|----------|
| `TELEGRAM_BOT_TOKEN` | Token bot từ BotFather | ✅ |
| `GROQ_API_KEY` | API key Groq (dùng LLaMA 3.3 70B) | ✅ |
| `PAYOS_CLIENT_ID` | Client ID của payOS | ✅ |
| `PAYOS_API_KEY` | API Key của payOS | ✅ |
| `PAYOS_CHECKSUM_KEY` | Checksum Key để xác thực webhook | ✅ |
| `REDIS_URL` | URL kết nối Redis | ✅ |
| `DB_PATH` | Đường dẫn file SQLite | ✅ |
| `GROUP_CHAT_ID` | ID nhóm Telegram nhận thông báo đơn hàng | ⚠️ |
| `PORT` | Port HTTP server (mặc định `:8080`) | ❌ |

---

## 🔗 Bot & Kênh Demo

| | |
|--|--|
| **Telegram Bot** | [@lanvadip_bot](https://t.me/lanvadip_bot) |
| **Nhóm thông báo Admin** | https://t.me/+8kIJdQPsqq8xYWQ9 |
| **Deployed trên** | [Fly.io](https://fly.io) — region `sin` (Singapore) |

> ⚠️ **Lưu ý về nhóm Admin:** Link nhóm Telegram trên là link mời dạng private. Ban giám khảo có thể join vào nhóm qua link này để xem thông báo đơn hàng mới theo thời gian thực khi test bot.

---

## 🎥 Video Demo

✍️ [demo](https://drive.google.com/file/d/1McHvN8BSeIWQF6D_z3Aa-TYK27H40V-y/view?usp=sharing)

---

## 💻 Giới Thiệu Dự Án

### a. Tổng Quan

**LanVaDip Bot** là một chatbot Telegram AI được xây dựng để giải quyết bài toán thực tế: mẹ chủ quán trà sữa không thể trả lời kịp lượng đơn online ngày càng tăng. Bot đóng vai một nhân viên ảo dễ thương — xưng là "Quán", gọi khách là "Cục vàng" — có khả năng tiếp nhận order qua ngôn ngữ tự nhiên, tính tiền, tạo QR thanh toán và thông báo đơn mới về nhóm admin.

Hệ thống được xây dựng trên nền tảng **Golang** với kiến trúc production-ready: FSM lưu trạng thái trên Redis, cơ sở dữ liệu SQLite persistent, AI sử dụng Function Calling, và tích hợp thanh toán qua payOS.

### b. Tính Năng Chính & Hướng Dẫn Sử Dụng

**Dành cho Khách hàng:**
- Nhắn tin tự nhiên để xem menu, hỏi về món, thêm/bớt/đổi order
- Bot tự động tính hóa đơn tạm tính sau mỗi thay đổi
- Cung cấp tên, SĐT, địa chỉ để chốt đơn
- Nhận link thanh toán VietQR (qua payOS), quét mã và hoàn tất
- Gõ `/cancel` để hủy link thanh toán và sửa đơn hàng

**Dành cho Admin:**
- Nhận thông báo ngay lập tức vào nhóm Telegram khi có đơn hàng thanh toán thành công
- Thông báo kèm đầy đủ thông tin: tên khách, SĐT, địa chỉ, giờ giao

### c. Điểm Nổi Bật

- **AI Function Calling**: Bot không chỉ trả lời văn bản — nó chủ động gọi các hàm nghiệp vụ (lấy menu, tính tiền, tạo QR) trong nội bộ, đảm bảo dữ liệu luôn chính xác.

- **Xử lý Trạng Thái Bền Vững**: FSM lưu trên Redis đảm bảo giỏ hàng không mất khi server restart hoặc deploy bản mới.

- **Webhook Worker Queue**: Webhook từ payOS được đưa vào Go Channel và xử lý bởi Worker Goroutine riêng biệt — đảm bảo phản hồi tức thì, không bao giờ timeout.

- **Graceful Shutdown**: Toàn bộ services (bot, HTTP server, workers) shutdown an toàn khi nhận tín hiệu interrupt.

- **Vệ sinh HTML output**: Tự động lọc các HTML tag không hợp lệ với Telegram trước khi gửi tin nhắn.

### d. Công Nghệ Sử Dụng

- **Backend**: Go 1.25, chi router, go-telegram/bot, go-openai (Groq adapter)
- **AI**: Groq API — model `llama-3.3-70b-versatile` với Function Calling
- **State Management**: Redis (FSM + giỏ hàng), SQLite (dữ liệu bền vững)
- **Thanh toán**: payOS SDK (Go) + VietQR, xác thực webhook HMAC-SHA256
- **Infrastructure**: Fly.io, Docker
- **API Docs**: Swagger (swaggo)
- **Database**: SQLite + Redis
- **Migration**: Golang-migrations

### e. Kiến Trúc Hệ Thống & Cơ Sở Dữ Liệu

**Luồng xử lý chính:**

```
Khách nhắn tin Telegram
        ↓
  go-telegram/bot (Goroutine)
        ↓
  BotHandler → FSMService (Redis)
        ↓
  AIService → Groq LLM (Function Calling)
        ↓ (khi cần)
  MenuStore (SQLite) / OrderStore (SQLite) / payOS API
        ↓
  Response → Telegram
```

**Luồng thanh toán:**

```
payOS → POST /v1/webhook/payos
        ↓
  WebhookHandler (verify HMAC-SHA256)
        ↓
  Go Channel (jobQueue)
        ↓
  PaymentWorker (3 Goroutines)
        ↓
  Update DB + Notify khách + Notify Admin Group
```

**Database Schema:**

- `categories`: Danh mục thực đơn (Trà Sữa, Cà Phê,...)
- `menu_items`: Thực đơn với giá size M/L, trạng thái available
- `orders`: Đơn hàng với trạng thái (PENDING → PAID_SUCCESS / CANCELLED)
- `order_items`: Chi tiết từng món trong đơn

**Redis Keys:**

- `fsm:user:{chatID}` — Trạng thái FSM của người dùng
- `order_map:{orderCode}` — Map order → userID
- `pending_order:user:{userID}` — Order đang chờ thanh toán
- `delivery_info:{orderCode}` — Thông tin giao hàng

### f. Cấu Trúc Dự Án

```
lanvadip-bot/
├── cmd/
│   ├── api/                  # Entrypoint: main, bot setup, HTTP server
│   └── migrate/migrations/   # SQL migration files
├── internal/
│   ├── handler/              # HTTP handlers (webhook, health) + Bot handler
│   ├── model/                # Domain models (FSM states, MenuItem, OrderItem)
│   ├── platform/             # Infra utilities (cache, db, env, transport, errs)
│   ├── service/              # Business logic (AI, FSM, Payment Worker)
│   └── store/                # Data access layer (FSM/Redis, Menu/SQLite, Order/SQLite)
├── docs/                     # Swagger generated docs
├── Dockerfile                # Multi-stage build (Go → Alpine)
├── docker-compose.yml        # Local dev stack (app + Redis)
├── fly.toml                  # Fly.io deployment config
└── .github/workflows/        # CI/CD pipeline
```

---

## 🧠 Reflection

### a. Nếu có thêm thời gian, bạn sẽ mở rộng gì?

**Xử lý ngoại lệ thanh toán (Overpay/Underpay):**
Trong kế hoạch ban đầu, tôi đã thiết kế logic xử lý trường hợp khách chuyển dư/thiếu tiền — phân biệt trạng thái `PAID_OVER` / `PAID_UNDER` và gửi thông báo kép (cho khách và admin nhóm). Do giới hạn 72h, hiện tại chỉ xử lý `PAID_SUCCESS`. Đây là tính năng nghiệp vụ quan trọng sẽ được bổ sung trong bước tiếp theo.

**Báo cáo tài chính đa kênh:**
Kế hoạch ban đầu bao gồm: (1) tự động vẽ biểu đồ doanh thu bằng `vicanso/go-charts` và gửi vào nhóm Admin theo lịch hàng ngày; (2) tích hợp **Casso Table** để đẩy từng giao dịch hoàn tất (tên khách, mã đơn, số tiền, trạng thái) thẳng vào bảng tính Casso — giúp chủ quán đối soát dòng tiền theo thời gian thực mà không cần công cụ khác. Đây là điểm tích hợp sâu nhất với hệ sinh thái Casso và sẽ là ưu tiên mở rộng kế tiếp.

**Quản lý giờ hoạt động động (Dynamic Hours):**
Thiết kế đã có đầy đủ: khung giờ mở/đóng cửa lưu trong SQLite, admin điều chỉnh qua lệnh Telegram (`/sethours`, `/holiday`), trạng thái này được inject vào System Prompt mỗi lượt hội thoại để AI tự từ chối khéo khi ngoài giờ. Chưa implement trong 72h.

**Admin commands đầy đủ:**
`/report` (thống kê hôm nay), `/orders` (xem đơn đang pending), `/menu` (bật/tắt món hết hàng).

**Casso Table Integration (ưu tiên cao):**
Mỗi khi đơn hàng hoàn tất, Worker Goroutine sẽ gọi API đẩy dữ liệu sang Casso Table — mang lại trải nghiệm đối soát tài chính chuyên nghiệp ngay trong hệ sinh thái Casso, không cần xuất dữ liệu thủ công.
**GitHub Actions CI/CD**
Push lên nhánh main → kích hoạt workflow
Build và kiểm tra Go code
flyctl deploy --remote-only → Fly.io build Docker image và deploy
Zero-downtime deployment — bot không gián đoạn khi có bản cập nhật

### b. Nếu tích hợp thêm AI API, bạn sẽ làm gì?

**Agent-based Architecture với Tool System:**
Thay vì nhúng toàn bộ menu vào prompt, xây dựng AI agent thực sự với các tool như `query_orders(filter)`, `get_inventory()`, `find_free_delivery_slot()` — giúp tiết kiệm token và scale tốt hơn khi menu lớn.

**Memory dài hạn:**
Tích hợp hệ thống memory để bot nhớ sở thích của khách quen ("order của Cục vàng tuần trước", "không đường như mọi lần").

**Multi-modal Input:**
Upload ảnh order từ menu giấy → AI parse thành danh sách món. Tích hợp voice message để nhận order bằng giọng nói.

**MCP (Model Context Protocol):**
Expose các nghiệp vụ (orders, menu, payments) thành MCP tools chuẩn hóa, cho phép tích hợp với các AI client khác trong tương lai.

---

## ✅ Checklist

- ✅ Code chạy không lỗi
- ✅ Nhận order qua ngôn ngữ tự nhiên (AI Function Calling)
- ✅ Tính tiền và in hóa đơn tạm tính
- ✅ Lấy thông tin giao hàng (Tên, SĐT, Địa chỉ)
- ✅ Tạo link thanh toán VietQR qua payOS
- ✅ Xác thực webhook payOS (HMAC-SHA256)
- ✅ Thông báo đơn thành công về nhóm Admin Telegram
- ✅ FSM trạng thái bền vững trên Redis
- ✅ Hỗ trợ hủy đơn và đặt lại (`/cancel`)
- ✅ Deploy trên Fly.io với CI/CD GitHub Actions
- ✅ SQLite với Persistent Volume trên Fly.io