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
