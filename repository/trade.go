package repository

import "fmt"

type TradeRepository struct{}

func NewTradeRepository() *TradeRepository {
	return &TradeRepository{}
}

func (r *TradeRepository) SaveSession(symbol, interval string, sl float64) {
	fmt.Printf("[repository] Session saved: %s %s SL: %.2f\n", symbol, interval, sl)
}
