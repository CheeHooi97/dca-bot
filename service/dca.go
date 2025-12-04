package service

import (
	"bot-1/bot"
	"bot-1/repository"
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

func (s *DCAService) Start(symbol string, totalUSDT, dropPercent float64) error {
	dcaAmount := totalUSDT * 0.01 // buy 1% per entry

	fmt.Println("===== DCA MODE =====")
	fmt.Printf("Symbol: %s\n", symbol)
	fmt.Printf("Total USDT: %.2f\n", totalUSDT)
	fmt.Printf("Buy per entry (1%%): %.2f USDT\n", dcaAmount)
	fmt.Printf("Drop trigger: %.2f%%\n", dropPercent)

	s.repo.Save(symbol, totalUSDT, dropPercent) // optional persistence

	// run DCA bot (websocket)
	go bot.RunDCABot(symbol, totalUSDT, dcaAmount, dropPercent)

	return nil
}
