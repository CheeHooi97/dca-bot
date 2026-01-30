package bot

import (
	"context"
	"dca-bot/config"
	"dca-bot/constant"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	bybit "github.com/bybit-exchange/bybit.go.api"
	"github.com/gorilla/websocket"
)

type DCABot struct {
	Symbol         string
	DropPercent    float64
	SellPercent    float64
	TotalUSDT      float64
	OneBuyUSDT     float64
	LastBuyPrice   float64
	Started        bool
	Records        []DCARecord
	LastBuyTime    time.Time
	FallbackHours  time.Duration
	LatestDayPrice float64
	RealizedPNL    float64
	BybitClient    *bybit.Client
	Category       string
}

type DCARecord struct {
	BuyNumber     int
	Price         float64
	USDTSpent     float64
	AmountBought  float64
	RemainingUSDT float64
	TotalHoldings float64
}

func NewDCABot(symbol string, totalUSDT, dropPercent, sellPercent float64, fallbackBuyHours int) *DCABot {
	client := bybit.NewBybitHttpClient(config.BybitApiKey, config.BybitApiSecret, bybit.WithBaseURL(bybit.MAINNET))

	return &DCABot{
		Symbol:        strings.ToUpper(symbol),
		DropPercent:   dropPercent,
		SellPercent:   sellPercent,
		TotalUSDT:     totalUSDT,
		OneBuyUSDT:    totalUSDT * 0.01,
		Records:       []DCARecord{},
		FallbackHours: time.Duration(fallbackBuyHours) * time.Hour,
		BybitClient:   client,
		Category:      "spot",
	}
}

func (b *DCABot) OnPrice(price float64, token string) {
	b.LatestDayPrice = price

	if !b.Started {
		fmt.Printf("\nDCA START — FIRST BUY at %.4f\n", price)
		b.executeBuy(price, token)
		b.LastBuyPrice = price
		b.LastBuyTime = time.Now()
		b.Started = true
		return
	}

	drop := ((b.LastBuyPrice - price) / b.LastBuyPrice) * 100
	if drop >= b.DropPercent {
		fmt.Printf("PRICE DROP %.2f%% → BUY triggered\n", drop)
		b.executeBuy(price, token)
		b.LastBuyPrice = price
		b.LastBuyTime = time.Now()
		return
	}

	rise := ((price - b.LastBuyPrice) / b.LastBuyPrice) * 100
	if time.Since(b.LastBuyTime) >= b.FallbackHours && rise >= b.DropPercent {
		fmt.Printf("FALLBACK BUY → Rise %.2f%% after %v\n", rise, b.FallbackHours)
		b.executeBuy(price, token)
		b.LastBuyPrice = price
		b.LastBuyTime = time.Now()
		return
	}

	avgPrice := b.avgBuyPrice()
	if avgPrice > 0 {
		targetPrice := avgPrice * (1 + b.SellPercent/100)
		if price >= targetPrice {
			fmt.Printf("SELL triggered → Price %.4f ≥ Target %.4f\n", price, targetPrice)
			b.executeSell(price, token)
		}
	}
}

func (b *DCABot) executeBuy(price float64, token string) {
	if b.TotalUSDT < b.OneBuyUSDT {
		sendTelegramMessage(token, "❗ No more USDT left for DCA.")
		return
	}

	params := map[string]interface{}{
		"category":  b.Category,
		"symbol":    b.Symbol,
		"side":      "Buy",
		"orderType": "Market",
		"qty":       fmt.Sprintf("%.2f", b.OneBuyUSDT), // For Spot Market Buy, qty is USDT
	}

	_, err := b.BybitClient.NewUtaBybitServiceWithParams(params).PlaceOrder(context.Background())
	if err != nil {
		log.Printf("Bybit Buy API Error: %v", err)
		return
	}

	qty := b.OneBuyUSDT / price
	b.TotalUSDT -= b.OneBuyUSDT

	record := DCARecord{
		BuyNumber:     len(b.Records) + 1,
		Price:         price,
		USDTSpent:     b.OneBuyUSDT,
		AmountBought:  qty,
		RemainingUSDT: b.TotalUSDT,
		TotalHoldings: b.totalHoldings() + qty,
	}
	b.Records = append(b.Records, record)

	message := fmt.Sprintf("📉 BYBIT BUY #%d\nSymbol: %s\nPrice: %.4f\nSpent: %.2f USDT\nAvg: %.4f",
		record.BuyNumber, b.Symbol, price, b.OneBuyUSDT, b.avgBuyPrice())
	sendTelegramMessage(token, message)
}

func (b *DCABot) executeSell(price float64, token string) {
	if len(b.Records) == 0 {
		return
	}

	totalHoldings := b.totalHoldings()
	sellQty := totalHoldings * 0.5 // Example: Sell 50%
	sellUSDT := sellQty * price

	params := map[string]interface{}{
		"category":  b.Category,
		"symbol":    b.Symbol,
		"side":      "Sell",
		"orderType": "Market",
		"qty":       fmt.Sprintf("%.6f", sellQty),
	}

	_, err := b.BybitClient.NewUtaBybitServiceWithParams(params).PlaceOrder(context.Background())
	if err != nil {
		log.Printf("Bybit Sell API Error: %v", err)
		return
	}

	// FIFO Logic
	remaining := sellQty
	realizedPNL := 0.0
	for i := 0; i < len(b.Records) && remaining > 0; i++ {
		r := &b.Records[i]
		if r.AmountBought <= remaining {
			realizedPNL += (price - r.Price) * r.AmountBought
			remaining -= r.AmountBought
			r.AmountBought = 0
		} else {
			realizedPNL += (price - r.Price) * remaining
			r.AmountBought -= remaining
			remaining = 0
		}
	}

	b.TotalUSDT += sellUSDT
	b.RealizedPNL += realizedPNL

	// Filter out empty records
	var updated []DCARecord
	for _, r := range b.Records {
		if r.AmountBought > 0 {
			updated = append(updated, r)
		}
	}
	b.Records = updated

	message := fmt.Sprintf("🔴 BYBIT SELL\nPrice: %.4f\nQty: %.6f\nRealized: %.2f", price, sellQty, realizedPNL)
	sendTelegramMessage(token, message)
}

func StartDCAWebSocket(bot *DCABot, token string) {
	go bot.StartDailyPNLTracker(token)

	for {
		err := startBybitWS(bot, token)
		log.Printf("Bybit WS Disconnected: %v. Reconnecting...", err)
		time.Sleep(5 * time.Second)
	}
}

func startBybitWS(bot *DCABot, token string) error {
	wsURL := "wss://stream.bybit.com/v5/public/spot"
	c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return err
	}
	defer c.Close()

	// 1. Subscribe to Trade Topic
	sub := map[string]interface{}{
		"op":   "subscribe",
		"args": []string{"publicTrade." + bot.Symbol},
	}
	if err := c.WriteJSON(sub); err != nil {
		return err
	}

	// 2. Start Heartbeat (Ping) every 20s
	go func() {
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if err := c.WriteJSON(map[string]string{"op": "ping"}); err != nil {
				return
			}
		}
	}()

	fmt.Printf("✅ Bybit WS Connected for %s\n", bot.Symbol)

	for {
		var msg map[string]interface{}
		if err := c.ReadJSON(&msg); err != nil {
			return err
		}

		// Bybit V5 Public Trade data structure
		if data, ok := msg["data"].([]interface{}); ok && len(data) > 0 {
			trade := data[0].(map[string]interface{})
			if pStr, ok := trade["p"].(string); ok {
				price, _ := strconv.ParseFloat(pStr, 64)
				bot.OnPrice(price, token)
			}
		}
	}
}

func RunDCABot(symbol string, totalUSDT, oneBuyUSDT, dropPercent, sellPercent float64, fallbackBuyHours int) {
	bot := NewDCABot(symbol, totalUSDT, dropPercent, sellPercent, fallbackBuyHours)
	bot.OneBuyUSDT = oneBuyUSDT

	tokenMap := constant.GetTokenMap()
	tokenConfig, ok := tokenMap[bot.Symbol].(map[float64]string)
	if !ok {
		log.Println("symbol not found")
	}

	token, ok := tokenConfig[bot.DropPercent]
	if !ok {
		log.Println("drop percent not found")
	}

	switch bot.Symbol {
	case "btcusdt":
		switch fallbackBuyHours {
		case 1:
			token = config.BTC1_1h
		case 4:
			token = config.BTC1_4h
		}
	case "ethusdt":
		switch fallbackBuyHours {
		case 1:
			token = config.ETH1_1h
		case 4:
			token = config.ETH1_4h
		}
	default:
	}

	StartDCAWebSocket(bot, token)
}

// --- Helper Functions ---

func (b *DCABot) totalCost() float64 {
	var total float64
	for _, r := range b.Records {
		total += (r.Price * r.AmountBought)
	}
	return total
}

func (b *DCABot) totalHoldings() float64 {
	sum := 0.0
	for _, r := range b.Records {
		sum += r.AmountBought
	}
	return sum
}

func (b *DCABot) avgBuyPrice() float64 {
	holdings := b.totalHoldings()
	if holdings == 0 {
		return 0
	}
	return b.totalCost() / holdings
}

func (b *DCABot) UnrealizedPNL(currentPrice float64) (pnlUSDT float64, pnlPercent float64) {
	avg := b.avgBuyPrice()
	holdings := b.totalHoldings()

	if holdings == 0 {
		return 0, 0
	}

	pnlUSDT = (currentPrice - avg) * holdings
	pnlPercent = ((currentPrice / avg) - 1) * 100
	return
}

func (b *DCABot) StartDailyPNLTracker(token string) {
	for {
		// Calculate duration until next midnight
		now := time.Now()
		nextMidnight := time.Date(
			now.Year(), now.Month(), now.Day()+1,
			0, 0, 0, 0,
			now.Location(),
		)
		timeUntilMidnight := nextMidnight.Sub(now)

		// Sleep until 12:00 AM
		time.Sleep(timeUntilMidnight)

		// Skip if no holdings
		if len(b.Records) == 0 {
			continue
		}

		currentPrice := b.LatestDayPrice
		pnlUSDT, pnlPercent := b.UnrealizedPNL(currentPrice)

		message := fmt.Sprintf(
			"📊 Daily PNL Report (%s)\nTime: %s\nCurrent Price: %.4f\nAvg Entry: %.4f\nTotal Holdings: %.6f\nRealized PNL: %.2f USDT\nUnrealized PNL: %.2f USDT (%.2f%%)",
			b.Symbol,
			time.Now().Format("2006-01-02 15:04:05"),
			currentPrice,
			b.avgBuyPrice(),
			b.totalHoldings(),
			b.RealizedPNL,
			pnlUSDT,
			pnlPercent,
		)

		sendTelegramMessage(token, message)

		// Loop will repeat → next iteration calculates next midnight
	}
}
