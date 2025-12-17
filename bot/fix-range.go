package bot

import (
	"dca-bot/constant"
	"fmt"
	"log"
	"math"
	"sync"
	"time"
)

type GridDirection int

const (
	GridUp GridDirection = iota
	GridDown
)

type FixRangeCandle struct {
	High  float64
	Low   float64
	Close float64
}

type FixRangeRecord struct {
	GridIndex int
	BuyPrice  float64
	Amount    float64
	BuyTime   time.Time
}

type FixRangeBot struct {
	mu sync.Mutex

	Symbol string

	// Capital
	TotalUSDT   float64
	OneBuyUSDT  float64
	RealizedPNL float64

	// Grid
	LowPrice   float64
	HighPrice  float64
	GridStep   float64
	GridCount  int
	GridHeight float64

	// Adaptive
	ATRPeriod      int
	ATRMultiplier  float64
	Candles        []FixRangeCandle
	LastATRUpdate  time.Time
	ATRUpdateEvery time.Duration

	// Trend
	Direction    GridDirection
	TrailingStop float64
	StopLossPct  float64

	// Records
	Records []FixRangeRecord

	// Price
	LatestPrice float64
}

func NewFixRangeBot(symbol string, usdt float64) *FixRangeBot {
	return &FixRangeBot{
		Symbol:         symbol,
		TotalUSDT:      usdt,
		OneBuyUSDT:     usdt * 0.02, // 2% per grid buy
		GridCount:      10,          // default 10 grids
		ATRPeriod:      14,
		ATRMultiplier:  1.2,
		ATRUpdateEvery: time.Minute * 15,
		StopLossPct:    0.08, // 8%
		Direction:      GridUp,
		Records:        []FixRangeRecord{},
	}
}

////////////////////////////////////////////////////////////
// ATR (Volatility-Adaptive Grid)
////////////////////////////////////////////////////////////

func calculateATR(c []FixRangeCandle, period int) float64 {
	if len(c) < period+1 {
		return 0
	}

	sum := 0.0
	for i := len(c) - period; i < len(c); i++ {
		h := c[i].High
		l := c[i].Low
		pc := c[i-1].Close

		tr := math.Max(h-l, math.Max(
			math.Abs(h-pc),
			math.Abs(l-pc),
		))
		sum += tr
	}
	return sum / float64(period)
}

////////////////////////////////////////////////////////////
// Trend Detection (Auto Direction)
////////////////////////////////////////////////////////////

func (b *FixRangeBot) detectTrend() {
	if len(b.Candles) < 20 {
		return
	}

	recent := b.Candles[len(b.Candles)-1].Close
	old := b.Candles[len(b.Candles)-20].Close

	if recent > old {
		b.Direction = GridUp
	} else {
		b.Direction = GridDown
	}
}

////////////////////////////////////////////////////////////
// Grid Setup (Volatility-Adaptive)
////////////////////////////////////////////////////////////

func (b *FixRangeBot) rebuildGrid(price float64) {
	atr := calculateATR(b.Candles, b.ATRPeriod)
	if atr == 0 {
		return
	}

	b.GridStep = atr * b.ATRMultiplier
	b.GridHeight = b.GridStep * float64(b.GridCount)

	if b.Direction == GridUp {
		b.LowPrice = price - b.GridHeight/2
		b.HighPrice = b.LowPrice + b.GridHeight
		b.TrailingStop = b.LowPrice * (1 - b.StopLossPct)
	} else {
		b.HighPrice = price + b.GridHeight/2
		b.LowPrice = b.HighPrice - b.GridHeight
		b.TrailingStop = b.HighPrice * (1 + b.StopLossPct)
	}
}

////////////////////////////////////////////////////////////
// Core Price Handler
////////////////////////////////////////////////////////////

func (b *FixRangeBot) OnPrice(symbol string, price float64, candle FixRangeCandle) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.LatestPrice = price
	b.Candles = append(b.Candles, candle)

	// ATR + Trend Update
	if time.Since(b.LastATRUpdate) > b.ATRUpdateEvery {
		b.detectTrend()
		b.rebuildGrid(price)
		b.LastATRUpdate = time.Now()
	}

	// Stop Loss Trigger
	if b.Direction == GridUp && price < b.TrailingStop {
		b.forceSellAll(price)
		return
	}
	if b.Direction == GridDown && price > b.TrailingStop {
		b.forceSellAll(price)
		return
	}

	// Grid Shift (Trend-Following)
	if price >= b.HighPrice && b.Direction == GridUp {
		b.shiftGridUp()
	}
	if price <= b.LowPrice && b.Direction == GridDown {
		b.shiftGridDown()
	}

	// Calculate which grid the price is in
	grid := int((price - b.LowPrice) / b.GridStep)
	if grid < 0 || grid >= b.GridCount {
		return
	}

	tokenMap := constant.GetFixedRangeTokenMap()
	token, ok := tokenMap[symbol].(string)
	if !ok {
		log.Println("symbol not found")
	}

	fmt.Println(token)

	b.trySell(grid, price, token)
	b.tryBuy(grid, price, token)
}

////////////////////////////////////////////////////////////
// Grid Movement (Trend-Following Shift)
////////////////////////////////////////////////////////////

func (b *FixRangeBot) shiftGridUp() {
	b.LowPrice = b.HighPrice
	b.HighPrice = b.LowPrice + b.GridHeight
	b.TrailingStop = b.LowPrice * (1 - b.StopLossPct)
}

func (b *FixRangeBot) shiftGridDown() {
	b.HighPrice = b.LowPrice
	b.LowPrice = b.HighPrice - b.GridHeight
	b.TrailingStop = b.HighPrice * (1 + b.StopLossPct)
}

////////////////////////////////////////////////////////////
// Trading Logic (Pionex Exact)
////////////////////////////////////////////////////////////

func (b *FixRangeBot) tryBuy(grid int, price float64, token string) {
	if b.TotalUSDT < b.OneBuyUSDT {
		return
	}

	// Only buy if grid not already bought
	for _, r := range b.Records {
		if r.GridIndex == grid {
			return
		}
	}

	amt := b.OneBuyUSDT / price
	b.TotalUSDT -= b.OneBuyUSDT

	b.Records = append(b.Records, FixRangeRecord{
		GridIndex: grid,
		BuyPrice:  price,
		Amount:    amt,
		BuyTime:   time.Now(),
	})

	message := fmt.Sprintf("🟢 BUY %s Grid:%d Price:%.2f\n", b.Symbol, grid, price)
	fmt.Println(message)

	sendTelegramMessage(token, message)
}

func (b *FixRangeBot) trySell(grid int, price float64, token string) {
	for i := 0; i < len(b.Records); i++ {
		r := b.Records[i]

		if r.GridIndex == grid && price >= r.BuyPrice+b.GridStep {
			usdt := r.Amount * price
			pnl := usdt - (r.Amount * r.BuyPrice)

			b.TotalUSDT += usdt
			b.RealizedPNL += pnl

			// Remove sold record
			b.Records = append(b.Records[:i], b.Records[i+1:]...)

			message := fmt.Sprintf("🔴 SELL %s Grid:%d PNL:%.2f\n", b.Symbol, grid, pnl)
			fmt.Println(message)

			sendTelegramMessage(token, message)

			return
		}
	}
}

////////////////////////////////////////////////////////////
// Force Exit / Stop Loss
////////////////////////////////////////////////////////////

func (b *FixRangeBot) forceSellAll(price float64) {
	for _, r := range b.Records {
		usdt := r.Amount * price
		b.RealizedPNL += usdt - (r.Amount * r.BuyPrice)
		b.TotalUSDT += usdt
	}
	b.Records = []FixRangeRecord{}
	fmt.Println("🚨 STOP LOSS — ALL POSITIONS CLOSED")
}

////////////////////////////////////////////////////////////
// Unrealized PNL
////////////////////////////////////////////////////////////

func (b *FixRangeBot) UnrealizedPNL() float64 {
	sum := 0.0
	for _, r := range b.Records {
		sum += (b.LatestPrice - r.BuyPrice) * r.Amount
	}
	return sum
}
