package bot

import (
	"dca-bot/constant"
	"encoding/json"
	"fmt"
	"io"
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
	TotalUSDT      float64
	OneBuyUSDT     float64
	LastBuyPrice   float64
	Started        bool
	Records        []DCARecord
	LastBuyTime    time.Time
	FallbackHours  time.Duration
	LatestDayPrice float64
}

type DCARecord struct {
	BuyNumber     int
	Price         float64
	USDTSpent     float64
	AmountBought  float64
	RemainingUSDT float64
	TotalHoldings float64
}

func NewDCABot(symbol string, totalUSDT float64, dropPercent float64, fallbackBuyHours int) *DCABot {
	return &DCABot{
		Symbol:        symbol,
		DropPercent:   dropPercent,
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

	// CHECK PRICE DROP
	drop := ((b.LastBuyPrice - price) / b.LastBuyPrice) * 100
	if drop >= b.DropPercent {
		fmt.Printf("PRICE DROP %.2f%% → BUY triggered\n", drop)
		b.executeBuy(price, token)
		b.LastBuyPrice = price
		b.LastBuyTime = time.Now()
		return
	}

	// CHECK 12-HOUR FALLBACK
	if time.Since(b.LastBuyTime) >= b.FallbackHours {
		fmt.Printf("NO DROP for %v → 12h FALLBACK BUY at %.4f\n", b.FallbackHours, price)
		b.executeBuy(price, token)
		b.LastBuyPrice = price
		b.LastBuyTime = time.Now()
		return
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

func StartDCAWebSocket(bot *DCABot) {

	// Malaysia-safe endpoint (AWS mirror)
	wsURL := "wss://data-stream.binance.com/ws/" +
		strings.ToLower(bot.Symbol) + "@trade"

	fmt.Println("Connecting to:", wsURL)

	// Add safe headers (some ISPs require Origin / UA)
	header := http.Header{}
	header.Add("Origin", "https://binance.com")
	header.Add("User-Agent", "Mozilla/5.0")

	c, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		fmt.Println("❌ WebSocket handshake failed")

		if resp != nil {
			fmt.Println("Status:", resp.Status)
			body := "<no body>"
			if resp.Body != nil {
				b, _ := io.ReadAll(resp.Body)
				body = string(b)
			}
			fmt.Println("Response body:", body)
		}

		log.Fatal("WebSocket error:", err)
	}
	defer c.Close()

	fmt.Println("✅ WebSocket connected successfully!")

	for {
		_, msg, err := c.ReadMessage()
		if err != nil {
			fmt.Println("WS closed:", err)
			return
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

		tokenMap := constant.GetTokenMap()
		tokenConfig, ok := tokenMap[bot.Symbol].(map[float64]string)
		if !ok {
			log.Println("symbol not found")
			return
		}

		token, ok := tokenConfig[bot.DropPercent]
		if !ok {
			log.Println("drop percent not found")
			return
		}

		bot.OnPrice(price, token)
	}
}

func RunDCABot(symbol string, totalUSDT, oneBuyUSDT, dropPercent float64, fallbackBuyHours int) {
	bot := NewDCABot(symbol, totalUSDT, dropPercent, fallbackBuyHours)
	bot.OneBuyUSDT = oneBuyUSDT // ensure 1% of total

	StartDCAWebSocket(bot)
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
	ticker := time.NewTicker(24 * time.Hour)

	for {
		<-ticker.C

		// Skip if no holdings
		if len(b.Records) == 0 {
			continue
		}

		// Use latest known price (you must store it — see below)
		currentPrice := b.LatestDayPrice
		pnlUSDT, pnlPercent := b.UnrealizedPNL(currentPrice)

		message := fmt.Sprintf(
			"📊 24h PNL Report (%s)\nCurrent Price: %.4f\nAvg Entry: %.4f\nTotal Holdings: %.6f\nUnrealized PNL: %.2f USDT (%.2f%%)",
			b.Symbol,
			currentPrice,
			b.avgBuyPrice(),
			b.totalHoldings(),
			pnlUSDT,
			pnlPercent,
		)

		sendTelegramMessage(token, message)
	}
}
