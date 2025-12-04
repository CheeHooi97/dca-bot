package handler

import (
	"dca-bot/constant"
	"dca-bot/service"
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type TradeHandler struct {
	service *service.TradeService
}

func NewTradeHandler() *TradeHandler {
	return &TradeHandler{
		service: service.NewTradeService(),
	}
}

func (h *TradeHandler) StartCLI() error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("Enter trading pair (e.g. btcusdt): ")
	symbol, _ := reader.ReadString('\n')
	symbol = strings.TrimSpace(symbol)

	fmt.Printf("Enter interval for %s (e.g. 1m, 5m, 15m, 1h): ", symbol)
	interval, _ := reader.ReadString('\n')
	interval = strings.TrimSpace(interval)

	fmt.Print("Enter stop loss percentage (e.g. 1.5): ")
	slInput, _ := reader.ReadString('\n')
	slInput = strings.TrimSpace(slInput)

	stopLossPercent, err := strconv.ParseFloat(slInput, 64)
	if err != nil {
		fmt.Println("Invalid stop loss, using default 1.5%")
		stopLossPercent = 1.5
	}

	// fetch token
	tokenMap := constant.GetTokenMap()
	token := tokenMap[symbol].(map[string]string)[interval]

	return h.service.Start(symbol, interval, token, stopLossPercent)
}
