package service

import (
	"lanvadip-bot/internal/store"

	"github.com/payOSHQ/payos-lib-golang/v2"
	"github.com/sashabaranov/go-openai"
	"go.uber.org/zap"
)

type Service struct {
	FSM           FSMService
	AI            AIService
	PaymentWorker PaymentWorker
}

func NewService(s store.Storage, aiClient *openai.Client, logger *zap.SugaredLogger, payosClient *payos.PayOS, adminGroupID int64) Service {
	fsmService := NewRedisFSMService(s.FSM)
	worker := NewPaymentWorker(logger, fsmService, s.Order, adminGroupID)
	return Service{
		FSM:           fsmService,
		AI:            NewAIService(aiClient, s.Menu, logger, payosClient, fsmService, s.Order),
		PaymentWorker: worker,
	}
}
