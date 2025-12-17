package bot

import (
	"dca-bot/config"
	"dca-bot/constant"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

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
	return &DCABot{
		Symbol:        symbol,
		DropPercent:   dropPercent,
		SellPercent:   sellPercent,
		TotalUSDT:     totalUSDT,
		OneBuyUSDT:    totalUSDT * 0.01,
		Records:       []DCARecord{},
		FallbackHours: time.Duration(fallbackBuyHours) * time.Hour,
	}
}

func (b *DCABot) OnPrice(price float64, token string) {
	b.LatestDayPrice = price

	// FIRST BUY
	if !b.Started {
		fmt.Printf("\nDCA START — FIRST BUY at %.4f\n", price)
		b.executeBuy(price, token)
		b.LastBuyPrice = price
		b.LastBuyTime = time.Now()
		b.Started = true
		return
	}

	// PRICE DROP BUY
	drop := ((b.LastBuyPrice - price) / b.LastBuyPrice) * 100
	if drop >= b.DropPercent {
		fmt.Printf("PRICE DROP %.2f%% → BUY triggered\n", drop)
		b.executeBuy(price, token)
		b.LastBuyPrice = price
		b.LastBuyTime = time.Now()
		return
	}

	// PRICE RISE %
	rise := ((price - b.LastBuyPrice) / b.LastBuyPrice) * 100

	// FALLBACK BUY (after X hours + rise >= DropPercent)
	if time.Since(b.LastBuyTime) >= b.FallbackHours && rise >= b.DropPercent {
		fmt.Printf("NO DROP for %v → Rise %.2f%% ≥ %.2f%% → FALLBACK BUY at %.4f\n",
			b.FallbackHours, rise, b.DropPercent, price)

		b.executeBuy(price, token)
		b.LastBuyPrice = price
		b.LastBuyTime = time.Now()
		return
	}

	// SELL CONDITION
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
		fmt.Println("No more USDT left.")
		sendTelegramMessage(token, "❗ No more USDT left for DCA.")
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
	avgPrice := b.avgBuyPrice()

	message := fmt.Sprintf(
		"📉 DCA BUY #%d\nSymbol: %s\nPrice: %.4f\nBought: %.6f %s\nUSDT Spent: %.2f\nRemaining: %.2f\nTotal Holdings: %.6f %s\n📊 Avg Buy Price: %.4f",
		record.BuyNumber, b.Symbol,
		record.Price, record.AmountBought, b.Symbol,
		record.USDTSpent, record.RemainingUSDT,
		record.TotalHoldings, b.Symbol,
		avgPrice,
	)

	// Send Telegram message
	sendTelegramMessage(token, message)

	fmt.Printf(`
===== DCA BUY #%d =====
Price: %.4f
Bought: %.6f %s
USDT Spent: %.2f
Remaining USDT: %.2f
Total Holdings: %.6f %s
Avg Buy Price: %.4f
========================
`,
		record.BuyNumber, record.Price, record.AmountBought, b.Symbol,
		record.USDTSpent, record.RemainingUSDT, record.TotalHoldings, b.Symbol,
		avgPrice)

}

func (b *DCABot) executeSell(price float64, token string) {
	if len(b.Records) == 0 {
		return
	}

	totalHoldings := b.totalHoldings()
	recordCount := float64(len(b.Records))

	// Sell half of ONE record-sized chunk
	oneChunk := totalHoldings / recordCount
	sellQty := oneChunk * 0.5
	sellUSDT := sellQty * price
	remainingToSell := sellQty
	realizedPNL := 0.0

	// FIFO reduce from records
	for i := 0; i < len(b.Records) && remainingToSell > 0; i++ {
		r := &b.Records[i]

		if r.AmountBought <= remainingToSell {
			realizedPNL += (price - r.Price) * r.AmountBought
			remainingToSell -= r.AmountBought
			r.AmountBought = 0
		} else {
			realizedPNL += (price - r.Price) * remainingToSell
			r.AmountBought -= remainingToSell
			remainingToSell = 0
		}
	}

	// Clean up empty records
	newRecords := []DCARecord{}
	for _, r := range b.Records {
		if r.AmountBought > 0 {
			newRecords = append(newRecords, r)
		}
	}
	b.Records = newRecords

	b.TotalUSDT += sellUSDT
	b.RealizedPNL += realizedPNL // <--- accumulate realized PNL

	message := fmt.Sprintf(
		"🔴 MOCK SELL (½ of 1 record)\nSymbol: %s\nPrice: %.4f\nSold: %.6f %s\nReceived: %.2f USDT\nRealized PNL: %.2f USDT\nRemaining Holdings: %.6f %s\nRemaining USDT: %.2f",
		b.Symbol,
		price,
		sellQty, b.Symbol,
		sellUSDT,
		realizedPNL,
		b.totalHoldings(), b.Symbol,
		b.TotalUSDT,
	)

	sendTelegramMessage(token, message)

	fmt.Printf(`
===== MOCK SELL =====
Price: %.4f
Sold: %.6f %s
USDT Received: %.2f
Realized PNL: %.2f
Remaining Holdings: %.6f %s
Total USDT: %.2f
=====================
`,
		price,
		sellQty, b.Symbol,
		sellUSDT,
		realizedPNL,
		b.totalHoldings(), b.Symbol,
		b.TotalUSDT,
	)
}

func StartDCAWebSocket(bot *DCABot, token string) {
	go bot.StartDailyPNLTracker(token)

	for {
		err := startWS(bot, token)
		fmt.Println("WebSocket disconnected:", err)

		// Backoff before reconnect
		time.Sleep(5 * time.Second)
		fmt.Println("Reconnecting WebSocket...")
	}
}

func startWS(bot *DCABot, token string) error {
	wsURL := "wss://data-stream.binance.com/ws/" +
		strings.ToLower(bot.Symbol) + "@trade"

	header := http.Header{}
	header.Add("Origin", "https://binance.com")
	header.Add("User-Agent", "Mozilla/5.0")

	c, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		return err
	}
	defer c.Close()

	fmt.Println("✅ WebSocket connected successfully!")

	for {
		_, msg, err := c.ReadMessage()
		if err != nil {
			return err
		}

		var data struct {
			Price string `json:"p"`
		}

		if jsonErr := json.Unmarshal(msg, &data); jsonErr != nil {
			continue
		}

		price, err := strconv.ParseFloat(data.Price, 64)
		if err != nil {
			continue
		}

		bot.OnPrice(price, token)
	}
}

func RunDCABot(symbol string, totalUSDT, oneBuyUSDT, dropPercent, sellpercent float64, fallbackBuyHours int) {
	bot := NewDCABot(symbol, totalUSDT, dropPercent, sellpercent, fallbackBuyHours)
	bot.OneBuyUSDT = oneBuyUSDT // ensure 1% of total

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

func (b *DCABot) totalCost() float64 {
	var total float64
	for _, r := range b.Records {
		total += r.USDTSpent
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
	totalCost := b.totalCost()
	totalHoldings := b.totalHoldings()

	if totalHoldings == 0 {
		return 0
	}

	return totalCost / totalHoldings
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
