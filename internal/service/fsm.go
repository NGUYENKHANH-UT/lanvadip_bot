package service

import (
	"context"
	"fmt"
	"lanvadip-bot/internal/store"
)

type FSMService interface {
	SetState(ctx context.Context, userID int64, state string) error
	GetState(ctx context.Context, userID int64) (string, error)
	ClearState(ctx context.Context, userID int64) error
	SetOrderUser(ctx context.Context, orderCode int64, userID int64) error
	GetOrderUser(ctx context.Context, orderCode int64) (int64, error)
	SetUserPendingOrder(ctx context.Context, userID int64, orderCode int64) error // THÊM
	GetUserPendingOrder(ctx context.Context, userID int64) (int64, error)
}

type redisFSMService struct {
	store store.FSMStore
}

func NewRedisFSMService(store store.FSMStore) FSMService {
	return &redisFSMService{
		store: store,
	}
}

func (s *redisFSMService) SetState(ctx context.Context, userID int64, state string) error {
	key := fmt.Sprintf("fsm:user:%d", userID)
	return s.store.SetState(ctx, key, state)
}

func (s *redisFSMService) GetState(ctx context.Context, userID int64) (string, error) {
	key := fmt.Sprintf("fsm:user:%d", userID)
	return s.store.GetState(ctx, key)
}

func (s *redisFSMService) ClearState(ctx context.Context, userID int64) error {
	key := fmt.Sprintf("fsm:user:%d", userID)
	return s.store.ClearState(ctx, key)
}

func (s *redisFSMService) SetOrderUser(ctx context.Context, orderCode int64, userID int64) error {
	key := fmt.Sprintf("order_map:%d", orderCode)
	// Lợi dụng hàm SetState có sẵn để lưu UserID dưới dạng chuỗi
	return s.store.SetState(ctx, key, fmt.Sprintf("%d", userID))
}

func (s *redisFSMService) GetOrderUser(ctx context.Context, orderCode int64) (int64, error) {
	key := fmt.Sprintf("order_map:%d", orderCode)
	valStr, err := s.store.GetState(ctx, key)
	if err != nil || valStr == "" {
		return 0, fmt.Errorf("không tìm thấy user cho order")
	}
	var userID int64
	fmt.Sscanf(valStr, "%d", &userID) // Ép ngược chuỗi về int64
	return userID, nil
}

func (s *redisFSMService) SetUserPendingOrder(ctx context.Context, userID int64, orderCode int64) error {
	key := fmt.Sprintf("pending_order:user:%d", userID)
	return s.store.SetState(ctx, key, fmt.Sprintf("%d", orderCode))
}

func (s *redisFSMService) GetUserPendingOrder(ctx context.Context, userID int64) (int64, error) {
	key := fmt.Sprintf("pending_order:user:%d", userID)
	valStr, err := s.store.GetState(ctx, key)
	if err != nil || valStr == "" {
		return 0, fmt.Errorf("không có đơn chờ")
	}
	var orderCode int64
	fmt.Sscanf(valStr, "%d", &orderCode)
	return orderCode, nil
}
