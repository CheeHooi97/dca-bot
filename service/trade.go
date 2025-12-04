package service

import (
	"bot-1/bot"
	"bot-1/repository"
)

type TradeService struct {
	repo *repository.TradeRepository
}

func NewTradeService() *TradeService {
	return &TradeService{
		repo: repository.NewTradeRepository(),
	}
}

func (s *TradeService) Start(symbol, interval, token string, sl float64) error {

	// save user session
	s.repo.SaveSession(symbol, interval, sl)

	// run your existing bot logic
	bot.Bot(symbol, interval, token, sl)

	return nil
}
