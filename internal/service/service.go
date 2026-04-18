package service

import (
	"lanvadip-bot/internal/store"
)

type Service struct {
	FSM FSMService
}

func NewService(s store.Storage) Service {
	return Service{
		FSM: NewRedisFSMService(s.FSM),
	}
}
