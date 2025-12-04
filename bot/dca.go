package bot

import (
	"dca-bot/config"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
    "io"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"
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

        bot.OnPrice(price)
    }
}

func RunDCABot(symbol string, totalUSDT, oneBuyUSDT, dropPercent float64) {
	bot := NewDCABot(symbol, totalUSDT, dropPercent)
	bot.OneBuyUSDT = oneBuyUSDT // ensure 1% of total

	StartDCAWebSocket(bot)
}
