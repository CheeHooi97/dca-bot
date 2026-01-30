package service

import (
	"dca-bot/bot"
	"dca-bot/repository"
	"fmt"
)

type DCAService struct {
	repo *repository.DCARepository
}

func NewDCAService() *DCAService {
	return &DCAService{
		repo: repository.NewDCARepository(),
	}
}

func (s *DCAService) Start(symbol string, totalUSDT, dropPercent, sellPercent float64, fallbackBuyHours int) error {
	// dcaAmount := totalUSDT * 0.01 // buy 1% per entry

	fmt.Println("===== DCA MODE =====")
	fmt.Printf("Symbol: %s\n", symbol)
	fmt.Printf("Total USDT: %.2f\n", totalUSDT)
	fmt.Printf("Buy per entry (1%%): %.2f USDT\n", 1)
	fmt.Printf("Drop trigger: %.2f%%\n", dropPercent)
	fmt.Printf("Sell trigger: %.2f%%\n", sellPercent)

	s.repo.Save(symbol, totalUSDT, dropPercent) // optional persistence

	// run DCA bot (websocket)
	go bot.RunDCABot(symbol, totalUSDT, dcaAmount, dropPercent, sellPercent, fallbackBuyHours)

	return nil
}

