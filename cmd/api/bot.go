package main

import (
	"lanvadip-bot/internal/handler"
	"lanvadip-bot/internal/service"

	"github.com/go-telegram/bot"
	"go.uber.org/zap"
)

func setupBot(token string, logger *zap.SugaredLogger, fsmService service.FSMService) (*bot.Bot, error) {
	botHandler := handler.NewBotHandler(logger, fsmService)

	opts := []bot.Option{
		bot.WithDefaultHandler(botHandler.HandleMessage),
	}

	b, err := bot.New(token, opts...)
	if err != nil {
		return nil, err
	}

	return b, nil
}
