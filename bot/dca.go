package bot

import (
	"dca-bot/config"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"
	"net/http"
    "io" 
)

type DCABot struct {
	Symbol       string
	DropPercent  float64
	TotalUSDT    float64
	OneBuyUSDT   float64
	LastBuyPrice float64
	Started      bool
	Records      []DCARecord
}

type DCARecord struct {
	BuyNumber     int
	Price         float64
	USDTSpent     float64
	AmountBought  float64
	RemainingUSDT float64
	TotalHoldings float64
}

func NewDCABot(symbol string, totalUSDT float64, dropPercent float64) *DCABot {
	return &DCABot{
		Symbol:      symbol,
		DropPercent: dropPercent,
		TotalUSDT:   totalUSDT,
		OneBuyUSDT:  totalUSDT * 0.01,
		Records:     []DCARecord{},
	}
}

func (b *DCABot) OnPrice(price float64) {
	if !b.Started {
		fmt.Printf("\nDCA START — FIRST BUY at %.4f\n", price)
		b.executeBuy(price)
		b.LastBuyPrice = price
		b.Started = true
		return
	}

	drop := ((b.LastBuyPrice - price) / b.LastBuyPrice) * 100

	if drop >= b.DropPercent {
		fmt.Printf("PRICE DROP %.2f%% → BUY triggered\n", drop)
		b.executeBuy(price)
		b.LastBuyPrice = price
	}
}

func (b *DCABot) executeBuy(price float64) {
	if b.TotalUSDT < b.OneBuyUSDT {
		fmt.Println("No more USDT left.")
		sendTelegramMessage(config.DCAToken, "❗ No more USDT left for DCA.")
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

	message := fmt.Sprintf(
		"📉 DCA BUY #%d\nSymbol: %s\nPrice: %.4f\nBought: %.6f %s\nUSDT Spent: %.2f\nRemaining: %.2f\nTotal Holdings: %.6f %s",
		record.BuyNumber, b.Symbol,
		record.Price, record.AmountBought, b.Symbol,
		record.USDTSpent, record.RemainingUSDT,
		record.TotalHoldings, b.Symbol,
	)

	// Send Telegram message
	sendTelegramMessage(config.DCAToken, message)

	fmt.Printf(`
===== DCA BUY #%d =====
Price: %.4f
Bought: %.6f %s
USDT Spent: %.2f
Remaining USDT: %.2f
Total Holdings: %.6f %s
========================
`, record.BuyNumber, record.Price, record.AmountBought, b.Symbol,
		record.USDTSpent, record.RemainingUSDT, record.TotalHoldings, b.Symbol)
}

func (b *DCABot) totalHoldings() float64 {
	sum := 0.0
	for _, r := range b.Records {
		sum += r.AmountBought
	}
	return sum
}

func StartDCAWebSocket(bot *DCABot) {
    wsURL := fmt.Sprintf(
        "wss://stream.binance.com:9443/ws/%s@trade",
        strings.ToLower(bot.Symbol),
    )

    fmt.Println("Connecting to:", wsURL)

    dialer := websocket.Dialer{}

    header := http.Header{}
    header.Add("Origin", "https://binance.com")

    c, resp, err := dialer.Dial(wsURL, header)
    if err != nil {
        if resp != nil {
            body, _ := io.ReadAll(resp.Body)
            fmt.Println("Handshake failed:", resp.Status, string(body))
        }
        log.Fatal("WebSocket error:", err)
    }
    defer c.Close()

    fmt.Println("Connected OK!")

    for {
        _, msg, err := c.ReadMessage()
        if err != nil {
            fmt.Println("WS closed:", err)
            return
        }

        var data struct {
            Price string `json:"p"`
        }

        if err := json.Unmarshal(msg, &data); err != nil {
            continue
        }

        price, _ := strconv.ParseFloat(data.Price, 64)
        bot.OnPrice(price)
    }
}

func RunDCABot(symbol string, totalUSDT, oneBuyUSDT, dropPercent float64) {
	bot := NewDCABot(symbol, totalUSDT, dropPercent)
	bot.OneBuyUSDT = oneBuyUSDT // ensure 1% of total

	StartDCAWebSocket(bot)
}
