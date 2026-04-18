package service

import (
	"lanvadip-bot/internal/store"

	"github.com/google/generative-ai-go/genai"
	"go.uber.org/zap"
)

type Service struct {
	FSM FSMService
	AI  AIService
}

func NewService(s store.Storage, aiClient *genai.Client, logger *zap.SugaredLogger) Service {
	return Service{
		FSM: NewRedisFSMService(s.FSM),
		AI:  NewAIService(aiClient, s.Menu, logger),
	}
}
