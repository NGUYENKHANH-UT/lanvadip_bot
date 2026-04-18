package store

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	StateStart           = "START"
	StateOrdering        = "ORDERING"
	StateAwaitingPayment = "AWAITING_PAYMENT"
	StateCompleted       = "COMPLETED"
)

type FSMStore interface {
	SetState(ctx context.Context, key, state string) error
	GetState(ctx context.Context, key string) (string, error)
	ClearState(ctx context.Context, key string) error
}

type redisFSMStore struct {
	client *redis.Client
}

func NewRedisFSMStore(client *redis.Client) FSMStore {
	return &redisFSMStore{
		client: client,
	}
}

func (s *redisFSMStore) SetState(ctx context.Context, key, state string) error {
	return s.client.Set(ctx, key, state, 24*time.Hour).Err()
}

func (s *redisFSMStore) GetState(ctx context.Context, key string) (string, error) {
	state, err := s.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return StateStart, nil
	}
	if err != nil {
		return "", err
	}
	return state, nil
}

func (s *redisFSMStore) ClearState(ctx context.Context, key string) error {
	return s.client.Del(ctx, key).Err()
}
