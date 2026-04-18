package service

import (
	"lanvadip-bot/internal/store"

	"github.com/google/generative-ai-go/genai"
	"github.com/payOSHQ/payos-lib-golang/v2"
	"go.uber.org/zap"
)

type Service struct {
	FSM           FSMService
	AI            AIService
	PaymentWorker PaymentWorker
}

func NewService(s store.Storage, aiClient *genai.Client, logger *zap.SugaredLogger, payosClient *payos.PayOS) Service {
	fsmService := NewRedisFSMService(s.FSM)
	worker := NewPaymentWorker(logger, fsmService, s.Order)
	return Service{
		FSM:           fsmService,
		AI:            NewAIService(aiClient, s.Menu, logger, payosClient, fsmService, s.Order),
		PaymentWorker: worker,
	}
}
