package bot

import (
	"dca-bot/config"
	"dca-bot/constant"
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

// Candle represents a Binance kline/candle message
type Candle struct {
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
	CloseTime time.Time
	IsFinal   bool
}

var (
	closes          []float64
	volumes         []float64
	rsiLength       = 14
	volumeLookback  = 20
	bbLength        = 20
	bbMult          = 2.0
	balance         = 10000.0
	totalProfitLoss = 0.0
	entryPrice      float64
	state           = 0 // 0 = neutral, 1 = long, -1 = short
	stopLossPercent float64
	numOfWin        = 0
	numOfLose       = 0
)

// Bot runs the trading bot on given symbol, interval and stop loss percent
func Bot(symbol, interval, token string, slPercent float64) {
	stopLossPercent = slPercent

	// Fetch historical candles
	history, err := fetchHistoricalCandles(strings.ToUpper(symbol), interval)
	if err != nil {
		log.Fatal("Error fetching historical candles:", err)
	}
	for _, c := range history {
		closes = append(closes, c.Close)
		volumes = append(volumes, c.Volume)
	}

	// Keep buffer size trimmed
	if len(closes) > 500 {
		closes = closes[len(closes)-500:]
		volumes = volumes[len(volumes)-500:]
	}

	msg := fmt.Sprintf("%s %s start~~~", symbol, interval)
	sendTelegramMessage(token, msg)

	// Start WebSocket
	go startWebSocket(strings.ToLower(symbol), interval, token)

	waitForShutdown()
}

func fetchHistoricalCandles(symbol, interval string) ([]Candle, error) {
	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/klines?symbol=%s&interval=%s", symbol, interval)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data [][]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var candles []Candle
	for _, item := range data {
		open := parseStringToFloat(item[1])
		high := parseStringToFloat(item[2])
		low := parseStringToFloat(item[3])
		close := parseStringToFloat(item[4])
		volume := parseStringToFloat(item[5])
		closeTime := time.UnixMilli(int64(item[6].(float64)))

		candles = append(candles, Candle{
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
			CloseTime: closeTime,
			IsFinal:   true,
		})
	}
	return candles, nil
}

func parseStringToFloat(s any) float64 {
	val, _ := strconv.ParseFloat(s.(string), 64)
	return val
}

func startWebSocket(symbol, interval, token string) {
	urlStr := fmt.Sprintf("wss://fstream.binance.com/ws/%s@kline_%s", symbol, interval)

	for {
		log.Println("Connecting to", urlStr)
		c, _, err := websocket.DefaultDialer.Dial(urlStr, nil)
		if err != nil {
			log.Println("WebSocket dial error:", err)
			time.Sleep(5 * time.Second)
			continue
		}

		func(conn *websocket.Conn) {
			defer conn.Close()

			for {
				_, message, err := conn.ReadMessage()
				if err != nil {
					log.Println("Read error:", err)
					return // exit inner loop to reconnect
				}

				var raw map[string]any
				if err := json.Unmarshal(message, &raw); err != nil {
					continue
				}

				kline, ok := raw["k"].(map[string]any)
				if !ok || !kline["x"].(bool) {
					continue
				}

				candle := Candle{
					Open:      parseStringToFloat(kline["o"]),
					High:      parseStringToFloat(kline["h"]),
					Low:       parseStringToFloat(kline["l"]),
					Close:     parseStringToFloat(kline["c"]),
					Volume:    parseStringToFloat(kline["v"]),
					CloseTime: time.UnixMilli(int64(kline["T"].(float64))),
					IsFinal:   true,
				}

				spikeUpPerc := ((candle.High - candle.Open) / candle.Open * 100)
				spikeDownPerc := ((candle.Low - candle.Open) / candle.Open * 100)
				a := constant.PercentageMap[interval]

				if spikeUpPerc >= a {
					msg := fmt.Sprintf("⚠️ Sudden PUMP detected!\nSymbol: %s\nHigh: %.4f\nOpen: %.4f\nChange: +%.2f%%", symbol, candle.High, candle.Open, spikeUpPerc)
					sendTelegramMessage(token, msg)
				}

				if spikeDownPerc <= -a {
					msg := fmt.Sprintf("⚠️ Sudden DUMP detected!\nSymbol: %s\nLow: %.4f\nOpen: %.4f\nChange: %.2f%%", symbol, candle.Low, candle.Open, spikeDownPerc)
					sendTelegramMessage(token, msg)
				}

				processCandle(candle, symbol, token)
			}
		}(c)

		log.Println("WebSocket disconnected. Reconnecting in 5s...")
		time.Sleep(5 * time.Second)
	}
}

func waitForShutdown() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-interrupt
	log.Println("Shutting down.")
}

// processCandle runs the strategy logic on each closed candle
func processCandle(c Candle, symbol, token string) {
	s := strings.ToUpper(symbol[:len(symbol)-4])

	closes = append(closes, c.Close)
	volumes = append(volumes, c.Volume)

	if len(closes) > 500 {
		closes = closes[1:]
		volumes = volumes[1:]
	}

	if len(closes) < rsiLength || len(volumes) < volumeLookback || len(closes) < bbLength {
		return
	}

	rsiVal := calcRSI(closes, rsiLength)
	avgVolume := sma(volumes, volumeLookback)
	highVolume := c.Volume > avgVolume*1.5
	extremeHighVolume := c.Volume > avgVolume*3

	greenCandle := c.Close > c.Open
	redCandle := c.Close < c.Open

	highLowDiff := c.High - c.Low
	if highLowDiff == 0 {
		return
	}
	topWick := c.High - math.Max(c.Open, c.Close)
	bottomWick := math.Min(c.Open, c.Close) - c.Low
	topWickPerc := (topWick / highLowDiff) * 100
	bottomWickPerc := (bottomWick / highLowDiff) * 100

	basis := sma(closes[len(closes)-bbLength:], bbLength)
	stdDev := stddev(closes[len(closes)-bbLength:], basis)
	upper := basis + bbMult*stdDev
	lower := basis - bbMult*stdDev

	rawBuy := (rsiVal < 35 && highVolume && (greenCandle || (redCandle && bottomWickPerc > 60))) || extremeHighVolume
	rawSell := (rsiVal > 65 && highVolume && (redCandle || (greenCandle && topWickPerc > 60))) || extremeHighVolume

	combinedBuy := rawBuy && c.Close <= lower
	combinedSell := rawSell && c.Close >= upper

	buySignal := combinedBuy
	sellSignal := combinedSell

	// === STOP LOSS CHECK ===
	if state == 1 && c.Close <= entryPrice*(1-stopLossPercent/100) {
		size := constant.QuantityMap[symbol]
		positionSize := strconv.FormatFloat(size, 'f', constant.SymbolPrecisionMap[symbol][1], 64)
		profit := (c.Close - entryPrice) * size
		percentChange := ((c.Close - entryPrice) / entryPrice) * 100
		price := strconv.FormatFloat(c.Close, 'f', constant.SymbolPrecisionMap[symbol][0], 64)
		balance += size*c.Close + profit
		// placeOrder(symbol, "SELL")
		a := fmt.Sprintf("STOP LOSS [LONG]\nAmount: %s %s \nPrice: %s \nPercent changed: %.2f\nLoss: %.2f USDT\nBalance: %.2f USDT\n", positionSize, s, price, percentChange, profit, balance)
		log.Println(a)
		sendTelegramMessage(token, a)
		state, entryPrice = 0, 0
		totalProfitLoss += profit
		b := fmt.Sprintf("Total profit/loss : %.2f", totalProfitLoss)
		log.Println(b)
		sendTelegramMessage(token, b)
		numOfLose += 1
		c := fmt.Sprintf("Win: %d | Lose: %d", numOfWin, numOfLose)
		sendTelegramMessage(token, c)
		return
	}
	if state == -1 && c.Close >= entryPrice*(1+stopLossPercent/100) {
		size := constant.QuantityMap[symbol]
		positionSize := strconv.FormatFloat(size, 'f', constant.SymbolPrecisionMap[symbol][1], 64)
		profit := (entryPrice - c.Close) * size
		percentChange := ((c.Close - entryPrice) / entryPrice) * 100
		price := strconv.FormatFloat(c.Close, 'f', constant.SymbolPrecisionMap[symbol][0], 64)
		balance += size*c.Close + profit
		// placeOrder(symbol, "BUY")
		a := fmt.Sprintf("STOP LOSS [SHORT]\nAmount: %s %s \nPrice: %s \nPercent changed: %.2f\nLoss: %.2f USDT\nBalance: %.2f USDT\n", positionSize, s, price, percentChange, profit, balance)
		log.Println(a)
		sendTelegramMessage(token, a)
		state, entryPrice = 0, 0
		totalProfitLoss += profit
		b := fmt.Sprintf("Total profit/loss : %.2f", totalProfitLoss)
		log.Println(b)
		sendTelegramMessage(token, b)
		numOfLose += 1
		c := fmt.Sprintf("Win: %d | Lose: %d", numOfWin, numOfLose)
		sendTelegramMessage(token, c)
		return
	}

	// === TRADING LOGIC ===
	if state == 0 {
		// Neutral: open position on any signal
		if buySignal {
			size := constant.QuantityMap[symbol]
			if balance >= size*c.Close {
				size := constant.QuantityMap[symbol]
				positionSize := strconv.FormatFloat(size, 'f', constant.SymbolPrecisionMap[symbol][1], 64)
				entryPrice = c.Close
				price := strconv.FormatFloat(entryPrice, 'f', constant.SymbolPrecisionMap[symbol][0], 64)
				balance -= size * c.Close
				state = 1
				stopLoss := strconv.FormatFloat(c.Close*(1-stopLossPercent/100), 'f', 2, 64)
				// placeOrder(symbol, "BUY")
				a := fmt.Sprintf("[LONG]\nAmount: %s %s \nPrice: %s \nStop loss: %s \nBalance: %.2f", positionSize, s, price, stopLoss, balance)
				log.Println(a)
				sendTelegramMessage(token, a)
			} else {
				a := "Insufficient balance to open LONG position"
				log.Println(a)
				sendTelegramMessage(token, a)
			}
			return
		}
		if sellSignal {
			size := constant.QuantityMap[symbol]
			if balance >= size*c.Close {
				size := constant.QuantityMap[symbol]
				positionSize := strconv.FormatFloat(size, 'f', constant.SymbolPrecisionMap[symbol][1], 64)
				entryPrice = c.Close
				price := strconv.FormatFloat(entryPrice, 'f', constant.SymbolPrecisionMap[symbol][0], 64)
				balance -= size * c.Close
				state = -1
				stopLoss := strconv.FormatFloat(c.Close*(1-stopLossPercent/100), 'f', 2, 64)
				// placeOrder(symbol, "SELL")
				a := fmt.Sprintf("[LONG]\nAmount: %s %s \nPrice: %s \nStop loss: %s \nBalance: %.2f", positionSize, s, price, stopLoss, balance)
				log.Println(a)
				sendTelegramMessage(token, a)
			} else {
				a := "Insufficient balance to open SHORT position"
				log.Println(a)
				sendTelegramMessage(token, a)
			}
			return
		}
	} else if state == 1 {
		// Long position: close only on sell signal
		if sellSignal {
			size := constant.QuantityMap[symbol]
			positionSize := strconv.FormatFloat(size, 'f', constant.SymbolPrecisionMap[symbol][1], 64)
			profit := (c.Close - entryPrice) * size
			percentChange := ((c.Close - entryPrice) / entryPrice) * 100
			price := strconv.FormatFloat(c.Close, 'f', constant.SymbolPrecisionMap[symbol][0], 64)
			balance += size*c.Close + profit
			// placeOrder(symbol, "SELL")
			a := fmt.Sprintf("Closed [LONG]\nAmount: %s %s \nPrice: %s \nPercent changed: %.2f\nProfit: %.2f USDT\nBalance: %.2f USDT\n", positionSize, s, price, percentChange, profit, balance)
			log.Println(a)
			sendTelegramMessage(token, a)
			state, entryPrice = 0, 0
			totalProfitLoss += profit
			b := fmt.Sprintf("Total profit/loss : %.2f", totalProfitLoss)
			log.Println(b)
			sendTelegramMessage(token, b)
			if profit < 0 {
				numOfLose += 1
			} else {
				numOfWin += 1
			}
			c := fmt.Sprintf("Win: %d | Lose: %d", numOfWin, numOfLose)
			sendTelegramMessage(token, c)
			return
		}
	} else if state == -1 {
		// Short position: close only on buy signal
		if buySignal {
			size := constant.QuantityMap[symbol]
			positionSize := strconv.FormatFloat(size, 'f', constant.SymbolPrecisionMap[symbol][1], 64)
			profit := (entryPrice - c.Close) * size
			percentChange := ((c.Close - entryPrice) / entryPrice) * 100
			price := strconv.FormatFloat(c.Close, 'f', constant.SymbolPrecisionMap[symbol][0], 64)
			balance += size*c.Close + profit
			// placeOrder(symbol, "BUY")
			a := fmt.Sprintf("Closed [SHORT]\nAmount: %s %s \nPrice: %s \nPercent changed: %.2f\nProfit: %.2f USDT\nBalance: %.2f USDT\n", positionSize, s, price, percentChange, profit, balance)
			log.Println(a)
			sendTelegramMessage(token, a)
			state, entryPrice = 0, 0
			totalProfitLoss += profit
			b := fmt.Sprintf("Total profit/loss : %.2f", totalProfitLoss)
			log.Println(b)
			sendTelegramMessage(token, b)
			if profit < 0 {
				numOfLose += 1
			} else {
				numOfWin += 1
			}
			c := fmt.Sprintf("Win: %d | Lose: %d", numOfWin, numOfLose)
			sendTelegramMessage(token, c)
			return
		}
	}
}

func placeOrder(symbol string, side string) {
	endpoint := "https://fapi.binance.com/fapi/v1/order"
	timestamp := time.Now().UnixMilli()

	// Convert quantity to string with 4 decimals
	qtyStr := strconv.FormatFloat(constant.QuantityMap[symbol], 'f', constant.SymbolPrecisionMap[symbol][1], 64)

	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("side", side) // "BUY" or "SELL"
	params.Set("type", "MARKET")
	params.Set("quantity", qtyStr)
	params.Set("timestamp", strconv.FormatInt(timestamp, 10))

	// Sign the query
	queryString := params.Encode()
	signature := sign(queryString, config.BinanceApiSecret)
	params.Set("signature", signature)

	req, err := http.NewRequest("POST", endpoint, bytes.NewBufferString(params.Encode()))
	if err != nil {
		log.Println("Error creating order request:", err)
		return
	}

	req.Header.Set("X-MBX-APIKEY", config.BinanceApiKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error placing order:", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	log.Println("Order response:", string(body))
}

// sign generates HMAC-SHA256 signature
func sign(data, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

func calcRSI(closes []float64, length int) float64 {
	if len(closes) < length+1 {
		return 0
	}
	var gains, losses float64
	for i := len(closes) - length; i < len(closes); i++ {
		change := closes[i] - closes[i-1]
		if change > 0 {
			gains += change
		} else {
			losses -= change
		}
	}
	if losses == 0 {
		return 100
	}
	rs := gains / losses
	rsi := 100 - (100 / (1 + rs))
	return rsi
}

func sma(data []float64, length int) float64 {
	if len(data) < length {
		return 0
	}
	sum := 0.0
	for _, v := range data[len(data)-length:] {
		sum += v
	}
	return sum / float64(length)
}

func stddev(data []float64, mean float64) float64 {
	var sum float64
	for _, v := range data {
		sum += (v - mean) * (v - mean)
	}
	return math.Sqrt(sum / float64(len(data)))
}
