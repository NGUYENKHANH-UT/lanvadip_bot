package main

import (
	"lanvadip-bot/internal/handler"

	"github.com/go-telegram/bot"
	"go.uber.org/zap"
)

func setupBot(token string, logger *zap.SugaredLogger) (*bot.Bot, error) {
	opts := []bot.Option{
		bot.WithDefaultHandler(handler.StartHandler),
	}

	b, err := bot.New(token, opts...)
	if err != nil {
		return nil, err
	}

	return b, nil
}
